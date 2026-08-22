# Database & Persistence Specification

Technical specification for SQLite storage, transaction lifecycles, and repository contracts.

## Storage Architecture
- Engine: SQLite 3 with Write-Ahead Logging (`WAL`).
- Driver: `github.com/mattn/go-sqlite3` with build tag `sqlite_fts5`.
- Path: `data/plextracker.db` (+ `-wal` and `-shm` sidecars).
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
- `titles`: Core media table. Columns: `id`, `type` (`movie`/`series`), `is_anime`, `year`, `status` (`watching`/`completed`/`dropped`/`plan_to_watch`), `match_status` (`confirmed`/`pending_review`/`unconfirmed`), `series_status` (`returning`/`ended`/`cancelled`/`in_production`), `tmdb_id`, `imdb_id`, `tvdb_id`, `anilist_id`, `simkl_id`, `plex_rating_key`, `radarr_id`, `sonarr_id`, `arr_ignored`, `cover_url`, `overview`, `credits`, `runtime`, `my_rating`, `tmdb_rating`, `tvdb_rating`, `anilist_rating`, `first_watched_at`, `last_watched_at`, `last_refreshed_at`, `next_air_date`, `next_air_episode`, `origin_country`, `total_watch_minutes`, `accent_hex`, `watch_providers`.
- `title_names`: Multilingual and alternative aliases. Columns: `id`, `title_id`, `name`, `language`, `is_primary`, `created_at`.
- `title_genres`: Associated genres per title. Columns: `title_id`, `genre`.
- `seasons`: Seasons of a series. Columns: `id`, `title_id`, `season`, `name`, `overview`, `cover_url`, `episode_count`, `watched_count`, `air_date`, `anilist_score`, `first_watched_at`, `last_watched_at`.
- `season_external_ids`: Per-part external IDs (split-cour anime). Columns: `id`, `season_id`, `provider`, `external_id`, `position`, `created_at`.
- `episodes`: Episodes per season. Columns: `id`, `season_id`, `episode`, `name`, `overview`, `air_date`, `runtime`, `watched`, `first_watched_at`, `last_watched_at`.
- `watch_events`: Granular scrobble history log. Columns: `id`, `title_id`, `episode_id`, `source` (`jellyfin`/`manual`/`simkl`/`legacy_backfill`), `created_at`.
- `task_queue`: Asynchronous background jobs. Columns: `id`, `task_type`, `payload`, `status` (`pending`/`running`/`completed`/`failed`/`dead`), `attempts`, `max_attempts`, `run_at`, `created_at`, `updated_at`, `last_error`, `dedup_key`.
- `match_events`: Audit log for automated actions. Columns: `id`, `title_id`, `event_type` (`auto_confirmed`, `season_attached`), `detail`, `created_at`.
- `dismissed_season_proposals`: Discarded duplicate merge proposals. Columns: `source_title_id`, `target_title_id`, `created_at`.
- `push_subscriptions`: Web Push subscriber credentials. Columns: `id`, `endpoint`, `p256dh`, `auth`, `created_at`.
- `settings`: Key-value configuration store (`radarr_url`, `sonarr_url`, `anilist_token_invalid`, notification preferences, etc.). Columns: `key`, `value`, `updated_at`.
