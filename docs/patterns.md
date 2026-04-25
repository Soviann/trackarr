# Patterns & Codebase Map

Update when adding routes, services, components, or commands.

## Backend (Go)

### CLI Commands

| Command | File | Purpose |
|---|---|---|
| `serve` | `cmd/serve.go` | HTTP server (configurable with `DISABLE_BACKGROUND_TASKS`) |
| `import` | `cmd/import.go` | Simkl backup import |

### Models

`internal/model/` — Title (TitleType, TitleStatus, SeriesStatus, MatchStatus, NextEpisode; `total_watch_minutes`), TitleName, Season (EpisodeCount, WatchedCount), Episode, WatchEvent (WatchEventSource), Setting.

### Services

| Service | File | Purpose |
|---|---|---|
| PlexService | `internal/service/plex.go` | Webhook scrobble, delegates to TitleService + LibraryService |
| TitleService | `internal/service/title.go` | Title logic (creation, rematching, URL resolution, merging) |
| LibraryService | `internal/service/library.go` | User library (marking watched, auto-complete, rating, notifications) |
| BackfillService | `internal/service/backfill.go` | Episode backfill (metadata fetch, mark previous). **Opens own writeDB tx — never call from inside another tx; fire post-commit via `LibraryService.TriggerBackfillForEpisode`.** |
| AniListPushService | `internal/service/anilist_push.go` | Per-season / per-movie push of status, progress, score to AniList. Silently skips on missing token, missing season mapping, or 401 (flags token invalid). Enqueued via `TaskTypeAniListPushSeason` / `TaskTypeAniListPushMovie`. |
| PushNotifier | `internal/service/push.go` | Web Push VAPID (interface: PushService + noopNotifier) |
| BackgroundService | `internal/service/background.go` | Daily refresh (TMDB sync, auto-complete, push triggers, per-season AniList community score via `season_external_ids` mappings — 401 flags `anilist_token_invalid` and aborts remaining calls) |
| SimklImporter | `internal/service/simkl.go` | Simkl backup import (zip/JSON) |
| Pipeline | `internal/service/matching/pipeline.go` | Orchestrates matching Steps 1-5. URL resolution (TMDB, IMDb, AniList, TVDB slugs) |
| TMDBClient | `internal/service/matching/tmdb*.go` | TMDB API: search, details, covers, find-by-id |
| TVDBClient | `internal/service/matching/tvdb*.go` | TVDB v4: details, covers, search, slug resolution (JWT auth) |
| AniListClient | `internal/service/matching/anilist*.go` | AniList GraphQL: search, episodes, rating sync, covers |
| CrossRefDB | `internal/service/matching/crossref.go` | anime-offline-database ID cross-referencing |
| GeminiClient | `internal/service/matching/gemini.go` | AI match verification, fuzzy resolve, anime season ID, key rotation |

### Matching Pipeline Steps

1. **Plex IDs** — TMDB/IMDB from webhook metadata → `confirmed`
2. **Cross-reference** — anime-offline-database lookup → `confirmed`
3. **TMDB search** — by title+year → Step 5
4. **AniList search** — anime only → Step 5
5. **Gemini verification** — high confidence → `pending_review`, low → `unconfirmed`

Confidence: `ConfidenceHigh`/`Medium`/`Low` (constants in `pipeline.go`). Nil clients are skipped (graceful degradation). Each step sets `MatchSource` (`plex_ids`, `crossref`, `tmdb_search`, `anilist_search`, `gemini_fuzzy`, `none`).

After matching: parallel TMDB + TVDB fetch → fusion (overview: longest; genres: union; cover: TMDB > TVDB > AniList). TVDB URL resolution via `ParseURLFull()` → slug → numeric ID.

### AniList per-season push

**Data model.** `season_external_ids(season_id, provider, external_id)` stores per-provider external IDs per season. Accessed via `SeasonExternalIDRepository` (`Get`, `Set`, `Delete`, `ListForTitle`) and `SeasonExternalIDWriter` (`Stamp` — first-writer-wins). `titles.anilist_id` is retained for movies and title-level display but is no longer the canonical push target for multi-season anime.

**Push service.** `service.AniListPushService` exposes `PushSeasonState(ctx, seasonID)` and `PushMovieState(ctx, titleID)`. Both derive state from DB, skip silently when no mapping / no token / token flagged invalid, and absorb HTTP 401 by setting `settings.anilist_token_invalid = 'true'`.

**Status derivation** (`service.DeriveSeasonState`):
- All episodes watched → `COMPLETED` (wins over dropped)
- `title.status = dropped` + not fully watched → `DROPPED`
- `watchedEpisodes > 0` but not all → `CURRENT`
- `watchedEpisodes == 0` + `title.status = plan_to_watch` → `PLANNING`
- otherwise → `CURRENT`

**Rating guard** (`service.ShouldPushRating`): score is included in the mutation only when derived status is `COMPLETED` or `DROPPED`.

**Triggers** (all via task queue `anilist_push_season` / `anilist_push_movie`):
- Episode watched/unwatched → `LibraryService.MarkEpisodeWatched` enqueues.
- Title status change → `TitleHandler.Patch` enqueues for every season.
- Title rating change → `TitleHandler.Patch` enqueues for seasons whose derived status is `COMPLETED`/`DROPPED`.
- Season AniList-ID set via `PUT /titles/{titleID}/seasons/{seasonID}/anilist` → handler enqueues immediately.

**Token expiry.** `settings.anilist_token_invalid = 'true'` suppresses further pushes. The Settings screen surfaces a "Reconnect AniList" banner. Successful OAuth reconnect clears the flag (`anilist_auth.SaveToken` calls `settings.Delete("anilist_token_invalid")`).

**UI (Concept B).** Active-season info strip between the progress line and the episode grid. Component: `frontend/src/components/SeasonAniListStrip.tsx`. Multi-season titles hide the title-level AniList tab in the action bar; movies and single-season titles keep it. Per-season fix-match via the pencil ✎ in the strip (or "Link entry" CTA on unmapped seasons) opens `RematchSheet` with `seasonID` context — saving calls `PUT /titles/{titleID}/seasons/{seasonID}/anilist`, removing calls `DELETE` on the same route.

### Repositories

`internal/repository/` — Read methods accept `database.DBTX` (pool or tx). Write methods live on typed writer structs (e.g. `TitleWriter`, `SeasonWriter`, `EpisodeWriter`, `WatchEventWriter`) that take `*sql.Tx` in their constructor; the compiler rejects any attempt to write outside a transaction. Callers wrap in `database.WithTxContext(ctx, pool, func(tx *sql.Tx) error { ... })` and build writers from `tx`.

Test writes go through `internal/testutil` helpers (`CreateTitle`, `UpdateTitle`, `MergeTitles`, `GetOrCreateSeason`, `GetOrCreateEpisode`, `CreateWatchEvent`, …) which wrap the matching writer in a short tx — no test needs to repeat the boilerplate.

| Repository | Reader (DBTX) | Writer (*sql.Tx, ctx) |
|---|---|---|
| Title | `List`, `ListAll`, `GetByID`, `FindByExternalID`, search in `title_search.go` | `Create`, `Update`, `UpdateLastWatchedAt`, `ReplaceNames`, `Merge`, `Delete`, `BatchDelete`, `BatchUpdateStatus` |
| Season | `GetByID` | `GetOrCreate`, `UpdateRating`, `UpdateTotalEpisodes`, `Upsert` |
| Episode | `GetBySeasonID` | `GetOrCreate`, `ToggleWatched`, `BatchMarkWatched`, `UpdateMetadata`, `UpsertBatch`, `MarkWatched`, `UpdateLastWatchedAt` |
| WatchEvent | `CountByTitleID`, `ListByTitle` | `Create`, `BatchCreate` |
| Task | `GetByID`, `ListPending`, `ListDead`, `ListPaginated`, `CountByStatus`, `CountDead` | `Enqueue`, `EnqueueWithDelay`, `FetchDue`, `Complete`, `Fail`, `RetryDead`, `ResetRunning`, `Delete`, `DeleteBatch`. Task kinds: `TaskTypeEnrichment`, `TaskTypeRefresh`, `TaskTypeCoverFetch`, `TaskTypeAniListPushSeason`, `TaskTypeAniListPushMovie`. |
| SeasonExternalID | `Get`, `ListForTitle` | `Set`, `Delete` on pool (`SeasonExternalIDRepository`); `Stamp` on tx (`SeasonExternalIDWriter`) — `Stamp` is first-writer-wins (`ON CONFLICT DO NOTHING`) and is the entry point used by the merge flow (`TitleWriter.Merge`) and S1 backfill (`stampSeasonAniListID` in `backfill.go`); `Set` overwrites (used by Phase 7 fix-match). Provider key constant: `repository.ProviderAniList`. Migration 020. |
| Genre | `ListWithCounts` | `ReplaceForTitle` |
| Setting | `Get` | `Set`, `Delete` |
| StatsRepository | `TotalWatchMinutes`, `TopGenres`, `CurrentStreak`, `BestStreak` |
| ActivityRepository | `List` (paginated watch events) |
| HistoryRepository | `GetByTitleID` (per-title watch log) |

TitleFilter: Limit/Offset/UpToDate/WatchingBehind/SeriesStatus/Sort/Order/Genres/GenreOp. Genres in `title_genres` join table (migration 016), loaded separately (MaxOpenConns=1).

### Database

`internal/database/` — `Open()`, `Migrate()`, `WithTx(db, fn)`, `WithTxContext(ctx, db, fn)`, `DBTX` (read contract: `Exec`/`Query`/`QueryRow`), `WriteDBTX` (write contract: `ExecContext`/`QueryContext`/`QueryRowContext` — only `*sql.Tx` satisfies it). SQLite with WAL, `MaxOpenConns=1`.

**Nested-tx deadlock rule:** any method receiving `tx *sql.Tx` MUST NOT call another service that opens its own writeDB tx (directly or via a repo on the pool) — the inner `BeginTx` waits forever for the sole connection. Post-commit side effects (backfill, rating push, webpush) are returned to the caller and fired AFTER `WithTxContext` returns. See `LibraryService.TriggerBackfillForEpisode` / `SendRatingPrompt` for the pattern.

### Handlers

`internal/handler/` — auth, title, episode, season_external, cover, webhook, push, anilist_auth, settings, stats, activity, history, tmdb, genre, admin, client_errors, spa. DI via struct with repos. `internal/handler/httputil/` — WriteJSON, ReadJSON, ParseIDParam, ParseQueryInt, APIError, HandlerFunc (`func(w,r) error`), WrapHandler.

### Routes

| Method | Path | Handler | Auth |
|---|---|---|---|
| GET | `/api/health` | Health | No |
| GET | `/api/config` | PublicConfig | No |
| POST | `/api/auth/google` | GoogleCallback | No (rate-limited) |
| POST | `/api/auth/dev` | DevLogin | No (debug only) |
| POST | `/api/auth/logout` | Logout | No |
| POST | `/api/webhook/plex/{secret}` | HandlePlex | No (secret in URL) |
| GET | `/api/covers/{filename}` | Serve | No |
| GET | `/api/titles` | List | Yes |
| GET | `/api/genres` | List | Yes |
| GET | `/api/titles/continue-watching` | ContinueWatching | Yes |
| GET | `/api/titles/upcoming` | Upcoming | Yes |
| GET | `/api/titles/review-count` | ReviewCount | Yes |
| POST | `/api/titles/batch-delete` | BatchDelete | Yes |
| POST | `/api/titles/batch-status` | BatchStatus | Yes |
| GET | `/api/titles/resolve` | Resolve | Yes |
| GET | `/api/titles/{id}` | GetByID | Yes |
| POST | `/api/titles` | Create | Yes |
| PATCH | `/api/titles/{id}` | Update | Yes |
| DELETE | `/api/titles/{id}` | Delete | Yes |
| POST | `/api/titles/{id}/rematch` | Rematch | Yes |
| POST | `/api/titles/{id}/merge` | Merge | Yes |
| POST | `/api/titles/{id}/refresh` | RefreshOne | Yes |
| GET | `/api/tmdb/search` | Search | Yes |
| PATCH | `/api/titles/{titleID}/episodes/{episodeID}` | ToggleWatched | Yes |
| POST | `/api/titles/{titleID}/episodes/batch-watch` | BatchMarkWatched | Yes |
| PUT | `/api/titles/{titleID}/seasons/{seasonID}/anilist` | SetAniListID | Yes |
| DELETE | `/api/titles/{titleID}/seasons/{seasonID}/anilist` | ClearAniListID | Yes |
| POST | `/api/push/subscribe` | Subscribe | Yes |
| DELETE | `/api/push/subscribe` | Unsubscribe | Yes |
| GET | `/api/stats` | Get | Yes |
| GET | `/api/stats/activity` | List | Yes |
| GET | `/api/titles/{id}/history` | Get | Yes |
| GET | `/api/settings` | Get | Yes |
| GET | `/api/anilist/auth` | Authorize | Yes |
| POST | `/api/anilist/token` | SaveToken | Yes |
| DELETE | `/api/anilist/token` | Disconnect | Yes |
| GET | `/api/admin/counts` | Counts | Yes |
| GET | `/api/admin/tasks` | ListTasks | Yes |
| POST | `/api/admin/tasks/{id}/retry` | RetryTask | Yes |
| DELETE | `/api/admin/tasks/{id}` | DeleteTask | Yes |
| POST | `/api/admin/tasks/batch-delete` | DeleteTasksBatch | Yes |
| GET | `/api/admin/notifications` | GetNotificationPrefs | Yes |
| PUT | `/api/admin/notifications` | UpdateNotificationPrefs | Yes |
| POST | `/api/admin/refresh-all` | RefreshAll | Yes |
| POST | `/api/client-errors` | Handle | Yes |

Source of truth for routes: `internal/router/router.go`. Read handler files for request/response shapes.

## Frontend (Preact)

Design tokens: `frontend/src/theme.ts` (JS) + `frontend/src/tokens.css` (CSS custom properties). CSS Modules for all components. `clsx` for conditional classes. Shared utils in `frontend/src/utils.ts`. API client in `frontend/src/api.ts`. Types in `frontend/src/types.ts`.

### Hooks

| Hook | File | Purpose |
|---|---|---|
| `useApi` | `hooks/useApi.ts` | Fetch wrapper with loading/error/mutate |
| `useTitleStore` | `store.ts` | Zustand: paginated title fetch, filter, sort (localStorage), loadMore, cache |
| `useSearchStore` | `store.ts` | Zustand: search results + scroll position persistence |
| `usePush` | `hooks/usePush.ts` | Service worker registration + push subscription |
| `useLongPress` | `hooks/useLongPress.ts` | Long-press detection (pointer events, configurable threshold/tolerance) |

### Components

| Component | File | Purpose |
|---|---|---|
| TitleHistory | `components/TitleHistory.tsx` | Watch history overlay per title |
| Navbar | `components/Navbar.tsx` | 4-tab bottom nav |
| FilterDrawer | `components/FilterDrawer.tsx` | Collapsible filter drawer (sort/status/type/series status/release date/genres) |
| TitleCard | `components/TitleCard.tsx` | Horizontal card with progress + quick mark badge |
| PosterCard | `components/PosterCard.tsx` | Poster grid card (2:3, gradient overlay) |
| StatusBadge | `components/StatusBadge.tsx` | Colored status pill |
| SeasonTab | `components/SeasonTab.tsx` | Season pill with progress/check |
| EpisodeRow | `components/EpisodeRow.tsx` | Episode row with toggle watched |
| ActionDrawer | `components/ActionDrawer.tsx` | Quick actions (next ep, rate, links) + manage (edit, fix match) |
| PullToRefresh | `components/PullToRefresh.tsx` | Pull-to-refresh (touch events, rubber-band, haptic) |
| SwipeActions | `components/SwipeActions.tsx` | Swipe-to-reveal action buttons |
| BottomSheet | `components/BottomSheet.tsx` | Slide-up sheet with drag-to-dismiss, body scroll lock |
| RatingPrompt | `components/RatingPrompt.tsx` | 10-star rating with save/IMDb/AniList |
| EditSheet | `components/EditSheet.tsx` | Edit type/status |
| AniListSheet | `components/AniListSheet.tsx` | AniList match confirm/fix |
| RematchSheet | `components/RematchSheet.tsx` | TMDB search + manual IDs to fix match (per-title or per-season AniList) |
| MatchReviewCard | `components/MatchReviewCard.tsx` | Match review with ID chips + confirm/fix |
| CoverPlaceholder | `components/CoverPlaceholder.tsx` | Type-colored gradient + icon for missing covers |
| CollapsibleSection | `components/CollapsibleSection.tsx` | Collapsible header with lazy-load `onExpand` |
| PosterStrip | `components/PosterStrip.tsx` | Horizontal scrollable poster strip |
| SeasonAniListStrip | `components/SeasonAniListStrip.tsx` | Active-season AniList info strip (community score, link, fix-match pencil) |
| ConfirmationDrawer | `components/ConfirmationDrawer.tsx` | Confirm/cancel drawer for destructive bulk actions |
| ErrorBanner | `components/ErrorBanner.tsx` | Inline error banner with optional retry |
| ErrorBoundary | `components/ErrorBoundary.tsx` | App-level React error boundary |

### Pages & Routes

| Route | Page | File |
|---|---|---|
| `/` | Library | `pages/Library.tsx` |
| `/search` | Search | `pages/Search.tsx` |
| `/add` | Add | `pages/Add.tsx` |
| `/stats` | Stats | `pages/Stats.tsx` |
| `/login` | Login | `pages/Login.tsx` |
| `/title/:id` | TitleDetail | `pages/TitleDetail.tsx` |
| `/person/:name` | PersonTitles | `pages/PersonTitles.tsx` |
| `/admin` | Admin | `pages/Admin.tsx` |
| `/admin/validate` | Validate | `pages/Validate.tsx` |
| `/admin/tasks` | AdminTasks | `pages/AdminTasks.tsx` |
| `/admin/notifications` | AdminNotifications | `pages/AdminNotifications.tsx` |
| `/admin/anilist` | AdminAniList | `pages/AdminAniList.tsx` |
| `/admin/help` | Help | `pages/Help.tsx` (in-app FAQ) |
| `/anilist/callback` | AnilistCallback | `pages/AnilistCallback.tsx` |
| `/match-review` | MatchReview | `pages/MatchReview.tsx` |

### localStorage persistence

Pure UI preferences (e.g. `title-sort` in `store.ts:loadSort/saveSort`) can be stored raw — no staleness risk. Any future cache of server-sourced data MUST wrap the payload with a timestamp and enforce a TTL on read, otherwise a stale value will outlive any schema/semantic change:

```ts
type Cached<T> = { ts: number; v: T }
const TTL_MS = 30 * 24 * 60 * 60 * 1000

function load<T>(key: string): T | null {
  try {
    const raw = localStorage.getItem(key)
    if (!raw) return null
    const c: Cached<T> = JSON.parse(raw)
    if (Date.now() - c.ts > TTL_MS) { localStorage.removeItem(key); return null }
    return c.v
  } catch { return null }
}
```

## Quality

### Linting
`.golangci.yml` — errcheck, gocritic, govet. Run `make lint`.

### Backend Tests
`make test` — `testify/assert+require`, in-memory SQLite. Table-driven tests for repository and handlers.

### Frontend Tests
`make test-front` — vitest + jsdom + @testing-library/preact.
