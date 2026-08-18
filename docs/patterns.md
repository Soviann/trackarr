# Patterns & Codebase Map

High-density, token-optimized file map for LLM agents. Look up file paths, symbols, routes, and invariants directly without exploratory scans.

## LLM Deep Dive Specifications
- Matching Pipeline & AI Auto-Confirm: `docs/dev/matching-pipeline.md`
- AniList GraphQL, Prequel Chaining & Multi-Parts: `docs/dev/anilist-sync.md`
- SQLite WAL, Writer/Reader Contracts & Deadlocks: `docs/dev/database-model.md`
- Radarr / Sonarr (*arr) Queue & Push Workflow: `docs/dev/arr-integration.md`

## Human / User Documentation
- Sommaire général: `docs/INDEX.md`
- Guide Interface & Vues: `docs/interface.md`
- Intégrations & Webhooks: `docs/integrations.md`
- Tâches de Fond: `docs/background-jobs.md`
- Déploiement NAS: `docs/deployment.md`
- Maintenance & Débogage de Prod: `docs/maintenance.md`

---

## Backend (Go)

### CLI Commands (`cmd/` & `Makefile`)

| Command | File | Purpose |
|---|---|---|
| `serve` | `cmd/serve.go` | HTTP server (`DISABLE_BACKGROUND_TASKS` flag available). |
| `import` | `cmd/import.go` | Simkl backup import (`--dry-run` available). |
| `backfill-accents` | `cmd/backfill_accents.go` | Extract and persist dominant cover accent colors (`--force` flag). |
| `make ssh-db-pull` | `Makefile` | Pull prod DB (`plextracker.db` + WAL/SHM) from NAS to local `data/`. |
| `make ssh-logs` | `Makefile` | Dump prod container logs to `data/plextracker.log` (`LINES=...` optional). |
| `make ssh-debug-pull`| `Makefile` | Combined target: downloads prod DB and logs for local-first debugging. |
| `make reset-import` | `Makefile` | Resets local DB and imports a Simkl backup (`BACKUP_FILE=`). |

### Schema & Models (`internal/model/`)

| Model | File | Description & Key Fields |
|---|---|---|
| `Title` | `internal/model/title.go` | `ID`, `Type`, `Status`, `SeriesStatus`, `MatchStatus`, `TMDBID`, `TVDBID`, `AniListID`, `SimklID`, `RadarrID`, `SonarrID`, `ArrIgnored`, `OriginCountry`, `TotalWatchMinutes`, `AccentHex`, `WatchProviders`. |
| `TitleName` | `internal/model/title.go` | Multi-language and alternative alias names (`TitleID`, `Language`, `Name`). |
| `Season` | `internal/model/season.go` | `ID`, `TitleID`, `SeasonNumber`, `EpisodeCount`, `WatchedCount`, `AirDate`. |
| `Episode` | `internal/model/episode.go` | `ID`, `SeasonID`, `EpisodeNumber`, `Name`, `Watched`, `LastWatchedAt`. |
| `WatchEvent`| `internal/model/watch_event.go` | Scrobble log (`ID`, `TitleID`, `EpisodeID`, `Source`, `WatchedAt`). |
| `Task` | `internal/model/task.go` | Background queue item (`ID`, `Type`, `Payload`, `Status`, `Attempts`, `RunAt`, `DedupKey`). |
| `MatchEvent`| `internal/model/match_event.go` | Match audit trail (`ID`, `TitleID`, `Kind`, `Detail`, `CreatedAt`). |
| `Setting` | `internal/model/setting.go` | Key-value string pair (`Key`, `Value`). |

### Repositories (`internal/repository/`)

| Repository | Reader Methods (`DBTX`) | Writer Methods (`*sql.Tx`) |
|---|---|---|
| `Title` | `GetByID`, `List`, `ListAll`, `FindByExternalID`, `ListOriginCountries`, search in `title_search.go` | `Create`, `Update`, `UpdateLastWatchedAt`, `ReplaceNames`, `AddMissingNames`, `Merge`, `Delete`, `BatchDelete`, `BatchStatus` |
| `Season` | `GetByID`, `ListByTitleID` | `GetOrCreate`, `UpdateRating`, `UpdateTotalEpisodes`, `Upsert` |
| `Episode` | `GetBySeasonID`, `GetByID` | `GetOrCreate`, `ToggleWatched`, `BatchMarkWatched`, `UpdateMetadata`, `UpsertBatch`, `MarkWatched` |
| `WatchEvent` | `CountByTitleID`, `ListByTitle` | `Create`, `BatchCreate` |
| `Task` | `GetByID`, `ListPending`, `ListDead`, `ListPaginated`, `CountByStatus` | `Enqueue`, `EnqueueWithDelay`, `FetchDue`, `Complete`, `Fail`, `RetryDead`, `ResetRunning`, `Delete`, `DeleteBatch` |
| `SeasonExternalID` | `Get`, `ListParts`, `ListPartsForTitle` | `Add`, `Delete`, `DeletePart`, `Reorder`, `UpdatePartMeta` (Multi-part season support) |
| `Genre` | `ListWithCounts` | `ReplaceForTitle` |
| `Setting` | `Get` | `Set`, `Delete` |
| `MatchEvent` | `ListRecent` (includes cover URL join) | `Create` |
| `SeasonAudit` | `ListDismissals` | `Dismiss` |
| `Stats` | `TotalWatchMinutes`, `TopGenres`, `CurrentStreak`, `BestStreak` | N/A (read-only) |
| `Activity` | `List` (paginated scrobble events) | N/A (read-only) |
| `History` | `GetByTitleID` (title watch log) | N/A (read-only) |

### Services (`internal/service/`)

| Service | File | Purpose | Deep Spec |
|---|---|---|---|
| `TitleService` | `internal/service/title.go` | Title CRUD, matching orchestration, rematching, merges | `docs/dev/matching-pipeline.md` |
| `LibraryService` | `internal/service/library.go` | User scrobbles, auto-completion, rating prompts, notifications | `docs/dev/database-model.md` |
| `JellyfinService` | `internal/service/jellyfin.go` | Jellyfin webhook ingestion (`PlaybackStop` + `PlayedToCompletion`) | `docs/integrations.md` |
| `ArrService` | `internal/service/arr.go` | Radarr/Sonarr proxy and push enqueuing | `docs/dev/arr-integration.md` |
| `AniListPushService` | `internal/service/anilist_push.go` | AniList GraphQL state push (per season part / movie) | `docs/dev/anilist-sync.md` |
| `BackfillService` | `internal/service/backfill.go` | Episode metadata backfilling (opens isolated writeDB tx) | `docs/dev/database-model.md` |
| `CoverService` | `internal/service/cover.go` | Cover downloading and accent color extraction (`colorextract/`) | `docs/patterns.md` |
| `APILimiter` | `internal/service/ratelimiter.go` | Global 2 rps token bucket for external APIs | `docs/patterns.md` |
| `BackgroundService` | `internal/service/background.go` | Daily metadata refresh crons and name sync | `docs/background-jobs.md` |
| `SeasonAuditService` | `internal/service/seasonaudit.go` | Split season detection and suggested merge engine | `docs/dev/anilist-sync.md` |
| `SimklImporter` | `internal/service/simkl.go` | Simkl backup archive parser and database populator | `docs/deployment.md` |
| `TaskQueueWorker` | `internal/service/taskqueue.go` | Asynchronous task execution engine (`enrichment`, `push`, `arr`) | `docs/background-jobs.md` |

### API Routes & Handlers (`internal/router/router.go`)

| Method | Path | Handler | Description |
|---|---|---|---|
| GET | `/api/health` | `handler.Health` | Health check endpoint |
| GET | `/api/config` | `handler.PublicConfig` | Pre-auth public configuration |
| POST | `/api/auth/google` | `handler.GoogleCallback` | OAuth Google authentication |
| POST | `/api/auth/dev` | `handler.DevLogin` | Local dev password login (debug only) |
| POST | `/api/auth/logout` | `handler.Logout` | Clear JWT auth cookie |
| POST | `/api/webhook/jellyfin/{secret}` | `handler.HandleJellyfin` | Ingest scrobbles from Jellyfin |
| GET | `/api/covers/{filename}` | `handler.ServeCover` | Serve cached cover image |
| GET | `/api/titles` | `titles.List` | Paginated library list with filters |
| POST | `/api/titles` | `titles.Create` | Manually create a new title |
| GET | `/api/titles/{id}` | `titles.GetByID` | Detailed title payload |
| PATCH | `/api/titles/{id}` | `titles.Update` | Update status, rating, or metadata |
| DELETE | `/api/titles/{id}` | `titles.Delete` | Delete title and cascade associations |
| POST | `/api/titles/{id}/rematch` | `titles.Rematch` | Reset IDs and trigger re-enrichment |
| PUT | `/api/titles/{id}/external-ids` | `titles.SetExternalIDs` | Explicitly overwrite external IDs |
| POST | `/api/titles/{id}/merge` | `titles.Merge` | Merge source title into target |
| POST | `/api/titles/{id}/refresh` | `titles.RefreshOne` | Force immediate metadata refresh |
| GET | `/api/titles/continue-watching` | `library.ContinueWatching` | Continue watching grid list |
| GET | `/api/titles/upcoming` | `library.Upcoming` | Upcoming titles grid list |
| GET | `/api/titles/review-count` | `titles.ReviewCount` | Badge count for review/unconfirmed |
| POST | `/api/titles/batch-delete` | `titles.BatchDelete` | Delete multiple titles |
| POST | `/api/titles/batch-status` | `titles.BatchStatus` | Bulk update title statuses |
| PATCH | `/api/titles/{titleID}/episodes/{episodeID}` | `episodes.ToggleWatched` | Mark episode watched / unwatched |
| POST | `/api/titles/{titleID}/episodes/batch-watch` | `episodes.BatchMarkWatched` | Bulk mark episodes watched |
| POST | `/api/titles/{titleID}/seasons/{seasonID}/anilist` | `seasonExternal.AddAniListID` | Attach AniList part to season |
| DELETE| `/api/titles/{titleID}/seasons/{seasonID}/anilist/{extID}` | `seasonExternal.RemoveAniListID` | Detach AniList part |
| PUT | `/api/titles/{titleID}/seasons/{seasonID}/anilist/order` | `seasonExternal.ReorderAniList` | Reorder AniList parts |
| GET | `/api/stats` | `stats.Get` | Library metrics & insight cards |
| GET | `/api/stats/activity` | `activity.List` | Paginated watch history feed |
| GET | `/api/match-events` | `matchEvents.List` | Audit trail of auto-matches |
| GET | `/api/genres` | `genres.List` | List genres with counts |
| GET | `/api/countries` | `titles.Countries` | List origin countries with counts |
| GET | `/api/admin/arr` | `admin.GetArrSettings` | Radarr & Sonarr configuration |
| PUT | `/api/admin/arr` | `admin.UpdateArrSettings` | Update Radarr & Sonarr configuration |
| GET | `/api/arr/queue` | `arr.ListArrQueue` | Active Radarr/Sonarr download queue |
| POST | `/api/arr/queue/{id}/push` | `arr.PushToArr` | Send title to Radarr / Sonarr |
| GET | `/api/admin/tasks` | `admin.ListTasks` | Task queue inspection |
| POST | `/api/admin/tasks/{id}/retry` | `admin.RetryTask` | Retry failed task |
| GET | `/api/admin/season-audit` | `seasonAudit.List` | Suggested multi-season merges |
| POST | `/api/admin/season-audit/accept` | `seasonAudit.Accept` | Execute suggested merge |
| POST | `/api/admin/season-audit/dismiss` | `seasonAudit.Dismiss` | Dismiss suggested merge |
| POST | `/api/admin/refresh-all` | `admin.RefreshAll` | Trigger full library refresh |

---

## Frontend (Preact)

### Routing Conventions (`frontend/src/routes.ts`)
- **Single Source of Truth**: All route patterns live in `ROUTE_PATHS`, route builders in `routeTo`.
- **API URL Rule**: `apiFetch` and `useApi` automatically prepend `/api`. Never write `apiFetch('/api/...')`.
- **Naming Rule**: SPA routes are singular (`/title/:id`), API routes are plural (`/api/titles/:id`).

### State Management (`frontend/src/store.ts`)
- `useTitleStore`: Zustand store for title listing, pagination, sorting (`localStorage`), and filters (`origin_country`, `my_rating_min`, `tmdb_rating_min`, `series_status`).
- `useSearchStore`: Search query state, debounce, and TMDB toggle.

### Components Map (`frontend/src/components/`)

| Component | File | Purpose |
|---|---|---|
| `Navbar` | `components/Navbar.tsx` | 4-tab bottom navigation bar |
| `TitleCard` | `components/TitleCard.tsx` | Horizontal list card with progress bar and quick mark action |
| `PosterCard` | `components/PosterCard.tsx` | 2:3 vertical grid poster card with type badge |
| `PosterTile` | `components/PosterTile.tsx` | Compact poster card for preset strips and grids |
| `TypeBadge` | `components/TypeBadge.tsx` | Movie/Series badge with optional colored Arr top accent border |
| `ArrBadge` | `components/ArrBadge.tsx` | State pill indicating Radarr/Sonarr status (*In Queue*, *Downloaded*, *Monitored*) |
| `FilterDrawer`| `components/FilterDrawer.tsx` | Collapsible filter panel (status, type, series status, genres, country, rating) |
| `SearchBar` | `components/SearchBar.tsx` | Sticky search input bound to `useSearchStore` |
| `SeasonAniListStrip` | `components/SeasonAniListStrip.tsx` | Active season AniList score and multi-part management strip |
| `RematchSheet`| `components/RematchSheet.tsx` | TMDB search & manual ID fixer for titles or season AniList parts |
| `MatchReviewCard` | `components/MatchReviewCard.tsx` | Review card with external ID chips, confirm, and fix actions |
| `RatingPrompt`| `components/RatingPrompt.tsx` | 10-star rating popup with AniList / IMDb shortcuts |
| `EditSheet` | `components/EditSheet.tsx` | Quick edit for status, type, and display title |
| `BottomSheet` | `components/BottomSheet.tsx` | Slide-up modal sheet with drag gestures and backdrop |
| `PullToRefresh`| `components/PullToRefresh.tsx` | Touch-based pull-to-refresh wrapper |
| `SwipeActions`| `components/SwipeActions.tsx` | Swipeable item revealing action buttons |

### Pages Map (`frontend/src/pages/`)

| Route | Page Component | File |
|---|---|---|
| `/` | `Library` | `pages/Library.tsx` |
| `/continue-watching` | `ContinueWatching` | `pages/ContinueWatching.tsx` |
| `/coming-up` | `ComingUp` | `pages/ComingUp.tsx` |
| `/search` | `Search` | `pages/Search.tsx` |
| `/add` | `Add` | `pages/Add.tsx` |
| `/stats` | `Stats` | `pages/Stats.tsx` |
| `/title/:id` | `TitleDetail` | `pages/TitleDetail.tsx` |
| `/person/:name` | `PersonTitles` | `pages/PersonTitles.tsx` |
| `/match-review` | `MatchReview` | `pages/MatchReview.tsx` |
| `/admin` | `Admin` | `pages/Admin.tsx` |
| `/admin/arr` | `AdminArr` | `pages/AdminArr.tsx` |
| `/admin/arr/queue` | `ArrQueue` | `pages/ArrQueue.tsx` |
| `/admin/tasks` | `AdminTasks` | `pages/AdminTasks.tsx` |
| `/admin/notifications` | `AdminNotifications` | `pages/AdminNotifications.tsx` |
| `/admin/anilist` | `AdminAniList` | `pages/AdminAniList.tsx` |
| `/admin/season-audit` | `AdminSeasonAudit` | `pages/AdminSeasonAudit.tsx` |
| `/admin/validate` | `Validate` | `pages/Validate.tsx` |
| `/admin/help` | `Help` | `pages/Help.tsx` |

---

## Core Invariants & Engineering Rules

1. **Production Debugging (Local-First)**: Never run exploratory commands or grep logs live on production via SSH. Pull state locally with `make ssh-debug-pull` and debug against `data/plextracker.db` and `data/plextracker.log`.
2. **SQLite Single Writer**: SQLite connection pool is locked to `MaxOpenConns=1`. Never open nested transactions.
3. **Frontend Re-Embed**: `dist/` is embedded in the Go binary. After editing frontend and running `make test-front`, run `touch main.go` so `air` re-embeds the assets.
4. **PWA Cache Busting**: The service worker caches aggressively. When validating UI changes in the browser, append `?t=$(date +%s)` to the test URL.
5. **Antigravity NAS Daemon Boundary**: The webhook daemon (`scripts/github-pr-daemon/server.py`) running on the NAS is a lightweight assistant that reads local logs and calls Gemini to draft plans. It does not execute `make` or Docker builds. All testing and code implementations are executed locally on the developer workstation or in GitHub Actions.
