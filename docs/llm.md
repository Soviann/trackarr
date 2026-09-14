# LLM Context Reference (High-Density Specification)

[← Back to Index](INDEX.md)

---

## 1. Core Architecture
- **Stack**: Go 1.24, SQLite 3 (WAL mode, FTS5), `chi/v5`, Preact 10, TypeScript (strict), Vite.
- **Entrypoint**: `main.go` ➔ CLI dispatcher (`serve`, `import`, `migrate`, `reset-password`, `backfill-accents`, `version`).
- **HTTP Server**: `internal/router/router.go` wires middleware (CORS, Auth, Compression, Recovery, Rate Limiting) and routes.
- **Dependency Injection**:
  - `internal/database`: `DBTX` interface (`*sql.DB` or `*sql.Tx`), helper `WithTx` / `WithTxContext`.
  - `internal/repository`: Owns 100% of SQL. Read queries via structs, write queries via dedicated `*_writer.go` (requiring `*sql.Tx`).
  - `internal/service`: Business logic, AniList GraphQL client, *arr API clients, Matching pipeline, Task queue, Backup & export.
  - `internal/handler`: HTTP request decoding, DTO parsing, `httputil.WriteJSON`.

---

## 2. Invariants & Critical Gotchas
1. **SQLite Concurrency**: `MaxOpenConns = 1` for write safety. Always close `rows.Close()` before issuing nested queries. Never run transactions across unbuffered row iterators.
2. **Matching Status State Machine**:
   `titles.match_status` ∈ `{'confirmed', 'pending_review', 'unconfirmed'}`.
   - `confirmed`: Fully identified, safe for auto-sync and Arr queueing.
   - `unconfirmed`: Low/medium confidence match or AI failure; shown in `/match-review`.
   - `pending_review`: Unconfirmed match queued when Gemini AI verifier was unavailable.
3. **Quick Mark & Arr Availability Rules**:
   - Quick +1 episode mark and Arr availability badge (`SxxExx DISPO`) are strictly hidden on titles with `status = 'dropped'`.
   - Quick +1 episode mark, Arr availability badge (`SxxExx DISPO`), and Next Episode hero banner are strictly hidden for unaired episodes (future air date or missing air date) and placeholder episodes marked as "TBA" or "TBD" (`IsTBA = true`).
   - "Caught up" status is derived and propagated dynamically when all currently aired episodes have been watched while future episodes remain scheduled.
   - Bulk completion (`MarkAllWatchedForTitle`) and unwatched episode checks (`HasUnwatchedEpisodes`) strictly ignore unaired future episodes and TBA/TBD placeholders, ensuring completed series do not falsely mark future episodes as watched and ended series auto-complete cleanly.
4. **AniList Synchronization Constraints**:
   - Scores (1–10) are only pushed when anime status is `Completed` or `Dropped` (AniList API restriction).
   - Multi-part seasons map to separate AniList IDs in `season_external_ids` (`provider = 'anilist'`); episode counts are distributed sequentially across parts.
5. **Duplicate Detection & Union-Find**:
   - `DuplicateSeriesGroups` queries series sharing `imdb_id`, `tmdb_id` (>0), or `tvdb_id` (>0). Empty strings (`""`) and `0` values are strictly excluded.
   - Results are unified using Disjoint-Set Union (Union-Find) and sorted deterministically.
6. **Universal Undo Snackbar & Haptic Feedback**:
   - State-changing actions (`+1` quick-mark, episode watch toggles, movie watch status, and title deletion) trigger a floating `UndoSnackbar` with a 5-second circular perimeter radial countdown.
   - Quick-mark `+1` buttons trigger an overshoot spring animation (`springPop`) and haptic vibration (`haptic([15, 30, 15])`).
   - Destructive title deletion employs delayed execution (`onExpire`) allowing full undo prior to permanent SQLite purge.
   - Destructive and bulk operations (mass match confirmation, season merges, AniList disconnection) are guarded by `ConfirmationDrawer`.
7. **Session Filter Persistence & Single-Line Handle**:
   - `FilterDrawer` closed handle is a 42px row docked above the navbar with `[ FILTERS (count) ⌃ ]` trigger and horizontally scrolling dismissible chips (`✕`) to remove criteria without opening the drawer.
   - Active filter state is held in Zustand `useTitleStore` and persists across title navigation and tab switches until explicit reset or reload.
8. **Unified Hub Bar & BottomSheet Drawers**:
   - On `TitleDetail`, the previous stacked ~720px franchise card and history buttons are unified into a compact ~100px Hub Bar with summary glance rows, opening slide-up `BottomSheet` drawers and elevating season/episode lists by ~600px for immediate mobile visibility.

---

## 3. Database Schema Overview
- `titles`: Core media table (`id`, `type: 'movie'|'series'`, `is_anime`, `status: 'watching'|'plan_to_watch'|'completed'|'dropped'`, `series_status: 'returning'|'ended'|'cancelled'|'in_production'`, `match_status: 'confirmed'|'pending_review'|'unconfirmed'`, `year`, `cover_url`, `imdb_id`, `tmdb_id`, `tvdb_id`, `anilist_id`, `simkl_id`, `radarr_id`, `sonarr_id`, `arr_ignored`, `origin_country`, `total_watch_minutes`, `accent_hex`, `watch_providers`, `personal_notes`).
- `title_names`: Multilingual title aliases (`title_id`, `name`, `language`, `is_primary`).
- `title_genres`: Associated genres per title (`title_id`, `genre`).
- `seasons`: Season entries (`title_id`, `season_number`, `total_episodes`).
- `episodes`: Episode entries (`season_id`, `episode`, `name`, `air_date`, `watched`, `watched_at`, `external_source_id`).
- `watch_events`: Watch history logs (`title_id`, `episode_id`, `source: 'manual'|'jellyfin'|'plex'|'emby'|'simkl'|'backfill'`, `raw_payload`, `created_at`).
- `task_queue`: Asynchronous background jobs (`task_type`, `payload`, `status: 'pending'|'running'|'completed'|'failed'|'dead'`, `attempts`, `run_at`, `dedup_key`).
- `match_events`: Match audit trail (`title_id`, `kind`, `detail`, `created_at`).
- `season_external_ids`: Per-part AniList season mappings (`season_id`, `provider: 'anilist'`, `external_id`, `anilist_episode_count`, `anilist_start_date`, `anilist_average_score`, `sort_order`).
- `title_relations`: Side stories, sagas, and franchise relations (`title_id`, `season_id`, `provider: 'anilist'|'tmdb'|'tvdb'`, `external_id`, `relation_type: 'PREQUEL'|'SEQUEL'|'SPIN_OFF'|'SIDE_STORY'|'ALTERNATIVE'|'COLLECTION'`, `format`, `title`, `cover_url`, `year`, `score`, `overview`, `sort_order`).
- `season_audit_dismissals`: Dismissed merge proposals (`source_title_id`, `target_title_id`).
- `wrapped_snapshots`: Immutable annual retrospective snapshots (`year`, `data_json`, `created_at`).
- `settings`: Key-value config store (`radarr_url`, `sonarr_url`, `prowlarr_url`, `admin_password_hash`, `admin_recovery_key_hash`, `jwt_secret`, `vapid_public_key`, `vapid_private_key`, `push_subscription`, `metadata_language`, `enabled_watch_providers`, `calendar_token`, notification preferences). Sessions use signed JWT in HTTP cookies.

---

## 4. API Endpoints Map
- `GET /api/health` : Health check endpoint.
- `GET /api/config` : Pre-auth public configuration (Google Client ID, VAPID, Auth Mode, Setup status, Metadata Language, Enabled Watch Providers).
- `POST /api/auth/setup` : Initial admin account setup & emergency recovery key generation.
- `POST /api/auth/login` : Local password login (Bcrypt, constant-time, rate-limited).
- `POST /api/auth/google` : OAuth Google authentication callback.
- `POST /api/auth/recover` : Emergency recovery with key (`TRCK-...`) & auto-regeneration.
- `POST /api/auth/change-password` : Authenticated password change & auto-regeneration.
- `POST /api/auth/recovery-key/regenerate` : Regenerate emergency recovery key.
- `POST /api/auth/logout` : Clear JWT auth cookie.
- `GET /api/covers/{filename}` : Cached cover image loader (also `/covers/{filename}`).
- `GET /api/titles` : List titles with status/type/genre/person/country filtering, sorting, pagination.
- `GET /api/titles/{id}` : Full title entity with names, seasons, episodes, cast, external IDs.
- `POST /api/titles` : Create title (supports manual input or direct URL matching).
- `PATCH /api/titles/{id}` : Update status, rating, match status, type, anime flag, arr ignored, or personal notes.
- `DELETE /api/titles/{id}` : Delete title and cascaded relations.
- `POST /api/titles/{id}/rematch` : Rematch title with external IDs or title type.
- `PUT /api/titles/{id}/external-ids` : Explicitly overwrite external IDs.
- `POST /api/titles/{id}/merge` : Merge source title into target title with optional season offset.
- `POST /api/titles/{id}/refresh` : Trigger metadata re-fetch for a single title (supports `?sync=true` for synchronous execution).
- `GET /api/titles/{id}/history` : Detailed watch event history for title.
- `GET /api/tmdb/search` : Search TMDB for movie or TV titles (supports `type=all` / `type=multi`).
- `GET /api/anilist/search` : Search AniList for anime titles with posters and metadata.
- `GET /api/titles/resolve` : Resolve external metadata from direct URL (IMDb, TMDB, TVDB, AniList).
- `GET /api/titles/continue-watching` : List in-progress titles with progress ratios, streaming providers, and next unwatched episode.
- `GET /api/titles/upcoming` : List series with upcoming air dates.
- `GET /api/calendar.ics` : Public token-secured RFC 5545 iCalendar feed (`?token=...`).
- `GET /api/calendar/events` : List calendar events for in-app month/week/list views.
- `GET /api/calendar/token` : Get active calendar token and subscription URLs.
- `POST /api/calendar/token/regenerate` : Rotate calendar secret token.
- `GET /api/titles/review-count` : Badge count for unconfirmed/pending review titles.
- `POST /api/titles/batch-delete` : Delete multiple titles in a single transaction.
- `POST /api/titles/batch-status` : Update status for multiple titles.
- `PATCH /api/titles/{titleID}/episodes/{episodeID}` : Toggle watched state (triggers AniList push & auto-completion check).
- `POST /api/titles/{titleID}/episodes/batch-watch` : Batch mark episodes watched or unwatched (`watched: false`) with watch time recalculation and AniList sync.
- `POST /api/titles/{titleID}/seasons/{seasonID}/anilist` : Add per-season AniList ID mapping.
- `DELETE /api/titles/{titleID}/seasons/{seasonID}/anilist/{externalID}` : Remove AniList ID mapping.
- `PUT /api/titles/{titleID}/seasons/{seasonID}/anilist/order` : Reorder multi-part AniList season mappings.
- `GET /api/genres` : List genres with counts.
- `GET /api/countries` : List origin countries with counts.
- `GET /api/settings` : User and app settings.
- `GET /api/anilist/auth` : Generate AniList OAuth authorize URL.
- `POST /api/anilist/token` : Save AniList OAuth access token.
- `DELETE /api/anilist/token` : Disconnect AniList integration.
- `GET /api/admin/auth-settings` & `PUT /api/admin/auth-settings` : Get/update auth mode (`google`, `password`, `hybrid`).
- `GET /api/admin/system-settings` & `PUT /api/admin/system-settings` : Read/update system settings in SQLite (returns `app_version`, masked secrets, integration flags) and trigger client hot-reload.
- `POST /api/admin/system-settings/test/{service}` : Test connection for TMDB, TVDB, Gemini, Radarr, Sonarr, Prowlarr.
- `POST /api/admin/system-settings/vapid/generate` : Auto-generate fresh NIST P-256 VAPID keypair.
- `GET /api/admin/counts` : Total library titles and queued items for admin navbar badge.
- `GET /api/admin/tasks` : List background queue tasks and dead tasks.
- `POST /api/admin/tasks/{id}/retry` : Retry a dead task.
- `DELETE /api/admin/tasks/{id}` : Delete individual task.
- `POST /api/admin/tasks/batch-delete` : Bulk delete tasks.
- `GET /api/admin/notifications` & `PUT /api/admin/notifications` : Web Push notification preferences.
- `GET /api/admin/arr` & `PUT /api/admin/arr` : Radarr & Sonarr configuration.
- `POST /api/admin/refresh-all` : Start or resume full library metadata refresh in background (accepts `?restart=true`).
- `GET /api/admin/refresh-all/status` : Get real-time library refresh progress (`status`, `processed_titles`, `total_titles`, `current_title`).
- `POST /api/admin/refresh-all/cancel` : Cancel or pause running library refresh job.
- `GET /api/admin/export/json` : 1-Click full JSON library backup download.
- `GET /api/admin/export/csv` : 1-Click spreadsheet CSV library export download.
- `GET /api/admin/export/trakt` : 1-Click Trakt.tv sync JSON export download.
- `POST /api/admin/import` : Upload and import backup archive (.zip, .json, .csv) with dry-run support (`?dry_run=true|false`).
- `GET /api/admin/season-audit` : Scan duplicate series groups and return consolidation proposals.
- `POST /api/admin/season-audit/accept` : Execute single season attachment and delete source stray.
- `POST /api/admin/season-audit/dismiss` : Dismiss season proposal.
- `POST /api/push/subscribe` & `DELETE /api/push/subscribe` : Web Push subscription registration.
- `POST /api/webhook/jellyfin/{secret}` : Ingest Jellyfin `PlaybackStop` webhook.
- `POST /api/webhook/plex/{secret}` : Ingest Plex `media.scrobble` webhook.
- `GET /api/arr/{app}/rootfolder` & `GET /api/arr/{app}/qualityprofile` : Proxy Radarr/Sonarr root folders and quality profiles.
- `GET /api/arr/title/{id}` & `PUT /api/arr/title/{id}` : Check Arr library status and edit quality profile / root folder.
- `POST /api/arr/push/{id}` : Push title to Radarr/Sonarr download queue.
- `POST /api/arr/queue/{id}/push` : Push title from queue directly to Radarr / Sonarr.
- `GET /api/releases` & `POST /api/releases/add` : Browse Prowlarr releases feed and 1-click import.
- `POST /api/client-errors` : Client-side error reporting.
- `GET /api/stats` : Return library metrics, genre distribution, top actors & top directors, streaks, and fun stats with filters (`timeframe=all|year|30d`, `year=YYYY`, `media_type=all|movie|series|anime`).
- `GET /api/stats/wrapped` : Annual retrospective stats (overview, category tops, release tops, rewatch champion, top cast/genres, and Gemini AI persona).
- `GET /api/stats/wrapped/archives` : List all archived Wrapped snapshots.
- `POST /api/stats/wrapped/generate` : Force generation and persistence of Wrapped snapshot for a given year.
- `GET /api/stats/activity` : Paginated feed of watch activity events.
- `GET /api/match-events` : Audit trail of auto-matches.

---

## 5. Development Workflow
- Execute commands via `Makefile`: `make up`, `make down`, `make test`, `make test-front`, `make lint`, `make lint-front`.
- Live rebuild on frontend changes: Run `make test-front` then `touch main.go`.
