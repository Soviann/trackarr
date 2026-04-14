# Plex Rewatch Tracking + Dual Timestamps

## PO Summary

Rewatching a show on Plex doesn't update PlexTracker — the show stays buried in the library instead of rising to the top. Additionally, there's no distinction between when you first watched an episode and when you last rewatched it. This change makes rewatches visible in the "last watched" list and tracks both timestamps on episodes and titles.

## Root Cause Analysis

**Poirot (7570)**: Plex does NOT send `media.scrobble` for already-watched content. Rewatching only generates `media.play`/`media.stop`. PlexTracker filters out everything except `media.scrobble` → rewatches are invisible. Confirmed by NAS logs: all April 14 webhooks return 200 in <1ms (filtered), no DB writes since April 13 20:27.

**Title 7555 (ROLL OVER AND DIE)**:
- Ep 11: scrobble WAS processed (877ms, April 13 18:27) — data is in the WAL (not in main DB file, which is why initial investigation missed it)
- Ep 12: Plex DID send the scrobble (Simkl received it and marked it watched), but PlexTracker silently failed to process it — likely one of the 17-31ms webhooks on April 13 evening that hit an unlogged error. Root cause unknown without webhook logging (→ Task 2 fixes this).

**Secondary bug**: `make ssh-db-pull` only copies the main DB file, not WAL/SHM. This means pulled DBs miss recent un-checkpointed writes.

---

## Task 1 — DB migration: dual timestamps on episodes + titles

### Why first
The rewatch feature (Task 2) needs to update `last_watched_at` without destroying the original `first_watched_at`. This migration must land before the rewatch code.

### Migration 018

**Episodes**:
- Rename `watched_at` → `first_watched_at` (preserves existing data — when the episode was first watched)
- Add `last_watched_at` (nullable, defaults to `first_watched_at` for existing data)
- `ALTER TABLE episodes RENAME COLUMN watched_at TO first_watched_at` + `ALTER TABLE episodes ADD COLUMN last_watched_at DATETIME` + `UPDATE episodes SET last_watched_at = first_watched_at WHERE first_watched_at IS NOT NULL`

**Titles**:
- Add `first_watched_at` (nullable)
- Keep existing `last_watched_at`
- Backfill: `UPDATE titles SET first_watched_at = created_at WHERE last_watched_at IS NOT NULL` (best approximation — the title was created when first scrobbled)

### Code changes
- `internal/model/episode.go`: rename `WatchedAt` → `FirstWatchedAt`, add `LastWatchedAt`
- `internal/repository/episode.go`: update all queries referencing `watched_at`
  - `GetOrCreate`, `GetBySeasonID`, `ToggleWatched`: read `first_watched_at`, `last_watched_at`
  - `BatchMarkWatched`: set `first_watched_at = ? WHERE first_watched_at IS NULL` + always set `last_watched_at = ?`
  - `MarkWatched`: same dual-write logic
- `internal/model/title.go`: add `FirstWatchedAt`
- `internal/repository/title.go`: update `GetByID` scan, add `UpdateFirstWatchedAt` or fold into existing `Update`
- Frontend `types.ts` + components reading `watched_at`: rename to `first_watched_at`, add `last_watched_at`
- `internal/handler/history.go` / activity queries: use `last_watched_at` for ordering

### Files
- `internal/database/migrations/018_dual_timestamps.up.sql`
- `internal/database/migrations/018_dual_timestamps.down.sql`
- `internal/model/episode.go`, `internal/model/title.go`
- `internal/repository/episode.go`, `internal/repository/title.go`
- `internal/service/library.go` (MarkEpisodesWatched, ToggleEpisodeWatched)
- `internal/service/plex.go` (UpdateLastWatchedAt calls)
- Frontend: `types.ts`, `TitleDetail.tsx` / `TitleHistory.tsx` (if displaying watched_at)

### Tests
- Existing episode/title repo tests: update expected column names
- New test: `BatchMarkWatched` on already-watched episode preserves `first_watched_at`, updates `last_watched_at`

---

## Task 2 — Plex rewatch tracking + webhook logging

### Webhook logging (in `ProcessWebhook`)
One `log.Printf` per inbound webhook before any routing:
```
plex webhook: event=%s type=%s title=%q season=%d episode=%d ratingKey=%s
```

### Rewatch handling

**`internal/service/plex.go`**:
- Rename `ProcessScrobble` → `ProcessWebhook` (keep `ProcessScrobble` as 1-line shim for existing test compat)
- Route: `media.scrobble` → existing `handleScrobble`, `media.play` → new `handlePlay`, else → return nil
- `handlePlay`: only handles episodes (movies always get scrobble from Plex)

**`handleEpisodePlayInTx(tx, meta, rawPayload)`**:
1. `FindByExternalID` by grandparentKey — if not found → return nil (not a tracked title)
2. `seasons.GetOrCreate` + `episodes.GetOrCreate`
3. If `!ep.Watched` → return nil (unwatched = wait for scrobble, don't mark on play-start)
4. `events.Create` with source=`plex`, payload stored
5. `episodes.UpdateLastWatchedAt(ep.ID)` — updates only `last_watched_at` (not `first_watched_at`)
6. `titles.UpdateLastWatchedAt(title.ID, now)`
7. NO status change, NO auto-complete, NO rating prompt, NO total_watch_minutes increment

**`internal/handler/webhook.go`**: call `ProcessWebhook` instead of `ProcessScrobble`

### Files
- `internal/service/plex.go`
- `internal/handler/webhook.go`
- `internal/repository/episode.go` (add `UpdateLastWatchedAt` method for episodes)
- `internal/service/plex_test.go` — new tests:
  - `TestPlexService_PlayIgnoredForUnwatchedEpisode`
  - `TestPlexService_PlayCreatesRewatchEvent`
  - `TestPlexService_PlayNoAutoComplete`

---

## Task 3 — Makefile fix

No data fix for ep 12 — full DB reset + Simkl reimport planned for May 2026.

### Fix `ssh-db-pull` in Makefile
Also copy WAL and SHM files so pulled DBs include recent un-checkpointed writes.

### Files
- `Makefile`

---

## Execution Order

1. **Task 1** (migration + model/repo changes) — foundation, must land first
2. **Task 2** (rewatch handling + logging) — depends on Task 1's dual timestamps
3. **Task 3** (Makefile WAL fix) — independent, can land anytime

## Verification
- `make test` + `make test-front` pass after each task
- Deploy Task 1+2, then rewatch a Poirot episode on Plex → confirm:
  - NAS logs show `plex webhook: event=media.play ...`
  - Poirot rises to top of "last watched" list
  - Watch event created in history
  - `first_watched_at` preserved, `last_watched_at` updated
