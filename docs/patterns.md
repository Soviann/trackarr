# Patterns & Codebase Map

[← Back to Index](INDEX.md)

---

## LLM Deep Dive Specifications
- Matching Pipeline & AI Auto-Confirm: `docs/dev/matching-pipeline.md`
- AniList GraphQL, Prequel Chaining & Multi-Parts: `docs/dev/anilist-sync.md`
- SQLite WAL, Writer/Reader Contracts & Deadlocks: `docs/dev/database-model.md`
- Radarr / Sonarr (*arr) Queue & Push Workflow: `docs/dev/arr-integration.md`
- Background Jobs & Task Queue Architecture: `docs/background-jobs.md`

## Human / User Documentation
- Master Index: `docs/INDEX.md`
- Overview & Project Scope: `docs/overview.md`
- User Guide & Troubleshooting: `docs/user-guide.md`
- API Setup & Credentials: `docs/api-setup.md`
- Integrations & Webhooks: `docs/integrations.md`
- Deployment & Updates (3 Options): `docs/deployment.md`
- Maintenance & Operations: `docs/maintenance.md`
- LLM Context Reference: `docs/llm.md`

---

## Backend (Go)

### CLI Commands (`cmd/` & `Makefile`)

| Command | File | Purpose |
|---|---|---|
| `serve` | `cmd/serve.go` | HTTP server (`DISABLE_BACKGROUND_TASKS` flag available). |
| `version` | `cmd/version.go` | Show application version, commit SHA, and build date. |
| `import` | `cmd/import.go` | Simkl backup import (`--dry-run` available). |
| `backfill-accents` | `cmd/backfill_accents.go` | Extract and persist dominant cover accent colors (`--force` flag). |
| `reset-password` | `cmd/reset_password.go` | Reset admin password and output a fresh emergency recovery key (`--password`, `--username`). |
| `make version` | `Makefile` | Print Trackarr version (`Trackarr vX.Y.Z`). |
| `make reset-password` | `Makefile` | CLI wrapper to reset admin password locally (`PASSWORD=...`, `USERNAME=...`). |
| `make ssh-db-pull` | `Makefile.local` | Pull prod DB (`trackarr.db` + WAL/SHM) from NAS to local `data/`. |
| `make ssh-logs` | `Makefile.local` | Dump prod container logs to `data/trackarr.log` (`LINES=...` optional). |
| `make ssh-debug-pull`| `Makefile.local` | Combined target: downloads prod DB and logs for local-first debugging. |
| `make reset-import` | `Makefile` | Resets local DB and imports a Simkl backup (`BACKUP_FILE=`). |

### Schema & Models (`internal/model/`)

| Model | File | Description & Key Fields |
|---|---|---|
| `Title` | `internal/model/title.go` | `ID`, `Type`, `Status`, `SeriesStatus`, `MatchStatus`, `TMDBID`, `TVDBID`, `AniListID`, `SimklID`, `RadarrID`, `SonarrID`, `ArrIgnored`, `OriginCountry`, `TotalWatchMinutes`, `AccentHex`, `WatchProviders`, `PersonalNotes`. |
| `TitleName` | `internal/model/title.go` | Multi-language and alternative alias names (`TitleID`, `Language`, `Name`). |
| `Season` | `internal/model/season.go` | `ID`, `TitleID`, `SeasonNumber`, `EpisodeCount`, `WatchedCount`, `AirDate`. |
| `Episode` | `internal/model/episode.go` | `ID`, `SeasonID`, `EpisodeNumber`, `Name`, `Watched`, `LastWatchedAt`. |
| `WatchEvent`| `internal/model/watch_event.go` | Scrobble log (`ID`, `TitleID`, `EpisodeID`, `Source`, `WatchedAt`). |
| `Task` | `internal/model/task.go` | Background queue item (`ID`, `Type`, `Payload`, `Status`, `Attempts`, `RunAt`, `DedupKey`). |
| `MatchEvent`| `internal/model/match_event.go` | Match audit trail (`ID`, `TitleID`, `Kind`, `Detail`, `CreatedAt`). |
| `Setting` | `internal/model/setting.go` | Key-value string pair (`Key`, `Value`). |
| `TitleRelation` | `internal/model/title_relation.go` | Side stories, movies, sagas, and franchise relations (`TitleID`, `SeasonID`, `Provider`, `ExternalID`, `RelationType`, `Format`, `MatchedTitleID`). |
| `WrappedResponse` / `Stats` | `internal/model/stats.go` | Comprehensive statistics, actor/director rankings, and annual Wrapped payload (`WrappedResponse`, `WrappedAIPersona`, `WrappedArchiveItem`). |

### Repositories (`internal/repository/`)

| Repository | Reader Methods (`DBTX`) | Writer Methods (`*sql.Tx`) |
|---|---|---|
| `Title` | `GetByID`, `List`, `ListAll`, `FindByExternalID`, `ListOriginCountries`, `HasWatchedEpisodes`, `HasUnwatchedEpisodes`, search in `title_search.go` | `Create`, `Update`, `UpdateLastWatchedAt`, `ReplaceNames`, `AddMissingNames`, `Merge`, `Delete`, `BatchDelete`, `BatchStatus` |
| `TitleRelation` | `GetByTitleID`, `GetBySeasonID`, `DeleteForTitle` | `UpsertBatch`, `DeleteForTitle` |
| `Season` | `GetByID`, `ListByTitleID` | `GetOrCreate`, `UpdateRating`, `UpdateTotalEpisodes`, `Upsert` |
| `Episode` | `GetBySeasonID`, `GetByID` | `GetOrCreate`, `ToggleWatched`, `BatchMarkWatched`, `UpdateMetadata`, `UpsertBatch`, `MarkWatched` |
| `WatchEvent` | `CountByTitleID`, `ListByTitle` | `Create`, `BatchCreate` |
| `Task` | `GetByID`, `ListPending`, `ListDead`, `ListPaginated`, `CountByStatus` | `Enqueue`, `EnqueueWithDelay`, `FetchDue`, `Complete`, `Fail`, `RetryDead`, `ResetRunning`, `Delete`, `DeleteBatch` |
| `SeasonExternalID` | `Get`, `ListParts`, `ListPartsForTitle` | `Add`, `Delete`, `DeletePart`, `Reorder`, `UpdatePartMeta` (Multi-part season support) |
| `Genre` | `ListWithCounts` | `ReplaceForTitle` |
| `Setting` | `Get` | `Set`, `Delete` |
| `MatchEvent` | `ListRecent` (includes cover URL join) | `Create` |
| `SeasonAudit` | `ListDismissals` | `Dismiss` |
| `Stats` | `TotalWatchMinutes`, `TopGenres`, `TopActors`, `TopDirectors`, `CurrentStreak`, `BestStreak`, `GetWrappedData` | N/A (read-only) |
| `Wrapped` | `GetSnapshot`, `HasSnapshot`, `ListArchives` | `SaveSnapshot`, `DeleteSnapshot` |
| `Activity` | `List` (paginated scrobble events) | N/A (read-only) |
| `History` | `GetByTitleID` (title watch log) | N/A (read-only) |

### Services (`internal/service/`)

| Service | File | Purpose | Deep Spec |
|---|---|---|---|
| `TitleService` | `internal/service/title.go` | Title CRUD, matching orchestration, rematching, merges | `docs/dev/matching-pipeline.md` |
| `LibraryService` | `internal/service/library.go` | User scrobbles, auto-completion, rating prompts, notifications | `docs/dev/database-model.md` |
| `JellyfinService` | `internal/service/jellyfin.go` | Jellyfin webhook ingestion (`PlaybackStop` + `PlayedToCompletion`) | `docs/integrations.md` |
| `PlexService` | `internal/service/plex.go` | Plex webhook ingestion (`media.scrobble`, multipart + JSON fallback) | `docs/integrations.md` |
| `ArrService` | `internal/service/arr.go` | Radarr/Sonarr proxy and push enqueuing | `docs/dev/arr-integration.md` |
| `AniListPushService` | `internal/service/anilist_push.go` | AniList GraphQL state push (per season part / movie) | `docs/dev/anilist-sync.md` |
| `BackfillService` | `internal/service/backfill.go` | Episode metadata backfilling (opens isolated writeDB tx) | `docs/dev/database-model.md` |
| `CoverService` | `internal/service/cover.go` | Cover downloading and accent color extraction (`colorextract/`) | `docs/patterns.md` |
| `ProwlarrService` | `internal/service/prowlarr.go` | Prowlarr indexer search (releases), memory caching, and poster resolution | `docs/patterns.md` |
| `APILimiter` | `internal/service/ratelimiter.go` | Global 2 rps token bucket for external APIs | `docs/patterns.md` |
| `CalendarService` | `internal/service/calendar.go` | RFC 5545 iCalendar generation, token rotation & range queries | `docs/user-guide.md` |
| `BackgroundService` | `internal/service/background.go` | Daily metadata refresh crons and name sync | `docs/background-jobs.md` |
| `SeasonAuditService` | `internal/service/seasonaudit.go` | Split season detection and suggested merge engine | `docs/dev/anilist-sync.md` |
| `SimklImporter` | `internal/service/simkl.go` | Simkl backup archive parser and database populator | `docs/deployment.md` |
| `TaskQueueWorker` | `internal/service/taskqueue.go` | Asynchronous task execution engine (`enrichment`, `push`, `arr`) | `docs/background-jobs.md` |

### API Routes & Handlers (`internal/router/router.go`)

| Method | Path | Handler | Description |
|---|---|---|---|
| GET | `/api/health` | `handler.Health` | Health check endpoint |
| GET | `/api/config` | `auth.PublicConfig` | Pre-auth public configuration (Google Client ID, VAPID, Auth Mode, Setup status, Metadata Language, Enabled Watch Providers) |
| POST | `/api/auth/setup` | `auth.Setup` | Initial admin account setup & emergency recovery key generation |
| POST | `/api/auth/login` | `auth.Login` | Local password login (Bcrypt, constant-time, rate-limited) |
| POST | `/api/auth/google` | `auth.GoogleCallback` | OAuth Google authentication |
| POST | `/api/auth/recover` | `auth.Recover` | Emergency recovery with key (`TRCK-...`) & auto-regeneration |
| POST | `/api/auth/change-password` | `auth.ChangePassword` | Authenticated password change & auto-regeneration |
| POST | `/api/auth/recovery-key/regenerate` | `auth.RegenerateRecoveryKey` | Regenerate emergency recovery key |
| GET | `/api/admin/system-settings` | `adminSettings.GetSystemSettings` | Read current system configuration and webhook URLs |
| PUT | `/api/admin/system-settings` | `adminSettings.UpdateSystemSettings` | Update configuration in SQLite & trigger client hot-reload |
| POST | `/api/admin/system-settings/test/{service}` | `adminSettings.Test*` | Test connection for TMDB, TVDB, Gemini, Radarr, Sonarr, Prowlarr |
| POST | `/api/admin/system-settings/vapid/generate` | `adminSettings.GenerateVAPIDKeys` | Auto-generate fresh NIST P-256 VAPID keypair |
| GET | `/api/admin/auth-settings` | `auth.GetAuthSettings` | Get auth mode and configuration state |
| PUT | `/api/admin/auth-settings` | `auth.UpdateAuthSettings` | Update auth mode (`google`, `password`, `hybrid`) |
| POST | `/api/auth/logout` | `auth.Logout` | Clear JWT auth cookie |
| POST | `/api/webhook/jellyfin/{secret}` | `handler.HandleJellyfin` | Ingest scrobbles from Jellyfin |
| POST | `/api/webhook/plex/{secret}` | `handler.HandlePlex` | Ingest scrobbles from Plex |
| GET | `/api/calendar.ics` | `calendarHandler.ServeICS` | Public token-secured RFC 5545 iCalendar subscription feed (`?token=...`) |
| GET | `/api/calendar/events` | `calendarHandler.GetEvents` | In-app calendar events for interactive multi-view calendar |
| GET | `/api/calendar/token` | `calendarHandler.GetToken` | Retrieve active iCal subscription token & webcal URLs |
| POST | `/api/calendar/token/regenerate` | `calendarHandler.RegenerateToken` | Rotate iCal secret token |
| GET | `/api/covers/{filename}` | `covers.Serve` | Serve cached cover image |
| GET | `/covers/{filename}` | `covers.Serve` | Legacy top-level cover route without `/api` prefix |
| GET | `/api/titles` | `titles.List` | Paginated library list with filters and search |
| POST | `/api/titles` | `titles.Create` | Manually create a new title |
| GET | `/api/titles/review-count` | `titles.ReviewCount` | Badge count for review/unconfirmed titles |
| GET | `/api/titles/resolve` | `titles.Resolve` | Preview external metadata before creating a title |
| GET | `/api/titles/continue-watching` | `library.ContinueWatching` | Continue watching grid list with progress, providers & next episode |
| GET | `/api/titles/upcoming` | `library.Upcoming` | Upcoming titles grid list |
| POST | `/api/titles/batch-delete` | `titles.BatchDelete` | Delete multiple titles |
| POST | `/api/titles/batch-status` | `titles.BatchStatus` | Bulk update title statuses |
| GET | `/api/titles/{id}` | `titles.GetByID` | Detailed title payload |
| PATCH | `/api/titles/{id}` | `titles.Update` | Update status, rating, or metadata |
| DELETE | `/api/titles/{id}` | `titles.Delete` | Delete title and cascade associations |
| POST | `/api/titles/{id}/rematch` | `titles.Rematch` | Reset IDs and trigger re-enrichment |
| PUT | `/api/titles/{id}/external-ids` | `titles.SetExternalIDs` | Explicitly overwrite external IDs |
| POST | `/api/titles/{id}/merge` | `titles.Merge` | Merge source title into target |
| POST | `/api/titles/{id}/refresh` | `titles.RefreshOne` | Force immediate metadata refresh |
| GET | `/api/titles/{id}/history` | `history.Get` | Detailed watch event history for title |
| GET | `/api/tmdb/search` | `tmdbSearch.Search` | Search TMDB for movie or TV titles |
| GET | `/api/anilist/search` | `anilistSearch.Search` | Search AniList for anime titles |
| GET | `/api/releases` | `releasesHandler.List` | Latest Prowlarr releases with posters & local match |
| POST | `/api/releases/add` | `releasesHandler.Add` | Direct 1-click title creation from release |
| PATCH | `/api/titles/{titleID}/episodes/{episodeID}` | `episodes.ToggleWatched` | Mark episode watched / unwatched |
| POST | `/api/titles/{titleID}/episodes/batch-watch` | `episodes.BatchMarkWatched` | Bulk mark episodes watched |
| POST | `/api/titles/{titleID}/seasons/{seasonID}/anilist` | `seasonExternal.AddAniListID` | Attach AniList part to season |
| DELETE| `/api/titles/{titleID}/seasons/{seasonID}/anilist/{externalID}` | `seasonExternal.RemoveAniListID` | Detach AniList part |
| PUT | `/api/titles/{titleID}/seasons/{seasonID}/anilist/order` | `seasonExternal.ReorderAniList` | Reorder AniList parts |
| POST | `/api/push/subscribe` | `push.Subscribe` | Register Web Push subscription |
| DELETE | `/api/push/subscribe` | `push.Unsubscribe` | Remove Web Push subscription |
| GET | `/api/stats` | `stats.Get` | Library metrics & insight cards |
| GET | `/api/stats/wrapped` | `stats.GetWrapped` | Annual retrospective metrics, top favorites/releases & Gemini AI persona |
| GET | `/api/stats/wrapped/archives` | `stats.GetWrappedArchives` | List all archived Wrapped snapshots for gallery display |
| POST | `/api/stats/wrapped/generate` | `stats.RegenerateWrapped` | Force generation & SQLite persistence of Wrapped snapshot |
| GET | `/api/stats/activity` | `activity.List` | Paginated watch history feed |
| GET | `/api/match-events` | `matchEvents.List` | Audit trail of auto-matches |
| GET | `/api/genres` | `genres.List` | List genres with counts |
| GET | `/api/countries` | `titles.Countries` | List origin countries with counts |
| GET | `/api/settings` | `settings.Get` | User and app settings |
| GET | `/api/anilist/auth` | `anilistAuth.Authorize` | Generate AniList OAuth authorize URL |
| POST | `/api/anilist/token` | `anilistAuth.SaveToken` | Save AniList OAuth access token |
| DELETE | `/api/anilist/token` | `anilistAuth.Disconnect` | Disconnect AniList integration |
| GET | `/api/admin/counts` | `admin.Counts` | Library and queue totals for admin navbar badge |
| GET | `/api/admin/tasks` | `admin.ListTasks` | Task queue inspection |
| POST | `/api/admin/tasks/{id}/retry` | `admin.RetryTask` | Retry failed task |
| DELETE | `/api/admin/tasks/{id}` | `admin.DeleteTask` | Delete individual task |
| POST | `/api/admin/tasks/batch-delete` | `admin.DeleteTasksBatch` | Bulk delete tasks |
| GET | `/api/admin/notifications` | `admin.GetNotificationPrefs` | Web Push notification preferences |
| PUT | `/api/admin/notifications` | `admin.UpdateNotificationPrefs` | Update Web Push notification preferences |
| GET | `/api/admin/arr` | `admin.GetArrSettings` | Radarr & Sonarr configuration |
| PUT | `/api/admin/arr` | `admin.UpdateArrSettings` | Update Radarr & Sonarr configuration |
| POST | `/api/admin/refresh-all` | `admin.RefreshAll` | Trigger full library refresh |
| GET | `/api/admin/season-audit` | `seasonAudit.List` | Suggested multi-season merges |
| POST | `/api/admin/season-audit/accept` | `seasonAudit.Accept` | Execute suggested merge |
| POST | `/api/admin/season-audit/dismiss` | `seasonAudit.Dismiss` | Dismiss suggested merge |
| GET | `/api/arr/{app}/rootfolder` | `arr.ProxyRootFolder` | Proxy root folder options from Radarr/Sonarr |
| GET | `/api/arr/{app}/qualityprofile` | `arr.ProxyQualityProfile` | Proxy quality profile options from Radarr/Sonarr |
| GET | `/api/arr/title/{id}` | `arr.GetTitleArr` | Get title Arr configuration and status |
| PUT | `/api/arr/title/{id}` | `arr.UpdateTitleArr` | Update title Arr configuration and status |
| POST | `/api/arr/push/{id}` | `arr.PushToArr` | Send title directly to Radarr / Sonarr |
| POST | `/api/arr/queue/{id}/push` | `arr.PushToArr` | Push title from queue directly to Radarr / Sonarr |
| POST | `/api/client-errors` | `clientErrors.Handle` | Client-side error reporting |

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
| `ActionDrawer` | `components/ActionDrawer.tsx` | Slide-up drawer exposing management actions for titles and seasons |
| `SectionCards` | `components/SectionCards.tsx` | 3-column hub cards header on Library page with poster slices backdrop |
| `SectionRow` | `components/SectionRow.tsx` | Section row header container with count pill and action buttons |
| `TitleCard` | `components/TitleCard.tsx` | Horizontal list card with progress bar and quick mark action |
| `PosterCard` | `components/PosterCard.tsx` | 2:3 vertical grid poster card with type badge |
| `PosterTile` | `components/PosterTile.tsx` | Compact poster card for preset strips and grids |
| `PosterStrip` | `components/PosterStrip.tsx` | Horizontal scrolling strip of poster thumbnails |
| `CoverImage` | `components/CoverImage.tsx` | Resilient image loader with fallback handling and caching |
| `CoverPlaceholder` | `components/CoverPlaceholder.tsx` | Geometric stylized placeholder when cover art is unavailable |
| `TypeBadge` | `components/TypeBadge.tsx` | Movie/Series badge with optional colored Arr top accent border |
| `StatusBadge` | `components/StatusBadge.tsx` | Pill badge indicating watch and release statuses (*Watching*, *Completed*, *Plan to Watch*, *Caught Up*) |
| `ArrBadge` | `components/ArrBadge.tsx` | State pill indicating Radarr/Sonarr status (*In Queue*, *Downloaded*, *Monitored*) |
| `FilterDrawer`| `components/FilterDrawer.tsx` | Collapsible segmented filter panel with 3 tabs: Status & Type, Genres & Origin, Dates & Ratings |
| `SearchBar` | `components/SearchBar.tsx` | Sticky search input bound to `useSearchStore` |
| `SeasonAniListStrip` | `components/SeasonAniListStrip.tsx` | Active season AniList score and multi-part management strip |
| `SeasonSideStories` | `components/SeasonSideStories.tsx` | Inline cards for side stories and movies recommended at the end of the active season |
| `SeasonTab` | `components/SeasonTab.tsx` | Interactive tab button for switching season views in TitleDetail |
| `EpisodeRow` | `components/EpisodeRow.tsx` | Episode listing row with title, air date, and interactive toggle checkmark |
| `FranchiseRelationsSection` | `components/FranchiseRelationsSection.tsx` | Sagas & Franchise tracking card with progress bar, next chronological title chip, horizontal titles strip with hidden scrollbar, category filters, timeline/release sort toggle, and collapse/expand |
| `RematchSheet`| `components/RematchSheet.tsx` | TMDB search & manual ID fixer for titles or season AniList parts |
| `AniListSheet`| `components/AniListSheet.tsx` | Slide-up modal sheet for editing AniList multi-part season associations |
| `MatchReviewCard` | `components/MatchReviewCard.tsx` | Review card with external ID chips, confirm, and fix actions |
| `RatingPrompt`| `components/RatingPrompt.tsx` | 10-star rating popup with AniList / IMDb shortcuts |
| `EditSheet` | `components/EditSheet.tsx` | Quick edit for status, type, and display title |
| `ArrPushSheet` | `components/ArrPushSheet.tsx` | Slide-up modal sheet to configure options and push title directly to Radarr/Sonarr |
| `ReleaseDetailSheet` | `components/ReleaseDetailSheet.tsx` | Slide-up modal sheet displaying torrent metadata, scene release name, external links and 1-click addition button |
| `NextEpisodeHero` | `components/NextEpisodeHero.tsx` | Prominent call-to-action hero card with 1-click next episode mark & binge duration estimator |
| `PersonalNotesCard` | `components/PersonalNotesCard.tsx` | Private memo/notes card on title detail with debounced auto-save |
| `WatchProviderBadges` | `components/WatchProviderBadges.tsx` | Streaming provider badges (Netflix, Prime Video, Disney+, Apple TV+, Max, Canal+, Crunchyroll, Paramount+, ADN) |
| `CalendarMonthGrid` | `components/CalendarMonthGrid.tsx` | 7-column monthly interactive grid with today indicator & selected day release cards |
| `CalendarWeekTimeline` | `components/CalendarWeekTimeline.tsx` | 7-day weekly timeline view with rich release cards |
| `CalendarIcalModal` | `components/CalendarIcalModal.tsx` | iCal subscription modal with 1-click URL copy, Apple/Google links, and token rotation |
| `PersonFilmographyDrawer` | `components/PersonFilmographyDrawer.tsx` | Slide-up modal sheet listing filmography and library titles for a given actor or director |
| `TitleHistory` | `components/TitleHistory.tsx` | Chronological scrobble session logs on title detail |
| `ConfirmationDrawer` | `components/ConfirmationDrawer.tsx` | Slide-up confirmation modal with affirmative/cancel actions |
| `CollapsibleSection` | `components/CollapsibleSection.tsx` | Foldable accordion container with toggle indicator |
| `BottomSheet` | `components/BottomSheet.tsx` | Slide-up modal sheet with drag gestures and backdrop |
| `PullToRefresh`| `components/PullToRefresh.tsx` | Touch-based pull-to-refresh wrapper |
| `SwipeActions`| `components/SwipeActions.tsx` | Swipeable item revealing action buttons |
| `ErrorBanner` | `components/ErrorBanner.tsx` | Dismissible alert banner for API and network errors |
| `ErrorBoundary` | `components/ErrorBoundary.tsx` | React error boundary with error recovery fallback |

### Pages Map (`frontend/src/pages/`)

| Route | Page Component | File |
|---|---|---|
| `/` | `Library` | `pages/Library.tsx` |
| `/releases` | `Releases` | `pages/Releases.tsx` |
| `/continue-watching` | `ContinueWatching` | `pages/ContinueWatching.tsx` |
| `/coming-up` | `ComingUp` | `pages/ComingUp.tsx` |
| `/search` | `Search` | `pages/Search.tsx` |
| `/add` | `Add` | `pages/Add.tsx` |
| `/stats` | `Stats` | `pages/Stats.tsx` |
| `/title/:id` | `TitleDetail` | `pages/TitleDetail.tsx` |
| `/person/:name` | `PersonTitles` | `pages/PersonTitles.tsx` |
| `/wrapped` | `Wrapped` | `pages/Wrapped.tsx` |
| `/login` | `Login` | `pages/Login.tsx` |
| `/setup` | `Setup` | `pages/Setup.tsx` |
| `/match-review` | `MatchReview` | `pages/MatchReview.tsx` |
| `/admin` | `Admin` | `pages/Admin.tsx` |
| `/admin/settings` | `AdminSettings` | `pages/AdminSettings.tsx` |
| `/admin/auth` | `AdminAuth` | `pages/AdminAuth.tsx` |
| `/admin/arr` | `AdminArr` | `pages/AdminArr.tsx` |
| `/admin/tasks` | `AdminTasks` | `pages/AdminTasks.tsx` |
| `/admin/notifications` | `AdminNotifications` | `pages/AdminNotifications.tsx` |
| `/admin/jellyfin` | `AdminJellyfin` | `pages/AdminJellyfin.tsx` |
| `/admin/anilist` | `AdminAniList` | `pages/AdminAniList.tsx` |
| `/admin/season-audit` | `AdminSeasonAudit` | `pages/AdminSeasonAudit.tsx` |
| `/admin/validate` | `Validate` | `pages/Validate.tsx` |
| `/admin/help` | `Help` | `pages/Help.tsx` |
| `/anilist/callback` | `AnilistCallback` | `pages/AnilistCallback.tsx` |

---

## Core Invariants & Engineering Rules

1. **Production Debugging (Local-First)**: Never run exploratory commands or grep logs live on production via SSH. Pull state locally with `make ssh-debug-pull` and debug against `data/trackarr.db` and `data/trackarr.log`.
2. **SQLite Single Writer**: SQLite connection pool is locked to `MaxOpenConns=1`. Never open nested transactions.
3. **Frontend Re-Embed**: `dist/` is embedded in the Go binary. After editing frontend and running `make test-front`, run `touch main.go` so `air` re-embeds the assets.
4. **PWA Cache Busting**: The service worker caches aggressively. When validating UI changes in the browser, append `?t=$(date +%s)` to the test URL.
5. **Antigravity NAS Daemon Environment**: The webhook daemon (`scripts/github-pr-daemon/server.py`) running on the NAS is equipped with Git, Make, Docker CLI, Docker Compose, and access to `/var/run/docker.sock` and `.env.local` secrets. It can execute test/lint suites and generate implementation plans directly on the NAS.
6. **Title Merge Semantics (`TitleWriter.Merge`)**: Consolidating source into dest transfers missing external IDs and metadata (`sonarr_id`, `radarr_id`, `my_rating`, `cover_url`, `overview`, etc.) via `COALESCE`, maintains `arr_ignored = 0` if either was queued, preserves watched state (`watched = 1`, `external_source_id`) on colliding season episodes without dropping history, re-parents `watch_events.episode_id`, copies `title_genres`, recalculates `total_watch_minutes`, and purges orphan tasks.
7. **Frontend i18n & Hardcoded Text Prevention**: English is the source language for all code, components, tests, and comments. Hardcoded French strings or French comments outside `locales/fr.ts` are forbidden and systematically enforced by both `make test-front` (`src/i18n/i18n-audit.test.ts`) and `make lint-front` (`npm run lint:i18n`). Exclusions for legitimate proper names or specific test assertions require an explicit `// i18n-ignore` inline annotation.
