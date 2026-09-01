# Database & Persistence Specification

[← Back to Index](../INDEX.md)

---

## Storage Architecture
- Engine: SQLite 3 with Write-Ahead Logging (`WAL`).
- Driver: `github.com/mattn/go-sqlite3` with build tag `sqlite_fts5`.
- Path: `data/trackarr.db` (or `data/plextracker.db` on existing installs) (+ `-wal` and `-shm` sidecars).
- Connection Limits: `MaxOpenConns = 1` (strict single-writer serialization).

## Core Architecture Rule: Reader vs Writer Pattern
To enforce compile-time write safety:
1. **Readers** (`internal/repository/*_repository.go`):
   - Implemented as methods on repository structs accepting `database.DBTX` (`*sql.DB` or `*sql.Tx`).
   - Read methods can query either the `writeDB` pool or a separate read pool (`readDB`).
2. **Writers** (`internal/repository/*_writer.go`):
   - Implemented on dedicated writer structs (e.g. `TitleWriter`, `SeasonWriter`, `EpisodeWriter`, `WatchEventWriter`, `TaskWriter`, `MatchEventWriter`).
   - **Constructor accepts only `*sql.Tx`**: `NewTitleWriter(tx *sql.Tx)`.
   - The Go compiler strictly rejects executing mutating SQL statements outside an explicit transaction block.

## Critical Invariant: Nested Transaction Deadlock
Because `MaxOpenConns = 1`, acquiring a new write transaction while already holding a transaction will **permanently deadlock the server**:
- **Rule**: Never call another service or method that invokes `BeginTx` / `WithTxContext` from within an active transaction.
- **Rule**: Post-commit side effects (e.g. backfilling, push notifications, webhooks, queue dispatching) must be returned to the caller and executed **after** `WithTxContext` finishes and commits.

## Tables & Primary Schema Entities
- `titles`: Core media table. Columns: `id`, `type` (`movie`/`series`/`anime`), `is_anime`, `year`, `status` (`watching`/`completed`/`dropped`/`plan_to_watch`), `match_status` (`confirmed`/`pending_review`/`unconfirmed`), `series_status` (`returning`/`ended`/`cancelled`/`in_production`), `tmdb_id`, `imdb_id`, `tvdb_id`, `anilist_id`, `simkl_id`, `external_source_id`, `radarr_id`, `sonarr_id`, `arr_ignored`, `cover_url`, `overview`, `credits`, `runtime`, `my_rating`, `tmdb_rating`, `tvdb_rating`, `anilist_rating`, `first_watched_at`, `last_watched_at`, `last_refreshed_at`, `next_air_date`, `next_air_episode`, `origin_country`, `total_watch_minutes`, `accent_hex`, `watch_providers`, `personal_notes`.
- `title_names`: Multilingual and alternative aliases. Columns: `id`, `title_id`, `name`, `language`, `is_primary`.
- `title_genres`: Associated genres per title. Columns: `title_id`, `genre`.
- `title_relations`: Side stories, sagas, and franchise relations. Columns: `id`, `title_id`, `season_id`, `provider` (`anilist`/`tmdb`/`tvdb`), `external_id`, `relation_type` (`PREQUEL`/`SEQUEL`/`SPIN_OFF`/`SIDE_STORY`/`ALTERNATIVE`/`COLLECTION`), `format`, `title`, `cover_url`, `year`, `score`, `overview`, `sort_order`, `created_at`.
- `seasons`: Seasons of a series. Columns: `id`, `title_id`, `season_number`, `total_episodes`.
- `season_external_ids`: Per-part external IDs (split-cour anime). Primary Key: `(season_id, provider, external_id)`. Columns: `season_id`, `provider` (`anilist`), `external_id`, `anilist_episode_count`, `anilist_start_date`, `anilist_average_score`, `sort_order`, `created_at`, `updated_at`.
- `episodes`: Episodes per season. Columns: `id`, `season_id`, `episode`, `name`, `air_date`, `watched`, `watched_at`, `external_source_id`.
- `watch_events`: Granular scrobble history log. Columns: `id`, `title_id`, `episode_id`, `source` (`plex`/`jellyfin`/`emby`/`manual`/`simkl`/`backfill`), `raw_payload`, `created_at`.
- `task_queue`: Asynchronous background jobs. Columns: `id`, `task_type`, `payload`, `status` (`pending`/`running`/`completed`/`failed`/`dead`), `attempts`, `max_attempts`, `run_at`, `created_at`, `updated_at`, `last_error`, `dedup_key`.
- `match_events`: Audit log for automated actions. Columns: `id`, `title_id`, `kind` (`auto_confirmed`, `season_attached`), `detail`, `created_at`.
- `season_audit_dismissals`: Discarded duplicate merge proposals. Primary Key: `(source_title_id, target_title_id)`. Columns: `source_title_id`, `target_title_id`, `created_at`.
- `wrapped_snapshots`: Immutable annual retrospective snapshots. Primary Key: `year`. Columns: `year`, `data_json`, `created_at`.
- `settings`: Key-value configuration store (`radarr_url`, `sonarr_url`, `prowlarr_url`, `admin_password_hash`, `admin_recovery_key_hash`, `jwt_secret`, `vapid_public_key`, `vapid_private_key`, `push_subscription`, `metadata_language`, `enabled_watch_providers`, `calendar_token`, notification preferences, etc.). Columns: `key`, `value`.

## Title Merge Invariants (`TitleWriter.Merge`)
When merging a `sourceID` title into `destID`:
1. **Metadata & Integrations**: `COALESCE` transfers non-null fields from source (`sonarr_id`, `radarr_id`, `my_rating`, `cover_url`, `overview`, `origin_country`, `watch_providers`, etc.).
2. **Arr Queue**: `arr_ignored = MIN(dest.arr_ignored, source.arr_ignored)` preserves queue membership.
3. **Episode Collision Safety**: Colliding seasons preserve watched status (`watched = 1`, `external_source_id`), re-parent `watch_events.episode_id` to dest episode IDs, move uncollided episodes, and delete redundant source episode rows without history loss.
4. **Genres & Analytics**: `title_genres` are merged (`INSERT OR IGNORE`) and `total_watch_minutes` is recalculated.

