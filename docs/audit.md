# PlexTracker — Code Audit

Generated: 2026-04-09. Scope: Go backend, SQLite, Preact/TS frontend.
Format: `[ID] SEV | file:line — fix`
Sessions ordered by priority (highest first). Work top-to-bottom across sessions.

---

## SESSION-1: Correctness bugs
*Bugs that silently produce wrong output. Fix before anything else.*

- ~~[GO-14] HIGH | `service/matching/gemini.go:159`~~ — **already fixed** (`bytes.NewReader` was already inside the loop)
- ~~[GO-20] LOW | `service/simkl.go:122`~~ — **fixed** (04dcfda): `dryRun` threaded through `importItem`, DB writes skipped when true
- ~~[DB-16] LOW | `repository/title_search.go:127`~~ — **fixed** (04dcfda): `defer rows.Close()` added after `Query()`, manual calls removed
- ~~[DB-17] LOW | `repository/title.go:750`~~ — **fixed** (04dcfda): `defer` removed from `mergeInTx`, explicit close after loop + on scan error

---

## SESSION-2: Goroutine lifecycle & graceful shutdown
*Goroutines that cannot be stopped on SIGTERM. Fix as a unit — all require threading a cancellable context from serve.go.*

- ~~[GO-1] HIGH | `service/plex.go:259`~~ — **fixed**: service-level context stored on `PlexService`, goroutine selects on `ctx.Done()` before running pipeline. Note: goroutines already in-flight at SIGTERM run to completion because `Pipeline.Run` is not context-aware; complete cancellation of in-flight HTTP calls addressed by SESSION-8 GO-7
- ~~[GO-2] HIGH | `service/background.go:402`~~ — **fixed**: `StartTicker` accepts `ctx context.Context`, initial delay and ticker loop both select on `ctx.Done()`
- ~~[GO-3] HIGH | `service/taskqueue.go:111`~~ — **fixed**: panic recovery replaced with `for` loop + inner func; checks `ctx.Err()` after recovery instead of recursing
- ~~[GO-15] LOW | `cmd/serve.go:84`~~ — **fixed**: `signal.NotifyContext` with SIGINT/SIGTERM threaded to worker, bgSvc, and PlexService via router

---

## SESSION-3: Race conditions
*Non-deterministic behavior under concurrent use.*

- ~~[GO-13] MEDIUM | `service/matching/gemini.go:154`~~ — **fixed** (a3bc22a): `keyIndex.Add(1)` before use; constructor initialises to `len-1` so first call lands on index 0
- ~~[GO-4] HIGH | `service/taskqueue.go:140`~~ — **fixed** (a3bc22a): `processDueTasks` creates `context.WithTimeout(ctx, 5*time.Minute)` per task; ctx threaded into `ProcessTask` and `handleEnrichment`
- ~~[FE-1] HIGH | `store.ts:150`~~ — **fixed** (a3bc22a): `set(state => ...)` callback reads `state.titles` instead of stale closure variable
- ~~[FE-2] HIGH | `store.ts:191`~~ — **fixed** (a3bc22a): `_searchGen` counter incremented before each fetch; stale responses discarded on resolve/reject
- ~~[FE-4] HIGH | `hooks/useApi.ts:16`~~ — **fixed** (a3bc22a): `AbortController` created per load call; aborted in `useEffect` cleanup

---

## SESSION-4: Database query performance
*Bottlenecks that scale poorly with library size.*

- ~~[DB-1] HIGH | `repository/title_search.go:269`~~ — **fixed**: `fuzzySearch` now JOINs `titles` and applies status/type/isAnime/matchStatus filters before scanning rows, eliminating full table scan when filters are active
- ~~[DB-2] HIGH | `service/background.go:254`~~ — **fixed**: `SeasonRepository.Upsert` and `EpisodeRepository.UpsertBatch` collapse GetOrCreate + Update into single `INSERT … ON CONFLICT DO UPDATE` per season/episode batch; `refreshSeriesFromTMDB` uses them
- ~~[DB-3] HIGH | `service/background.go:280`~~ — **fixed**: `allEpisodesWatched` now uses `season.Episodes` (already loaded by `GetByID`) instead of calling `GetBySeasonID` per season
- [DB-5] MEDIUM | `repository/title.go:449` — `ListAll` unbounded SELECT loads entire library into memory; acceptable for personal use today, note for growth (streaming cursor or paginated batch)
- ~~[DB-12] MEDIUM | `repository/watch_event.go:51`~~ — **fixed**: `ListByTitle` now has `LIMIT 500`

---

## SESSION-5: Missing database indexes
*Single migration. Fix all at once.*

- ~~[DB-9] MEDIUM | `migrations/`~~ — **fixed**: `idx_title_names_title_id_primary ON title_names(title_id, is_primary)` added in migration 012
- ~~[DB-10] MEDIUM | `migrations/`~~ — **fixed**: `idx_episodes_watched_at ON episodes(watched, watched_at)` added in migration 012
- ~~[DB-11] MEDIUM | `migrations/`~~ — **fixed**: `idx_watch_events_episode_id ON watch_events(episode_id)` added in migration 012
- ~~[DB-13] MEDIUM | `repository/stats.go:469,474,479`~~ — **fixed**: `strftime` replaced with `>= yearStart AND < yearEnd` range bounds; sargable on `created_at`/`watched_at`/`updated_at`
- ~~[DB-14] LOW | `migrations/`~~ — **fixed**: `idx_titles_last_watched_at` dropped in migration 012 (superseded by `idx_titles_last_watched_at_desc` from 011)

---

## SESSION-6: Database transaction & round-trip correctness
*Multi-step writes without transactions; avoidable round-trips.*

- ~~[DB-6] MEDIUM | `repository/title.go:674`~~ — **fixed**: `ReplaceNames` wrapped in `database.WithTx`; loop of INSERTs replaced with single batch INSERT
- ~~[DB-7] MEDIUM | `repository/task.go:83`~~ — **fixed**: `FetchDue` uses single `UPDATE … WHERE id IN (?)` instead of one UPDATE per task
- ~~[DB-8] MEDIUM | `repository/task.go:121`~~ — **fixed**: `Fail` uses `UPDATE … RETURNING attempts, max_attempts, day` to collapse UPDATE + SELECT to one round-trip
- ~~[DB-15] LOW | `repository/episode.go:43`~~ — **fixed**: `ToggleWatched` uses `UPDATE … RETURNING *` to collapse SELECT → UPDATE → SELECT to one round-trip

---

## SESSION-7: Security hardening
*Fix high/medium items as a batch; low items optional.*

- ~~[SEC-3] MEDIUM | `router/router.go:92`, `handler/webhook.go:28`~~ — **fixed**: `RedactingLogger` middleware replaces chi Logger; webhook path `/api/webhook/plex/` prefix is redacted to `[redacted]` in all log output
- ~~[SEC-4] MEDIUM | `router/router.go:92`~~ — **fixed**: webhook endpoint wrapped with `RateLimit(60, time.Minute)`
- ~~[SEC-5] MEDIUM | `middleware/ratelimit.go:28`~~ — **fixed**: `middleware.RealIP` added as first global middleware; rate limiter now sees real client IP via `r.RemoteAddr`
- ~~[SEC-1] MEDIUM | `middleware/security.go:6`~~ — **fixed**: `Strict-Transport-Security: max-age=63072000; includeSubDomains` added
- ~~[SEC-2] MEDIUM | `middleware/security.go:17`~~ — **fixed**: `img-src` restricted to `'self'` (no data URIs used in frontend)
- ~~[SEC-11] LOW | `handler/cover.go`~~ — **already done**: `Serve` rejects filenames containing `..`, `/`, or `\` before `filepath.Join`
- ~~[SEC-6] LOW | `middleware/ratelimit.go:9`~~ — **fixed**: map capped at 10 000 IPs; new IPs beyond cap are denied (fail closed)
- ~~[SEC-10] LOW | `config/config.go:54`~~ — **fixed**: `COOKIE_SECURE` env var added; falls back to `!DebugLogin` when unset

---

## SESSION-8: Go error handling & context propagation
*Silent failures and missing cancellation throughout backend.*

- ~~[GO-9] MEDIUM | `service/plex.go:316`, `background.go:531,545`, `taskqueue.go:286`, `simkl.go:286`~~ — **fixed**: `payload, err := json.Marshal(...)` now checked; logs and returns on error
- ~~[GO-10] MEDIUM | `service/library.go:102`~~ — **fixed**: `BatchCreate` error is now logged
- ~~[GO-11] MEDIUM | `service/library.go:73,104,128`~~ — **fixed**: `GetByID` errors checked and returned
- ~~[GO-18] LOW | `handler/title.go:306`~~ — **fixed**: `Merge` now uses `httputil.ReadJSON` with size limit
- ~~[GO-5] MEDIUM | `service/background.go:103,275,493`~~ — **fixed**: `ctx` threaded through `RefreshTitles`, `refreshSeriesFromTMDB`, `CleanupUnusedCovers`; all `limiter.Wait` calls use passed ctx
- ~~[GO-6] MEDIUM | `service/taskqueue.go:150`~~ — **already fixed in SESSION-3**: `processDueTasks` already accepted `ctx` and used `limiter.Wait(ctx)`
- ~~[GO-7] MEDIUM | `service/matching/tmdb.go:37`, `anilist.go:38`, `gemini.go:158`~~ — **fixed**: `get`/`query`/`generate` accept `ctx` and use `http.NewRequestWithContext`; all callers (pipeline, taskqueue, background, handlers) thread request/service context through

---

## SESSION-9: Frontend error handling & TypeScript correctness
*Silent failures and unsafe types in frontend.*

- ~~[FE-3] HIGH | `hooks/usePush.ts:20`~~ — **fixed**: `.catch()` added to SW registration; logs to console.error
- ~~[FE-17] MEDIUM | `pages/TitleDetail.tsx:81`~~ — **fixed**: `handleMarkNext`, `handleSaveRating`, `handleSaveEdit` wrapped in try/catch; `actionError` state surfaced via `ErrorBanner`
- ~~[FE-18] MEDIUM | `components/TitleCard.tsx:52`~~ — **fixed**: catch added to `handleQuickMark`; logs error to console
- ~~[FE-19] MEDIUM | `pages/Search.tsx:65`~~ — **fixed**: `mergeError` state set on failure, displayed in merge BottomSheet
- ~~[FE-20] MEDIUM | `app.tsx:33`~~ — **fixed**: `.catch()` added to `/api/config` fetch; logs to console.error
- ~~[FE-7] MEDIUM | `components/BottomSheet.tsx:17,21`, `FilterDrawer.tsx:108,113`, `ActionDrawer.tsx:26,31`~~ — **fixed**: touch handlers typed `(e: TouchEvent)`
- ~~[FE-8] MEDIUM | `pages/Validate.tsx:86,113`~~ — **fixed**: `RematchPayload` and `AddTitlePayload` interfaces defined; `any` replaced
- ~~[FE-9] MEDIUM | `pages/Validate.tsx:57`~~ — **fixed**: `handleSearch` typed as `(e: SubmitEvent)`

---

## SESSION-10: Frontend performance & re-renders
*Unnecessary renders and missing memoization.*

- ~~[FE-10] MEDIUM | `components/TitleCard.tsx:38`~~ — **fixed**: `useTitleStore(s => s.sort.field === 'last_watched_at')` selector
- ~~[FE-11] MEDIUM | `pages/Library.tsx:72`~~ — **fixed**: granular selectors per field
- ~~[FE-12] MEDIUM | `pages/TitleDetail.tsx:19,67`~~ — **fixed**: `sortedSeasons` and `next` wrapped in `useMemo` keyed on `title?.id` / `title?.seasons`; memos placed before early return
- ~~[FE-5] MEDIUM | `pages/Library.tsx:78`~~ — **fixed**: `fetchTitles` added to `useEffect` deps
- ~~[FE-6] MEDIUM | `pages/Search.tsx:33`~~ — **fixed**: `search` added to `useEffect` deps
- ~~[FE-21] MEDIUM | `store.ts:93,132`~~ — **fixed**: `buildFilterParams(filter, sort?)` extracted; `TitleFilter` type extracted
- ~~[FE-22] LOW | `utils.ts:34`, `pages/TitleDetail.tsx:43`~~ — **fixed**: local `formatDate` removed from `TitleDetail`; `utils.ts` locale aligned to `en-GB`
- ~~[FE-23] LOW | `components/EditSheet.tsx:27`~~ — **fixed**: `useEffect(() => { ... }, [title.id])` resets local state on title change
- ~~[FE-24] LOW | `app.tsx:46`~~ — **fixed**: `navigate` wrapped in `useCallback([setFilter])`; `defaultFilter` moved to module scope
- ~~[FE-25] LOW | `pages/Search.tsx:71`~~ — **fixed**: `getMetadata` moved to module scope

---

## SESSION-11: Accessibility
*Interactive elements not keyboard-reachable.*

- ~~[FE-13] MEDIUM | `components/EpisodeRow.tsx:41`~~ — **fixed**: `<div onClick={handleToggle}>` changed to `<button type="button">`; CSS reset added to `.toggle`
- ~~[FE-14] MEDIUM | `components/TitleCard.tsx:99`~~ — **fixed**: quick-mark `<div>` changed to `<button type="button" aria-label="Mark E{n} as watched">`; CSS reset added to `.badge`
- ~~[FE-15] MEDIUM | `pages/Search.tsx:241`~~ — **fixed**: SVG wrapped in `<button type="button" aria-label="Clear search">`; CSS reset added to `.clearBtn`
- ~~[FE-16] MEDIUM | `pages/Library.tsx:39`~~ — **fixed**: banner `<div onClick>` changed to `<button type="button">`; CSS reset + `width:100%; text-align:left` added to `.bannerWrapper`

---

## SESSION-12: Low-priority cleanup
*No functional impact. Batch or opportunistic.*

- ~~[GO-16] LOW | `service/taskqueue.go:368`, `matching/pipeline.go:458`~~ — **fixed**: `fmt.Sprintf("%s/covers", dir)` replaced with `filepath.Join(dir, "covers")` in both files
- ~~[GO-17] LOW | `handler/episode.go:14`~~ — **fixed**: unused fields (titles, episodes, events, settings, push, backfill) removed; constructor simplified to `(db, svc)`
- ~~[GO-19] LOW | `service/matching/pipeline.go:346`~~ — **fixed**: AniList search gated on `input.IsAnime || result.TitleType == TitleTypeSeries`
- ~~[FE-26] LOW | `vite.config.ts`~~ — **fixed**: `manualChunks` added for admin routes (Admin, AdminNotifications, AdminTasks)
- ~~[FE-27] LOW | `store.ts:24`~~ — **fixed**: `loadSort` validates `field` against `SORT_FIELDS` array and `order` against `'asc'|'desc'`
- ~~[FE-28] LOW | `components/BottomSheet.tsx:15`~~ — **fixed**: `useEffect(() => { if (!open) touchStartY.current = null }, [open])` added before early return
- [DB-18] INFO | `migrations/` — `idx_titles_cover_url` stores full file path; current usage correct, informational only
- [SEC-7] LOW | `handler/auth.go:49` — Google ID token sent to `tokeninfo` without local pre-validation; mitigated by rate limiter (10 req/min), no action required
- [SEC-8] LOW | `router/router.go:83` — dev backdoor in prod binary gated by env var; acceptable for single-developer, build tag would be stronger
- [SEC-9] LOW | `handler/auth.go:70,104` — 30-day JWT, no refresh or revocation; acceptable for personal app
- [SEC-12] INFO | `config/config.go:60` — weak JWT default allowed when `DebugLogin=true`; acceptable if `.env.local` always overrides in prod

---

## Summary table

| Session | Count | Top severity | Theme |
|---------|-------|-------------|-------|
| 1 — Correctness bugs | 4 | HIGH | Silent wrong output |
| 2 — Goroutine lifecycle | 4 | HIGH | Leak / shutdown |
| 3 — Race conditions | 5 | HIGH | Concurrency |
| 4 — DB query perf | 5 | HIGH | N+1 / full scans |
| 5 — Missing indexes | 5 | MEDIUM | Single migration |
| 6 — DB transactions | 4 | MEDIUM | Round-trips / atomicity |
| 7 — Security | 8 | MEDIUM | Headers / rate limit |
| 8 — Go errors & context | 7 | MEDIUM | Silent failures |
| 9 — FE error handling | 8 | HIGH | Silent failures / types |
| 10 — FE performance | 10 | MEDIUM | Re-renders / memos |
| 11 — Accessibility | 4 | MEDIUM | Keyboard nav |
| 12 — Cleanup | 11 | LOW | No functional impact |

**Total: 75 findings across 12 sessions.**
