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

- ~~[GO-1] HIGH | `service/plex.go:259`~~ — **fixed**: service-level context stored on `PlexService`, goroutine selects on `ctx.Done()` before running pipeline. Note: goroutines already in-flight at SIGTERM run to completion because `Pipeline.Run` is not context-aware; complete cancellation of in-flight HTTP calls requires SESSION-8 GO-7 work
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

- [DB-1] HIGH | `repository/title_search.go:269` — `SELECT id, title_id, name, language FROM title_names` with no WHERE; loads every name row into memory for Levenshtein scoring. Add `JOIN titles ON title_id = titles.id` to filter by type/anime, and/or `WHERE LENGTH(name) BETWEEN ? AND ?` bounds
- [DB-2] HIGH | `service/background.go:254` — `refreshSeriesFromTMDB` loops O(seasons × episodes) individual DB writes: `GetOrCreate` + `UpdateTotalEpisodes` per season, `GetOrCreate` + `UpdateMetadata` per episode. Batch with `INSERT OR IGNORE … ON CONFLICT DO UPDATE` per season
- [DB-3] HIGH | `service/background.go:280` — `allEpisodesWatched` calls `episodes.GetBySeasonID(season.ID)` per season in a loop (N+1). Pass `title.Seasons[i].Episodes` directly (already loaded from `GetByID`) or load all episodes in one query keyed by season IDs
- [DB-5] MEDIUM | `repository/title.go:449` — `ListAll` unbounded SELECT loads entire library into memory; acceptable for personal use today, note for growth (streaming cursor or paginated batch)
- [DB-12] MEDIUM | `repository/watch_event.go:51` — `ListByTitle` no LIMIT; a heavily-watched title returns unbounded rows. Add default LIMIT (e.g. 500) or accept a limit param

---

## SESSION-5: Missing database indexes
*Single migration. Fix all at once.*

- [DB-9] MEDIUM | `migrations/` — no index on `title_names(title_id, is_primary)`; several queries filter `tn.is_primary = 1`. Add `CREATE INDEX idx_title_names_title_id_primary ON title_names(title_id, is_primary)`
- [DB-10] MEDIUM | `migrations/` — no `(watched, watched_at)` index on `episodes`; stats COUNT queries filter both cols without `season_id`. Existing `(season_id, watched)` index from 011 doesn't cover global counts. Add standalone index
- [DB-11] MEDIUM | `migrations/` — `watch_events.episode_id` FK has no index; `ON DELETE SET NULL` scans the entire table per episode delete. Add `CREATE INDEX idx_watch_events_episode_id ON watch_events(episode_id)`
- [DB-13] MEDIUM | `repository/stats.go:469,474,479` — `WHERE strftime('%Y', created_at) = ?` not sargable; replace with `WHERE created_at >= 'YYYY-01-01' AND created_at < 'YYYY+1-01-01'` range on raw column
- [DB-14] LOW | `migrations/` — `idx_titles_last_watched_at` (migration 008) is redundant once `idx_titles_last_watched_at_desc` (011) exists. Drop 008 version in a migration

---

## SESSION-6: Database transaction & round-trip correctness
*Multi-step writes without transactions; avoidable round-trips.*

- [DB-6] MEDIUM | `repository/title.go:674` — `ReplaceNames` DELETEs then INSERTs in a loop without a wrapping transaction; failure mid-loop leaves title nameless. Wrap in `database.WithTx` + batch INSERTs
- [DB-7] MEDIUM | `repository/task.go:83` — `FetchDue` fires individual `UPDATE … SET status='running' WHERE id=?` per task after one SELECT; replace with single `UPDATE … WHERE id IN (?)`
- [DB-8] MEDIUM | `repository/task.go:121` — `Fail` does UPDATE then SELECT to read new attempts count; use `UPDATE … RETURNING attempts, max_attempts` (SQLite 3.35+) to collapse to one round-trip
- [DB-15] LOW | `repository/episode.go:43` — `ToggleWatched` does SELECT → UPDATE → SELECT (3 round-trips); use `UPDATE … SET … WHERE id=? RETURNING *`

---

## SESSION-7: Security hardening
*Fix high/medium items as a batch; low items optional.*

- [SEC-3] MEDIUM | `router/router.go:92`, `handler/webhook.go:28` — webhook secret is a URL path segment logged verbatim by chi Logger; move auth to `X-Plex-Token` header or redact path before logging
- [SEC-4] MEDIUM | `router/router.go:92` — webhook endpoint outside all rate-limit groups; add a basic rate limiter
- [SEC-5] MEDIUM | `middleware/ratelimit.go:28` — `r.RemoteAddr` is always the proxy IP behind reverse proxy; use `chi/middleware.RealIP` before rate limiter to extract `X-Real-IP`/`X-Forwarded-For`
- [SEC-1] MEDIUM | `middleware/security.go:6` — missing `Strict-Transport-Security` header; add `max-age=63072000; includeSubDomains`
- [SEC-2] MEDIUM | `middleware/security.go:17` — `img-src 'self' data:` too broad (data URI exfil via CSS injection); restrict to `img-src 'self'` unless data URIs strictly needed
- [SEC-11] LOW | `handler/cover.go` — verify `filepath.Join` + base-dir validation in `Serve` prevents `../../` traversal; reject filenames containing `/` or `..`
- [SEC-6] LOW | `middleware/ratelimit.go:9` — `attempts` map unbounded under distributed attack; cap map size or use fixed-capacity LRU
- [SEC-10] LOW | `config/config.go:54` — `CookieSecure = !DebugLogin` couples TLS flag to debug flag; add explicit `COOKIE_SECURE` env var

---

## SESSION-8: Go error handling & context propagation
*Silent failures and missing cancellation throughout backend.*

- [GO-9] MEDIUM | `service/plex.go:316`, `background.go:531,545`, `taskqueue.go:286`, `simkl.go:286` — `payload, _ := json.Marshal(...)` swallows errors; check error before calling Enqueue
- [GO-10] MEDIUM | `service/library.go:102` — `_ = events.BatchCreate(watchEvents)` drops error silently; log or propagate
- [GO-11] MEDIUM | `service/library.go:73,104,128` — `title, _ := titles.GetByID(...)` post-transaction; check error and return it
- [GO-18] LOW | `handler/title.go:306` — `Merge` uses bare `json.NewDecoder(r.Body)` without `LimitReader`; use `httputil.ReadJSON`
- [GO-5] MEDIUM | `service/background.go:103,275,493` — `limiter.Wait(context.Background())` ignores cancellation; thread context through `refreshTitles`, `refreshSeriesFromTMDB`, `CleanupUnusedCovers`
- [GO-6] MEDIUM | `service/taskqueue.go:150` — same pattern; pass `ctx` through to `processDueTasks`
- [GO-7] MEDIUM | `service/matching/tmdb.go:37`, `anilist.go:38`, `gemini.go:158` — HTTP calls use `http.NewRequest` not `http.NewRequestWithContext`; context cancellation (shutdown, abort) does not interrupt in-flight calls. Accept `ctx` in `get`/`query`/`generate` and use `http.NewRequestWithContext`

---

## SESSION-9: Frontend error handling & TypeScript correctness
*Silent failures and unsafe types in frontend.*

- [FE-3] HIGH | `hooks/usePush.ts:20` — SW registration `.then(...)` has no `.catch()`; add error handler (console.error minimum, ideally surface in UI)
- [FE-17] MEDIUM | `pages/TitleDetail.tsx:81` — `handleMarkNext`, `handleSaveRating`, `handleSaveEdit` have no catch; add try/catch and local error state
- [FE-18] MEDIUM | `components/TitleCard.tsx:52` — `handleQuickMark` try/finally no catch; add catch + user feedback (e.g. revert optimistic update)
- [FE-19] MEDIUM | `pages/Search.tsx:65` — merge failure only `console.error`; surface error in merge BottomSheet
- [FE-20] MEDIUM | `app.tsx:33` — `/api/config` fetched with raw `fetch`, no error handling; use `apiFetch('/config')` or add `.catch()`
- [FE-7] MEDIUM | `components/BottomSheet.tsx:17,21`, `FilterDrawer.tsx:108,113`, `ActionDrawer.tsx:26,31` — touch handlers typed `(e: any)`; type as `(e: TouchEvent)`
- [FE-8] MEDIUM | `pages/Validate.tsx:86,113` — `const body: any = {}`; define typed interface for add/rematch payloads
- [FE-9] MEDIUM | `pages/Validate.tsx:57` — `handleSearch: (e: any)`; type as `(e: SubmitEvent)`

---

## SESSION-10: Frontend performance & re-renders
*Unnecessary renders and missing memoization.*

- [FE-10] MEDIUM | `components/TitleCard.tsx:38` — `useTitleStore()` subscribes to full store; re-renders on any store change. Use selector: `useTitleStore(s => s.sort.field)`
- [FE-11] MEDIUM | `pages/Library.tsx:72` — full store destructure causes render on every intermediate loading toggle; use granular selectors per field
- [FE-12] MEDIUM | `pages/TitleDetail.tsx:19,67` — `getNextUnwatched(title)` + 3 `.sort()` calls run on every render; wrap in `useMemo` keyed on `title.id` / `title.seasons`
- [FE-5] MEDIUM | `pages/Library.tsx:78` — `useEffect(() => { fetchTitles() }, [])` missing `fetchTitles` dep; add it (stable Zustand ref, no extra fetch)
- [FE-6] MEDIUM | `pages/Search.tsx:33` — `useEffect` uses `search` without listing it as dep; add `search` to array
- [FE-21] MEDIUM | `store.ts:93,132` — 10-line URLSearchParams block duplicated in `fetchTitles` and `loadMore`; extract `buildFilterParams(filter, sort?)` helper
- [FE-22] LOW | `utils.ts:34`, `pages/TitleDetail.tsx:43` — `formatDate` duplicated with different locale (`en-US` vs `en-GB`); delete local copy, import from utils, align locale
- [FE-23] LOW | `components/EditSheet.tsx:27` — local state not reset when `title` prop changes; add `useEffect(() => { ... }, [title.id])` to sync
- [FE-24] LOW | `app.tsx:46` — `navigate` recreated each render, passed to Navbar as prop; wrap in `useCallback([setFilter])`
- [FE-25] LOW | `pages/Search.tsx:71` — `getMetadata` closes over nothing but is redeclared each render; move outside component

---

## SESSION-11: Accessibility
*Interactive elements not keyboard-reachable.*

- [FE-13] MEDIUM | `components/EpisodeRow.tsx:41` — `<div onClick={handleToggle}>` not keyboard-accessible; change to `<button>` (or add `role="checkbox"`, `tabIndex={0}`, `onKeyDown` for Enter/Space)
- [FE-14] MEDIUM | `components/TitleCard.tsx:99` — quick-mark `<div onClick={handleQuickMark}>`; change to `<button>`
- [FE-15] MEDIUM | `pages/Search.tsx:241` — SVG clear button no role/label; wrap in `<button aria-label="Clear search">`
- [FE-16] MEDIUM | `pages/Library.tsx:39` — match-review banner `<div onClick>` not a button/link; use `<button>` or add `role="button"`, `tabIndex={0}`, `onKeyDown`

---

## SESSION-12: Low-priority cleanup
*No functional impact. Batch or opportunistic.*

- [GO-16] LOW | `service/taskqueue.go:368`, `matching/pipeline.go:458` — `fmt.Sprintf("%s/covers", dir)` for paths; use `filepath.Join`
- [GO-17] LOW | `handler/episode.go:14` — unused injected fields (titles, episodes, events, settings, push, backfill); handler delegates to service. Remove unused fields and simplify constructor
- [GO-19] LOW | `service/matching/pipeline.go:346` — AniList search runs for every title without a known AniListID, including confirmed non-anime movies; gate on `input.IsAnime || result.TitleType == TitleTypeSeries`
- [FE-26] LOW | `vite.config.ts` — no code-splitting; add `manualChunks` for admin routes to reduce initial bundle
- [FE-27] LOW | `store.ts:24` — `loadSort` accepts any string for `field`; validate against known `SortField` union values
- [FE-28] LOW | `components/BottomSheet.tsx:15` — drag state not reset when `open` goes false; add `useEffect(() => { touchStartY.current = null }, [open])`
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
