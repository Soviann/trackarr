# Patterns & Codebase Map

[← Back to Index](INDEX.md)

---

## LLM Deep Dive Specifications
- Matching Pipeline & AI Auto-Confirm: `docs/dev/matching-pipeline.md`
- AniList GraphQL, Prequel Chaining & Multi-Parts: `docs/dev/anilist-sync.md`
- SQLite WAL, Writer/Reader Contracts & Deadlocks: `docs/dev/database-model.md`
- Radarr / Sonarr (*arr) Queue & Push Workflow: `docs/dev/arr-integration.md`
- Background Jobs & Task Queue Architecture: `docs/background-jobs.md`
- Native Go MCP Server (stdio): `docs/dev/mcp-server.md`

## Human / User Documentation
- Master Index: `docs/INDEX.md`
- Overview & Project Scope: `docs/overview.md`
- User Guide & Troubleshooting: `docs/user-guide.md`
- API Setup & Credentials: `docs/api-setup.md`
- Integrations & Webhooks: `docs/integrations.md`
- Deployment & Updates (3 Options): `docs/deployment.md`
- Maintenance & Operations: `docs/maintenance.md`
- Design Revamp Roadmap: `docs/roadmap-design-revamp.md`
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
| `trackarr-mcp` | `cmd/trackarr-mcp/` | Standalone native Go Model Context Protocol (MCP) server for AI agents. |
| `make version` | `Makefile` | Print Trackarr version (`Trackarr vX.Y.Z`). |
| `make build-mcp` | `Makefile` | Build standalone MCP server binary (`./tmp/trackarr-mcp`). |
| `make reset-password` | `Makefile` | CLI wrapper to reset admin password locally (`PASSWORD=...`, `USERNAME=...`). |
| `make ssh-db-pull` | `Makefile.local` | Pull prod DB (`trackarr.db` + WAL/SHM) from NAS to local `data/`. |
| `make ssh-logs` | `Makefile.local` | Dump prod container logs to `data/trackarr.log` (`LINES=...` optional). |
| `make ssh-debug-pull`| `Makefile.local` | Combined target: downloads prod DB and logs for local-first debugging. |
| `make reset-import` | `Makefile` | Resets local DB and imports a Simkl backup (`BACKUP_FILE=`). |

### Schema & Models (`internal/model/`)

| Model | File | Description & Key Fields |
|---|---|---|
| `Title` | `internal/model/title.go` | `ID`, `Type`, `Status`, `SeriesStatus`, `MatchStatus`, `TMDBID`, `TVDBID`, `AniListID`, `SimklID`, `RadarrID`, `SonarrID`, `SonarrDeletedAt`, `ArrIgnored`, `OriginCountry`, `TotalWatchMinutes`, `AccentHex`, `WatchProviders`, `PersonalNotes`. |
| `TitleName` | `internal/model/title.go` | Multi-language and alternative alias names (`TitleID`, `Language`, `Name`). |
| `Season` | `internal/model/season.go` | `ID`, `TitleID`, `SeasonNumber`, `EpisodeCount`, `WatchedCount`, `AirDate`. |
| `Episode` | `internal/model/episode.go` | `ID`, `SeasonID`, `EpisodeNumber`, `Name`, `Watched`, `LastWatchedAt`. |
| `WatchEvent`| `internal/model/watch_event.go` | Scrobble log (`ID`, `TitleID`, `EpisodeID`, `Source`, `WatchedAt`). |
| `Task` | `internal/model/task.go` | Background queue item (`ID`, `Type`, `Payload`, `Status`, `Attempts`, `RunAt`, `DedupKey`). |
| `MatchEvent`| `internal/model/match_event.go` | Match audit trail (`ID`, `TitleID`, `Kind`, `Detail`, `CreatedAt`). |
| `Setting` | `internal/model/setting.go` | Key-value string pair (`Key`, `Value`). |
| `TitleRelation` | `internal/model/title_relation.go` | Side stories, movies, sagas, and franchise relations (`TitleID`, `SeasonID`, `Provider`, `ExternalID`, `RelationType`, `Format`, `MatchedTitleID`). |
| `NextEpisode` | `internal/model/title.go` | First unwatched episode projection (`ID`, `SeasonID`, `Episode`, `SeasonNumber`, `Name`, `AirDate`, `IsTBA`). |
| `WrappedResponse` / `Stats` | `internal/model/stats.go` | Comprehensive statistics, actor/director rankings, and annual Wrapped payload (`WrappedResponse`, `WrappedAIPersona`, `WrappedArchiveItem`). |
| `APIKey` | `internal/model/api_key.go` | Machine-to-machine authentication credential (`ID`, `Name`, `KeyHash`, `KeyPrefix`, `Scopes`, `CreatedAt`, `LastUsedAt`, `RevokedAt`). |

### Repositories (`internal/repository/`)

| Repository | Reader Methods (`DBTX`) | Writer Methods (`*sql.Tx`) |
|---|---|---|
| `Title` | `GetByID`, `List`, `ListAll`, `FindByExternalID`, `ListOriginCountries`, `HasWatchedEpisodes`, `HasUnwatchedEpisodes`, search in `title_search.go` | `Create`, `Update`, `UpdateLastWatchedAt`, `ReplaceNames`, `AddMissingNames`, `Merge`, `Delete`, `BatchDelete`, `BatchStatus` |
| `APIKey` | `List`, `GetByHash`, `GetByID` | `Create`, `Revoke`, `Delete`, `UpdateLastUsedAt` |
| `TitleRelation` | `GetByTitleID`, `GetBySeasonID` | `UpsertBatch`, `DeleteForTitle` |
| `Season` | `GetByID`, `ListByTitleID` | `GetOrCreate`, `UpdateRating`, `UpdateTotalEpisodes`, `Upsert` |
| `Episode` | `GetBySeasonID`, `GetByID` | `GetOrCreate`, `ToggleWatched`, `BatchMarkWatched`, `UpdateMetadata`, `UpsertBatch`, `MarkWatched`, `MarkAllWatchedForTitle` |
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
| `ArrService` | `internal/service/arr.go` | Radarr/Sonarr proxy, push, and deletion enqueuing | `docs/dev/arr-integration.md` |
| `AniListPushService` | `internal/service/anilist_push.go` | AniList GraphQL state push (per season part / movie) | `docs/dev/anilist-sync.md` |
| `BackfillService` | `internal/service/backfill.go` | Episode metadata backfilling (opens isolated writeDB tx) | `docs/dev/database-model.md` |
| `BackupService` | `internal/service/backup.go` | 1-Click JSON/CSV/Trakt exports and transactional archive import with dry-run | `docs/maintenance.md` |
| `CoverService` | `internal/service/cover.go` | Cover downloading and accent color extraction (`colorextract/`) | `docs/patterns.md` |
| `ProwlarrService` | `internal/service/prowlarr.go` | Prowlarr indexer search (releases), memory caching, and poster resolution | `docs/patterns.md` |
| `APILimiter` | `internal/service/ratelimiter.go` | Global 2 rps token bucket for external APIs | `docs/patterns.md` |
| `CalendarService` | `internal/service/calendar.go` | RFC 5545 iCalendar generation, token rotation & range queries | `docs/user-guide.md` |
| `BackgroundService` / `MetadataSyncService` | `internal/service/background.go` | Title metadata enrichment and synchronization (TMDB, AniList, TVDB) | `docs/background-jobs.md` |
| `Scheduler` | `internal/service/scheduler.go` | Periodic cron orchestrator (daily refresh, cover maintenance, wrapped checks) | `docs/background-jobs.md` |
| `SeasonAuditService` | `internal/service/seasonaudit.go` | Split season detection and suggested merge engine | `docs/dev/anilist-sync.md` |
| `SimklImporter` | `internal/service/simkl.go` | Simkl backup archive parser and database populator | `docs/deployment.md` |
| `TaskQueueWorker` | `internal/service/taskqueue.go` | Asynchronous task execution engine with priority ordering (interactive pushes ahead of bulk enrichment) | `docs/background-jobs.md` |
| `APIKeyService` | `internal/service/api_key.go` | Labeled API Key authentication, format validation, memory-throttled usage tracking | `docs/patterns.md` |

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
| POST | `/api/auth/logout` | `auth.Logout` | Clear JWT auth cookie |
| GET | `/api/admin/api-keys` | `adminAPIKeys.List` | List all labeled API keys |
| POST | `/api/admin/api-keys` | `adminAPIKeys.Create` | Create a new labeled API key with granular scopes |
| POST | `/api/admin/api-keys/{id}/revoke` | `adminAPIKeys.Revoke` | Revoke active API key |
| DELETE | `/api/admin/api-keys/{id}` | `adminAPIKeys.Delete` | Permanently remove API key |
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
| POST | `/api/titles/batch` | `titles.BatchCreate` | Bulk ingest titles (up to 100) with local regex URL parsing, deduplication, and task prioritization |
| GET | `/api/titles/review-count` | `titles.ReviewCount` | Badge count for review/unconfirmed titles |
| GET | `/api/titles/resolve` | `titles.Resolve` | Preview external metadata before creating a title |
| GET | `/api/titles/continue-watching` | `library.ContinueWatching` | Continue watching grid list with progress, providers & next episode |
| GET | `/api/titles/upcoming` | `library.Upcoming` | Upcoming titles grid list |
| POST | `/api/titles/batch-delete` | `titles.BatchDelete` | Delete multiple titles |
| POST | `/api/titles/batch-status` | `titles.BatchStatus` | Bulk update title statuses |
| GET | `/api/titles/{id}` | `titles.GetByID` | Detailed title payload |
| PATCH | `/api/titles/{id}` | `titles.Update` | Update status, rating, or metadata |
| DELETE | `/api/titles/{id}` | `titles.Delete` | Delete title and cascade associations |
| POST | `/api/titles/{id}/rematch` | `titles.Rematch` | Update external IDs and title type, trigger re-enrichment |
| PUT | `/api/titles/{id}/external-ids` | `titles.SetExternalIDs` | Explicitly overwrite external IDs |
| POST | `/api/titles/{id}/merge` | `titles.Merge` | Merge source title into target |
| POST | `/api/titles/{id}/refresh` | `titles.RefreshOne` | Force immediate metadata refresh (supports `?sync=true` for synchronous execution) |
| GET | `/api/titles/{id}/history` | `history.Get` | Detailed watch event history for title |
| GET | `/api/tmdb/search` | `tmdbSearch.Search` | Search TMDB for movie or TV titles (supports `type=all` / `type=multi`) |
| GET | `/api/anilist/search` | `anilistSearch.Search` | Search AniList for anime titles |
| GET | `/api/releases` | `releasesHandler.List` | Latest Prowlarr releases with posters & local match |
| POST | `/api/releases/add` | `releasesHandler.Add` | Direct 1-click title creation from release |
| PATCH | `/api/titles/{titleID}/episodes/{episodeID}` | `episodes.ToggleWatched` | Mark episode watched / unwatched |
| POST | `/api/titles/{titleID}/episodes/batch-watch` | `episodes.BatchMarkWatched` | Bulk mark episodes watched or unwatched (`watched: false`) |
| POST | `/api/titles/{titleID}/seasons/{seasonID}/anilist` | `seasonExternal.AddAniListID` | Attach AniList part to season (provisions Season 1 on demand if seasonID <= 0) |
| DELETE| `/api/titles/{titleID}/seasons/{seasonID}/anilist/{externalID}` | `seasonExternal.RemoveAniListID` | Detach AniList part |
| PUT | `/api/titles/{titleID}/seasons/{seasonID}/anilist/order` | `seasonExternal.ReorderAniList` | Reorder AniList parts |
| POST | `/api/push/subscribe` | `push.Subscribe` | Register Web Push subscription |
| DELETE | `/api/push/subscribe` | `push.Unsubscribe` | Remove Web Push subscription |
| GET | `/api/stats` | `stats.Get` | Library metrics, genre/people distributions & insight cards with timeframe, year and media type filters |
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
| GET | `/api/admin/auth-settings` | `auth.GetAuthSettings` | Get auth mode and configuration state |
| PUT | `/api/admin/auth-settings` | `auth.UpdateAuthSettings` | Update auth mode (`google`, `password`, `hybrid`) |
| GET | `/api/admin/system-settings` | `adminSettings.GetSystemSettings` | Read current system configuration (including `app_version`), masked secrets and webhook URLs |
| PUT | `/api/admin/system-settings` | `adminSettings.UpdateSystemSettings` | Update configuration in SQLite & trigger client hot-reload |
| POST | `/api/admin/system-settings/test/tmdb` | `adminSettings.TestTMDB` | Test TMDB connection |
| POST | `/api/admin/system-settings/test/tvdb` | `adminSettings.TestTVDB` | Test TVDB connection |
| POST | `/api/admin/system-settings/test/gemini` | `adminSettings.TestGemini` | Test Gemini AI connection |
| POST | `/api/admin/system-settings/test/{app}` | `adminSettings.TestArr` | Test Radarr, Sonarr, Prowlarr connection |
| POST | `/api/admin/system-settings/vapid/generate` | `adminSettings.GenerateVAPIDKeys` | Generate NIST P-256 VAPID keypair |
| POST | `/api/admin/refresh-all` | `admin.RefreshAll` | Trigger full library refresh (supports `?restart=true`) |
| GET | `/api/admin/refresh-all/status` | `admin.GetRefreshAllStatus` | Query current library refresh progress and state |
| POST | `/api/admin/refresh-all/cancel` | `admin.CancelRefreshAll` | Cancel or pause running library refresh job |
| GET | `/api/admin/export/json` | `admin.ExportJSON` | 1-Click full JSON library backup download |
| GET | `/api/admin/export/csv` | `admin.ExportCSV` | 1-Click spreadsheet CSV library export download |
| GET | `/api/admin/export/trakt` | `admin.ExportTrakt` | 1-Click Trakt.tv sync JSON export download |
| POST | `/api/admin/import` | `admin.ImportBackup` | Upload and import backup (.zip, .json, .csv) with dry-run support |
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
- **Contextual Active Tab Highlighting (`getActiveTab` in `components/navItems.tsx`)**: Bottom navigation bar and lateral sidebar evaluate `getActiveTab(currentPath)` to preserve active tab context across nested workflows: `/admin/validate` and `/add` map to `/search` (`Explore`), `/continue-watching` maps to `/` (`Collection`), `/releases` maps to `/coming-up` (`Calendar`), while `/admin/*` subroutes remain mapped to `/admin`.

### State Management (`frontend/src/store.ts`)
- `useTitleStore`: Zustand store for title listing, pagination, sorting (`localStorage`), and session-persistent filters (`status`, `type`, `is_anime`, `series_status`, `decade`, `release_from`, `release_to`, `genres`, `origin_country`, `my_rating_min`, `tmdb_rating_min`). Filters persist across title details and navigation, resetting only on explicit user action or reload.
- `useSearchStore`: Search query state, debounce, and TMDB toggle.

### Components Map (`frontend/src/components/`)

| Component | File | Purpose |
|---|---|---|
| `Navbar` | `components/Navbar.tsx` | 5-tab bottom navigation bar (`Collection`, `Explore`, `Calendar`, `Stats`, `Admin`) with localized labels, safe-area hit targets, and accessible ARIA attributes |
| `AdminHeader` | `components/AdminHeader.tsx` | Reusable sticky header for admin pages with 34px circular back button, deterministic navigation, title, optional badge slot, and action children slot |
| `Sidebar` | `components/Sidebar.tsx` | Desktop collapsible lateral navigation sidebar (≥ 1024px) with Trackarr branding, vertical navigation tabs, active state indicators, and collapse/expand toggle |
| `navItems` | `components/navItems.tsx` | 5-tab definitions (`navTabs`), SVG icons, and contextual active tab routing (`getActiveTab`) |
| `ActionDrawer` | `components/ActionDrawer.tsx` | Slide-up drawer exposing management actions for titles and seasons with GPU-accelerated translateY transitions, 38px docked handle, and overscroll containment |
| `SectionCards` | `components/SectionCards.tsx` | 3-column hub cards header on Library page with poster slices backdrop and editorial titles |
| `SectionRow` | `components/SectionRow.tsx` | Section row header container with count pill and action buttons |
| `TitleCard` | `components/TitleCard.tsx` | Horizontal list card with progress bar, quick mark action, and caught-up status |
| `PosterCard` | `components/PosterCard.tsx` | 2:3 vertical grid poster card with type badge, quick mark +1 (44×44px touch target, event isolation), and Arr availability badge |
| `PosterTile` | `components/PosterTile.tsx` | Compact poster card for preset strips and grids with quick mark +1 (44×44px touch target, event isolation) |
| `PosterStrip` | `components/PosterStrip.tsx` | Horizontal scrolling strip of poster thumbnails |
| `CoverImage` | `components/CoverImage.tsx` | Resilient image loader with fallback handling and caching |
| `CoverPlaceholder` | `components/CoverPlaceholder.tsx` | Geometric stylized placeholder when cover art is unavailable |
| `TypeBadge` | `components/TypeBadge.tsx` | Movie/Series badge with optional colored Arr top accent border |
| `StatusBadge` | `components/StatusBadge.tsx` | Localized pill badge indicating watch and release statuses (*Watching*, *Completed*, *Plan to Watch* / *À voir*, *Dropped*, *Caught Up* / *À jour*) |
| `ArrBadge` | `components/ArrBadge.tsx` | State pill indicating Radarr/Sonarr status (*In Queue*, *Downloaded*, *Monitored*) |
| `FilterDrawer`| `components/FilterDrawer.tsx` | Compact filter panel with zero-reflow CSS Grid row collapse and GPU translateY transitions, docked handle row (`[ FILTERS (count) ⌃ ]` + scrolling chips), 3 sub-tabs, and overscroll containment |
| `SearchBar` | `components/SearchBar.tsx` | Docked search input bound to `useSearchStore` with media URL auto-detection (IMDb, TMDB, AniList), clear text `✕` button, and integrated filter trigger button with active count badge |
| `SeasonAniListStrip` | `components/SeasonAniListStrip.tsx` | Active season AniList score and multi-part management strip |
| `SeasonSideStories` | `components/SeasonSideStories.tsx` | Inline cards for side stories and movies recommended at the end of the active season |
| `SeasonTab` | `components/SeasonTab.tsx` | Interactive tab button for switching season views in TitleDetail |
| `EpisodeRow` | `components/EpisodeRow.tsx` | Episode listing row with title, air date, and interactive toggle checkmark |
| `FranchiseRelationsSection` | `components/FranchiseRelationsSection.tsx` | Combined Hub Bar & slide-up BottomSheet drawer for Sagas & Franchise tracking (progress bar, next chronological title, category filters, timeline/release sort toggle) and unified Watch History glance row |
| `TitleHistory` | `components/TitleHistory.tsx` | Slide-up BottomSheet drawer displaying episode watch dates, season breakdowns, and rewatch badges |
| `RematchSheet`| `components/RematchSheet.tsx` | TMDB search & manual ID fixer for titles or season AniList parts |
| `AniListSheet`| `components/AniListSheet.tsx` | Slide-up modal sheet for editing AniList multi-part season associations |
| `MatchReviewCard` | `components/MatchReviewCard.tsx` | Review card with external ID chips, confirm, and fix actions |
| `RatingPrompt`| `components/RatingPrompt.tsx` | 10-star rating popup with AniList / IMDb shortcuts |
| `EditSheet` | `components/EditSheet.tsx` | Quick edit for status, type, and display title (with optional Sonarr deletion when marking series dropped) |
| `ArrPushSheet` | `components/ArrPushSheet.tsx` | Slide-up modal sheet to configure options and push title directly to Radarr/Sonarr |
| `ReleaseDetailSheet` | `components/ReleaseDetailSheet.tsx` | Slide-up modal sheet displaying torrent metadata, scene release name, external links and 1-click addition button |
| `NextEpisodeHero` | `components/NextEpisodeHero.tsx` | Prominent call-to-action hero card with 1-click next episode mark & binge duration estimator |
| `PersonalNotesCard` | `components/PersonalNotesCard.tsx` | Private memo/notes card on title detail with debounced auto-save |
| `WatchProviderBadges` | `components/WatchProviderBadges.tsx` | Streaming provider badges (Netflix, Prime Video, Disney+, Apple TV+, Max, Canal+, Crunchyroll, Paramount+, ADN) |
| `CalendarMonthGrid` | `components/CalendarMonthGrid.tsx` | 7-column monthly interactive grid with today indicator & selected day release cards |
| `CalendarWeekTimeline` | `components/CalendarWeekTimeline.tsx` | 7-day weekly timeline view with rich release cards |
| `CalendarIcalModal` | `components/CalendarIcalModal.tsx` | iCal subscription modal with 1-click URL copy, Apple/Google links, and token rotation |
| `PersonFilmographyDrawer` | `components/PersonFilmographyDrawer.tsx` | Slide-up modal sheet listing filmography and library titles for a given actor or director |
| `PrimeBadge` | `components/PrimeBadge.tsx` | Amazon Prime Video source badge |
| `ConfirmationDrawer` | `components/ConfirmationDrawer.tsx` | Slide-up confirmation modal with affirmative/cancel actions (safeguards mass confirm in MatchReview, season merges in AdminSeasonAudit, disconnect in AdminAniList, re-adding deleted series in TitleDetail) |
| `UndoSnackbar` | `components/UndoSnackbar.tsx` | Universal floating undo notification with perimeter radial/clock timer countdown (5s) for state-changing actions (+1, episode/movie mark, title deletion) |
| `CollapsibleSection` | `components/CollapsibleSection.tsx` | Foldable accordion container with toggle indicator |
| `useSwipeDownToClose` | `hooks/useSwipeDownToClose.ts` | Reusable touch hook managing drag offset, single-touch guard, internal scroll check (scrollTop > 0), event isolation, and threshold close |
| `useKeyboardShortcuts` | `hooks/useKeyboardShortcuts.ts` | Global power user keyboard shortcut listener (`Cmd+K` / `Ctrl+K` & `/` search trigger, `Escape` drawer & dialog dismissal) |
| `BottomSheet` | `components/BottomSheet.tsx` | Slide-up modal sheet with drag gestures and backdrop |
| `PullToRefresh`| `components/PullToRefresh.tsx` | Touch-based pull-to-refresh wrapper with disabled state when drawers or dialog sheets are open |
| `SwipeActions`| `components/SwipeActions.tsx` | Swipeable item revealing action buttons with zero-reflow CSS Grid row collapse and GPU slide-out exit animation |
| `ErrorBanner` | `components/ErrorBanner.tsx` | Dismissible alert banner for API and network errors |
| `ErrorBoundary` | `components/ErrorBoundary.tsx` | React error boundary with error recovery fallback |

### Design Tokens & Theming (`frontend/src/tokens.css`)
- **Theme Variables**: 4 distinct themes (`vault`, `cyber`, `sunset`, `emerald`).
- **Contrast Token**: `--accent-fg` dynamically guarantees WCAG compliant text contrast on top of `--accent` across all themes (dark background `var(--bg)` on Vault, Cyber, Sunset, and Emerald).
- **Secondary Text Token**: `--ink-mute` is calibrated across all themes to meet WCAG AA contrast (≥ 4.5:1 on dark background).
- **Brand Tokens**: Dedicated tokens `--brand-imdb`, `--brand-tmdb`, `--brand-tvdb`, `--brand-anilist`, `--brand-radarr` (`#ffc230`), and `--brand-sonarr` (`#00c0ff`).
- **Dynamic SVG Theming**: SVGs use `currentColor` or CSS variables (`var(--accent)`, `var(--ink)`, `var(--ink-dim)`) instead of static JS imports from `theme.ts`.
- **Skeleton Shimmer Waves**: `--skeleton-bg` token combined with `@keyframes shimmerWave` provides directional animated gradient waves across all loading skeletons.

### Pages Map (`frontend/src/pages/`)

| Route | Page Component | File | Description / Layout |
|---|---|---|---|
| `/` | `Library` | `pages/Library.tsx` | Main library fluid grid (2-8 columns), custom filters, and bottom navigation |
| `/releases` | `Releases` | `pages/Releases.tsx` | Prowlarr releases feed with multi-indexer filters and 1-click addition |
| `/continue-watching` | `ContinueWatching` | `pages/ContinueWatching.tsx` | In-progress titles, quick watch buttons, and unwatched episodes tracking |
| `/coming-up` | `ComingUp` | `pages/ComingUp.tsx` | Multi-view calendar (month, week, list) and iCal feed subscription |
| `/search` | `Search` | `pages/Search.tsx` | Full-text library search with category tabs |
| `/add` | `Add` | `pages/Add.tsx` | Instant live discovery and addition: debounced search across TMDB & AniList with local library cross-checking (`In Library ↗`), 1-tap tracking status buttons (`+ Plan to Watch`, `+ Watching`) with universal undo, and URL/share-target routing |
| `/stats` | `Stats` | `pages/Stats.tsx` | Decomposed watch metrics, top actors, directors, and genre charts with responsive desktop Bento grid and accessible ARIA meters |
| `/title/:id` | `TitleDetail` | `pages/TitleDetail.tsx` | Detailed title page with responsive Studio Sidebar: full-bleed cover hero on mobile (<640px), centered single-column (<1024px), and 2-column Studio Sidebar (sticky 290px poster + quick actions + fiche technique, right column hero, ratings, elevated seasons & episodes hub) on desktop (>=1024px) |
| `/person/:name` | `PersonTitles` | `pages/PersonTitles.tsx` | Interactive filmography list for cast and directors |
| `/wrapped` | `Wrapped` | `pages/Wrapped.tsx` | Annual retrospective stories player and historical archives gallery |
| `/login` | `Login` | `pages/Login.tsx` | Local password login and Google OAuth authentication |
| `/setup` | `Setup` | `pages/Setup.tsx` | Initial administrative setup and emergency recovery key generation |
| `/match-review` | `MatchReview` | `pages/MatchReview.tsx` | Review queue for unconfirmed and low-confidence title matches |
| `/admin` | `Admin` | `pages/Admin.tsx` | Admin Hub with responsive 2-column navigation grid (1040px) and live status badges |
| `/admin/settings` | `AdminSettings` | `pages/AdminSettings.tsx` | System settings (840px): Appearance & Localization (stylized theme, interface language & metadata language dropdowns), Streaming Platforms (stylized multi-select dropdown), and Metadata & AI keys at bottom |
| `/admin/auth` | `AdminAuth` | `pages/AdminAuth.tsx` | Authentication & Security (840px): Access mode (hybrid/local/Google OAuth), local credentials, single-use emergency recovery key generation, and sticky AdminHeader |
| `/admin/arr` | `AdminArr` | `pages/AdminArr.tsx` | Arr Stack management: Radarr, Sonarr & Prowlarr connection settings, live connection tests, 2-column desktop grid for standard/anime defaults, and sticky AdminHeader |
| `/admin/tasks` | `AdminTasks` | `pages/AdminTasks.tsx` | Background task queue & diagnostics: widescreen scannable layout (1200px), sticky AdminHeader, and centered floating batch action bar |
| `/admin/notifications` | `AdminNotifications` | `pages/AdminNotifications.tsx` | Web Push Notifications: VAPID key generation and admin contact subject, automatic configuration status, and notification triggers (rating reminder, failed task, series ended) |
| `/admin/jellyfin` | `AdminJellyfin` | `pages/AdminJellyfin.tsx` | Media Servers & Webhooks: Dual server management for Jellyfin and Plex scrobble webhook secrets, full webhook URLs with copy actions, last scrobble timestamps, and configuration guides |
| `/admin/anilist` | `AdminAniList` | `pages/AdminAniList.tsx` | AniList OAuth integration (840px): connection status, live token refresh, disconnect confirmation modal, and sticky AdminHeader |
| `/admin/season-audit` | `AdminSeasonAudit` | `pages/AdminSeasonAudit.tsx` | Split anime season audit: responsive visual diff connectors (1200px), sticky AdminHeader, and merge modal drawers |
| `/admin/validate` | `Validate` | `pages/Validate.tsx` | Match validation & rematching workspace: interactive status picker, search drawer, and merge utilities |
| `/admin/help` | `Help` | `pages/Help.tsx` | In-app user guides, FAQs, and setup instructions accordion |
| `/anilist/callback` | `AnilistCallback` | `pages/AnilistCallback.tsx` | AniList OAuth authorization code return handler |

---

## Core Invariants & Engineering Rules

1. **Production Debugging (Local-First)**: Never run exploratory commands or grep logs live on production via SSH. Pull state locally with `make ssh-debug-pull` and debug against `data/trackarr.db` and `data/trackarr.log`.
2. **SQLite Single Writer**: SQLite connection pool is locked to `MaxOpenConns=1`. Never open nested transactions.
3. **Frontend Re-Embed**: `dist/` is embedded in the Go binary. After editing frontend and running `make test-front`, run `touch main.go` so `air` re-embeds the assets.
4. **PWA Cache Busting**: The service worker caches aggressively. When validating UI changes in the browser, append `?t=$(date +%s)` to the test URL.
5. **Antigravity NAS Daemon Environment**: The webhook daemon (`scripts/github-pr-daemon/server.py`) running on the NAS is equipped with Git, Make, Docker CLI, Docker Compose, and access to `/var/run/docker.sock` and `.env.local` secrets. It can execute test/lint suites and generate implementation plans directly on the NAS.
6. **Title Merge Semantics (`TitleWriter.Merge`)**: Consolidating source into dest transfers missing external IDs and metadata (`sonarr_id`, `radarr_id`, `sonarr_deleted_at`, `my_rating`, `cover_url`, `overview`, etc.) via `COALESCE`, maintains `arr_ignored = 0` if either was queued, preserves watched state (`watched = 1`, `external_source_id`) on colliding season episodes without dropping history, re-parents `watch_events.episode_id`, copies `title_genres`, recalculates `total_watch_minutes`, and purges orphan tasks.
7. **Frontend i18n & Hardcoded Text Prevention**: English is the source language for all code, components, tests, and comments. Hardcoded French strings or French comments outside `locales/fr.ts` are forbidden and systematically enforced by both `make test-front` (`src/i18n/i18n-audit.test.ts`) and `make lint-front` (`npm run lint:i18n`). Exclusions for legitimate proper names or specific test assertions require an explicit `// i18n-ignore` inline annotation.
8. **Responsive Ergonomics & Desktop Navigation**: Viewports ≥ 1024px engage `Sidebar` (collapsible rail persisted in `localStorage`) and fluid shelf grid (`auto-fill, minmax(105px..160px, 1fr)`). Navigation state follows contextual highlighting (`getActiveTab`) where `/add` and `/admin/validate` preserve the `Explore` tab context.
