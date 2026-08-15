# Patterns & Codebase Map

Update when adding routes, services, components, or commands.

## Backend (Go)

### CLI Commands

| Command | File | Purpose |
|---|---|---|
| `serve` | `cmd/serve.go` | HTTP server (configurable with `DISABLE_BACKGROUND_TASKS`) |
| `import` | `cmd/import.go` | Simkl backup import |
| `reset-import` | `Makefile` | Calls `db-reset` (deletes .db + -wal + -shm, restarts Docker container so migrations re-run on boot), sleeps 5 s, then `import`. Refuses without `BACKUP_FILE=`. |
| `ssh-reset-import` | `Makefile` | Calls `ssh-db-reset` (rm files inside container + `docker restart plextracker`, migrations re-run on boot), sleeps 20 s, then `ssh-import`. `BACKUP_FILE=` is a filename under `/volume1/downloads`. Refuses without it. |
| `ssh-db-pull` | `Makefile` | Downloads prod DB (`plextracker.db` + wal + shm) from NAS to local `data/` and starts local app. |
| `ssh-logs` | `Makefile` | Dumps prod container logs from NAS into local `data/plextracker.log` (`LINES=...` optional). |
| `ssh-debug-pull` | `Makefile` | Runs `ssh-db-pull` then `ssh-logs` to prepare local environment for offline prod debugging. |

### Models

`internal/model/` — Title (TitleType, TitleStatus, SeriesStatus, MatchStatus, NextEpisode; `total_watch_minutes`; `watch_providers` TEXT — JSON `[{id,name}]` FR subscription-included providers, NULL=never fetched/`[]`=none, exposed on `Title`/`ContinueWatchingItem`/`UpcomingItem`), TitleName, Season (EpisodeCount, WatchedCount), Episode, WatchEvent (WatchEventSource), Setting.

### Services

| Service | File | Purpose |
|---|---|---|
| JellyfinService | `internal/service/jellyfin.go` | Webhook scrobble ingest, delegates to TitleService + LibraryService. `source=jellyfin` labels WatchEvents. Only `PlaybackStop`+`PlayedToCompletion` ingested (→ scrobble); else ignored. Movies carry provider IDs; episodes match via `SeriesID`+name/year |
| TitleService | `internal/service/title.go` | Title logic (creation, rematching, URL resolution, merging) |
| LibraryService | `internal/service/library.go` | User library (marking watched, auto-complete, rating, notifications) |
| BackfillService | `internal/service/backfill.go` | Episode backfill (metadata fetch, mark previous). **Opens own writeDB tx — never call from inside another tx; fire post-commit via `LibraryService.TriggerBackfillForEpisode`.** |
| AniListPushService | `internal/service/anilist_push.go` | Per-part (each AniList entry mapped to a season) / per-movie push of status, progress, score. `PushSeasonState` splits watched episodes across parts by derived range and pushes each. Silently skips on missing token, no parts, or 401 (flags token invalid). Enqueued via `TaskTypeAniListPushSeason` / `TaskTypeAniListPushMovie`. |
| CoverService | `internal/service/cover.go` | Owns cover image lifecycle: fetches from TMDB/TVDB/AniList with deadlines (a stalled CDN can't freeze the writeDB), persists filename via `TitleUpdate`, drives accent extraction (`colorextract/`). Shares the 2 rps `APILimiter` budget with TaskQueueWorker + BackgroundService. |
| APILimiter | `internal/service/ratelimiter.go` | Global 2 rps token bucket guarding TMDB/TVDB/AniList HTTP calls across all background workers. |
| PushNotifier | `internal/service/push.go` | Web Push VAPID (interface: PushService + noopNotifier) |
| BackgroundService | `internal/service/background.go` | Daily refresh (TVDB sync for series when `tvdb_id` present/backfilled from TMDB external IDs → `refreshSeriesFromTVDB`, TMDB fallback, auto-complete, push triggers, per-part AniList meta — score/episode_count/start_date — via `ListPartsForTitle` → `UpdatePartMeta`; 401 flags `anilist_token_invalid` and aborts remaining calls). Name backfill: `syncTitleNames` → `AddMissingNames` inserts en/fr translations from TMDB (`GetTitleNames`) + TVDB (`details.Names()`) on refresh, additive (never deletes → anime romaji & merged-season aliases survive). AniList-only titles (no TMDB) still `ReplaceNames` (en+romaji) in `refreshFromAniList`. Watch providers: `refreshMovieFromTMDB`/`refreshSeriesFromTMDB` populate `watch_providers` via `matching.ExtractFlatrateProvidersFR`, which reads `watch/providers` data piggybacked on the existing detail `append_to_response`; parsed by `parseWatchProviders` (repository/title.go). Origin country: `refreshMovieFromTMDB`/`refreshSeriesFromTMDB` populate `titles.origin_country` (ISO-3166-1 alpha-2, uppercase) from TMDB `origin_country` via `matching.ExtractOriginCountry`; `refreshFromAniList` populates it from AniList `countryOfOrigin`. NULL = never determined; existing library backfilled by running `POST /api/admin/refresh-all`. |
| SimklImporter | `internal/service/simkl.go` | Simkl backup import (zip/JSON) |
| Pipeline | `internal/service/matching/pipeline.go` | Orchestrates matching Steps 1-5. URL resolution (TMDB, IMDb, AniList, TVDB slugs) |
| TMDBClient | `internal/service/matching/tmdb*.go` | TMDB API: search, details, covers, find-by-id |
| TVDBClient | `internal/service/matching/tvdb*.go` | TVDB v4: details, covers, search, official series episodes (`/series/{id}/episodes/official`), slug resolution (JWT auth) |
| AniListClient | `internal/service/matching/anilist*.go` | AniList GraphQL: search, episodes, rating sync, covers |
| CrossRefDB | `internal/service/matching/crossref.go` | anime-offline-database ID cross-referencing |
| GeminiClient | `internal/service/matching/gemini.go` | AI match verification, fuzzy resolve, anime season ID, key rotation |

### Matching Pipeline Steps

1. **Plex IDs** — TMDB/IMDB from webhook metadata → `confirmed`
2. **Cross-reference** — anime-offline-database lookup → `confirmed`
3. **TMDB search** — by title+year → Step 5
4. **AniList search** — anime only → Step 5
5. **Gemini verification** — `verifyAndEnrich` in `pipeline.go`:
   - Gemini present: `Confirmed && Confidence==High` → `confirmed`; all other outcomes (medium, low, not-confirmed, HTTP error) → `unconfirmed`.
   - Gemini absent (`p.gemini == nil`) → `pending_review`.

**Auto-confirm rule (migration 026+):** When Gemini returns `ConfidenceHigh` AND `MatchSource` is a search strategy (`tmdb_search`, `anilist_search`, `gemini_fuzzy`) AND `PreserveMatch` is false → `match_status = confirmed` directly (skips `pending_review`). A `match_events` row with kind `auto_confirmed` is written in the same tx. `isSearchSource()` in `taskqueue.go` encodes which sources qualify.

Confidence: `ConfidenceHigh`/`Medium`/`Low` (constants in `pipeline.go`). Nil clients are skipped (graceful degradation). Each step sets `MatchSource` (`plex_ids`, `crossref`, `tmdb_search`, `anilist_search`, `gemini_fuzzy`, `none`).

After matching: parallel TMDB + TVDB fetch → fusion (overview: longest; genres: union; cover: TMDB > TVDB > AniList). TVDB URL resolution via `ParseURLFull()` → slug → numeric ID.

### AniList Season Chain

`AniListClient.ResolveSeasonChain(ctx, id)` in `internal/service/matching/anilist_relations.go` walks PREQUEL edges to the chain root and returns `SeasonChain{RootID, RootTitle, SeasonNumber, IsRoot, RootIsSeries}`.

Rules:
- TV and ONA formats count as season carriers (increment ordinal); movies/OVA/specials are traversed without incrementing.
- Non-TV/ONA entry → `IsRoot=true, SeasonNumber=1, RootIsSeries=false` immediately.
- `pickPrequel`: prefers TV over ONA over any other ANIME PREQUEL edge.
- Cycle guard: errors on revisited node. Depth guard: errors at `maxChainDepth=25`.

Pipeline wrapper: `Pipeline.ResolveAniListSeason(ctx, anilistID)` (called from `resolveAnimeSeason` in `taskqueue.go`).

### Season Auto-Attach (`decideSeasonAction`)

Pure fn in `taskqueue.go`; called from `resolveAnimeSeason` after `ResolveSeasonChain`. No DB/HTTP — unit-testable.

| Input state | Decision |
|---|---|
| `chain == nil` (resolve failed) | `legacy` — fall back to IMDb-collision path |
| `chain.IsRoot` | `legacyRoot` — offset 0 (season 1 treatment) |
| `!chain.RootIsSeries` | `none` — never attach TV season to movie/special root |
| `parentByIDs != nil` (shared own IMDb/TMDB id) | `mergeInto parentByIDs` at `SeasonNumber-1` offset |
| `parentByRoot != nil` AND entry has own IMDb/TMDB/TVDB identity | `none` — distinct franchise member (e.g. Dragon Ball Z) |
| `parentByRoot != nil` AND no own identity | `mergeInto parentByRoot` at offset |
| No parent found AND entry has own identity | `none` |
| No parent found AND no own identity | `createRoot` (create the parent series, then merge) |

On accepted merge: writes `match_events` row with kind `season_attached` on the surviving parent title.

### match_events

Migration 026. Table: `match_events(id PK, title_id INTEGER nullable FK→titles CASCADE, kind TEXT, detail TEXT, created_at)`. `title_id` has no NOT NULL constraint — the model field is `*int64`; it becomes NULL when the referenced title is deleted (CASCADE sets it). Index on `created_at DESC`.

Kinds (`model.MatchEventKind`):
- `auto_confirmed` — written in `handleEnrichment` when a search-source match lands `confirmed` (not `PreserveMatch`).
- `season_attached` — written in `mergeSeasonInto` on successful auto-attach; `title_id` points to the surviving parent.

Writer: `repository.NewMatchEventWriter(tx).Create(ctx, titleID, kind, detail)`.
Reader: `repository.MatchEventRepository.ListRecent(ctx, limit)` — returns events with `cover_url` join.

**`year`/`release_date` backfilled ONLY by enrichment, NOT by refresh.** `buildEnrichmentUpdate` (`taskqueue.go`) writes `release_date` from the pipeline result and derives `year` via `releaseYear(result.ReleaseDate)` (v0.34.1). The refresh path (`refreshMovieFromTMDB`/`refreshSeriesFromTMDB`, `background.go`) refreshes cover/overview/genres/rating/status but **never** touches `year` or `release_date`. Consequence: a `year=0` title is repaired by a **rematch** (`POST /titles/{id}/rematch`, which enqueues enrichment) — a plain refresh will not fix it. Re-supplying the existing `tmdb_id` to rematch is enough to re-enrich, but it also sets `match_status=confirmed` + `match_source=manual`.

**Seasons NOT created by enrichment.** Created only by refresh (`TaskTypeRefresh` → `refreshSeriesFromTMDB`, needs `TMDBID`) or watched-episode (`SeasonWriter.GetOrCreate`, `plex.go`). Consequences:
- Just-matched series has 0 seasons until refresh/watch. API omits the field (`Seasons json:"seasons,omitempty"` → `undefined` client-side) — front-end must guard (e.g. `title.seasons ?? []`).
- `handleEnrichment.enqueueSeasonBackfill` enqueues `refresh:<id>` for each matched non-movie with a TMDB ID → seasons appear immediately, not next cron.
- Periodic `RefreshTitles` cron backfills existing titles regardless of `match_status`. Skips completed/dropped titles **except** those still missing a TMDB-synced episode list (`needsEpisodeBackfill`: non-movie + has `tmdb_id` + no season with `total_episodes`) — a Simkl-imported "completed" series or one only ever touched by scrobbles. Those get a one-time list backfill, then `total_episodes` flips the predicate and they're skipped again. After a refresh, `refreshTitle` Step 2b enforces **completed ⟹ every episode watched** (`completeEpisodes` → `EpisodeWriter.MarkAllWatchedForTitle` + `TitleWriter.AddWatchMinutesForEpisodes`); idempotent, writes no `watch_events` (keeps activity feed / streaks honest while still counting watchtime via `total_watch_minutes`). `dropped` titles get the list backfill but keep their real watched flags.

### AniList per-season push

**Data model — multiple parts per season.** A season can map to several AniList entries ("parts" — split-cour seasons like AoT Final Season Part 1 + Part 2). `season_external_ids(season_id, provider, external_id, anilist_episode_count, anilist_start_date, anilist_average_score, sort_order)` with PK `(season_id, provider, external_id)` (migration 027; was `(season_id, provider)` in 020). Each row is one part; dedup is by `external_id`. The per-part `anilist_*` meta columns are populated by the daily refresh; `sort_order` is a manual override. **Ordering is always** `ORDER BY (sort_order IS NULL), sort_order, (anilist_start_date IS NULL), anilist_start_date, external_id` — explicit order first, then AniList start date (NULL-last), `external_id` tiebreak. The detail API exposes `season.anilist_parts[]`; the legacy `season.anilist_id`/`anilist_community_score` remain as **derived primary-part aliases** (first part) for backward compat. `seasons.anilist_average_score` column survives but is no longer read/written (score lives on the part rows). `titles.anilist_id` is retained for movies and title-level display.

Accessed via `SeasonExternalIDRepository` (`Get` → primary part, `ListParts`, `ListPartsForTitle`, `Add`, `DeletePart`, `Reorder`, `UpdatePartMeta`) and `SeasonExternalIDWriter` (`Stamp`/`Add` — append a part, `ON CONFLICT(season_id,provider,external_id) DO NOTHING`; `Delete` all parts; `DeletePart` one). Merge appends incoming parts (a split-cour merge keeps both entries).

**Push service.** `service.AniListPushService` exposes `PushSeasonState(ctx, seasonID)` and `PushMovieState(ctx, titleID)`. `PushSeasonState` derives a state **per part** and pushes each independently; skips silently when no parts / no token / token flagged invalid, absorbs HTTP 401 by setting `settings.anilist_token_invalid = 'true'`.

**Per-part range derivation** (`service.DerivePartStates(titleStatus, parts, watched, seasonTotal)`): walking parts in sort order, part *i* covers season episodes `(cum, cum+anilist_episode_count]`; the last part / any part with an unknown count absorbs the remainder (no watched episode dropped). When a part's AniList count is unknown (e.g. freshly linked, refresh not yet run), the last/only part's effective count falls back to `seasonTotal − cum` so a fully-watched single-part season still reaches `COMPLETED`. Each part's status uses `service.derivePartState` (same rules as the old `DeriveSeasonState`, scoped to the part's range): all-in-range watched → `COMPLETED` (wins over dropped); dropped + unfinished → `DROPPED`; partial → `CURRENT`; zero + plan_to_watch → `PLANNING`. `DeriveSeasonState` is retained (still used by `TitleHandler.Patch`).

**Rating guard** (`service.ShouldPushRating`): score included only when a part's derived status is `COMPLETED` or `DROPPED`.

**Triggers** (all via task queue `anilist_push_season` / `anilist_push_movie`):
- Episode watched/unwatched → `LibraryService.MarkEpisodeWatched` enqueues.
- Title status change → `TitleHandler.Patch` enqueues for every season.
- Title rating change → `TitleHandler.Patch` enqueues for seasons whose derived status is `COMPLETED`/`DROPPED`.
- Season AniList part added via `POST /titles/{titleID}/seasons/{seasonID}/anilist` → handler enqueues immediately.

**Token expiry.** `settings.anilist_token_invalid = 'true'` suppresses further pushes. The Settings screen surfaces a "Reconnect AniList" banner. Successful OAuth reconnect clears the flag (`anilist_auth.SaveToken` calls `settings.Delete("anilist_token_invalid")`).

**UI (Concept B).** Active-season info strip between the progress line and the episode grid. Component: `frontend/src/components/SeasonAniListStrip.tsx` — single part renders as before (`S{n}` link + score); 2+ parts render a list of "Part N · View on AniList · score" rows. Per-season management via the pencil ✎ (or "Link entry" CTA) opens `RematchSheet` with `seasonID` context: a multi-link manager listing parts with ▲/▼ reorder + Remove, plus an Add-by-ID input. Add → `POST .../anilist`; remove → `DELETE .../anilist/{externalID}`; reorder → `PUT .../anilist/order` `{ordered_ids}`. The sheet stays open (`onDone`, not `onClose`) for managing several parts.

### Season Audit Service

`internal/service/seasonaudit.go` — `SeasonAuditService.Scan(ctx)` finds confirmed series that share an external id (IMDb/TMDB/TVDB/AniList), groups them into connected components, then proposes merges named via AniList relations (or fallback heuristics). Returns `[]SeasonAuditProposal`. Dismissed pairs (persisted in `season_audit_dismissals`) are excluded.

`Accept(ctx, sourceTitleID, targetTitleID, seasonNumber)` — executes the merge (source → target at given season offset). Never automatic; always user-initiated.

`Dismiss(ctx, sourceTitleID, targetTitleID)` — inserts a `season_audit_dismissals` row; pair never re-proposed.

### Simkl Provenance

Migration 026 adds `titles.simkl_id INTEGER` and `titles.simkl_slug TEXT`. `SimklImporter` (`internal/service/simkl.go`) populates these from `ids.simkl` / `ids.slug` on every imported title. `MatchReviewCard.tsx` uses them to build outbound Simkl links (`https://simkl.com/<section>/<id>/<slug>`). Also renders IMDb, TMDB, and AniList outbound links from their respective title fields. AniList id now flows through `SimklIDs` → `EnrichmentPayload.AniListID` → `MatchInput.AniListID` so it's available to the pipeline on first enrichment.

### Repositories

`internal/repository/` — Read methods accept `database.DBTX` (pool or tx). Write methods live on typed writer structs (e.g. `TitleWriter`, `SeasonWriter`, `EpisodeWriter`, `WatchEventWriter`) that take `*sql.Tx` in their constructor; the compiler rejects any attempt to write outside a transaction. Callers wrap in `database.WithTxContext(ctx, pool, func(tx *sql.Tx) error { ... })` and build writers from `tx`.

Test writes go through `internal/testutil` helpers (`CreateTitle`, `UpdateTitle`, `MergeTitles`, `GetOrCreateSeason`, `GetOrCreateEpisode`, `CreateWatchEvent`, …) which wrap the matching writer in a short tx — no test needs to repeat the boilerplate.

| Repository | Reader (DBTX) | Writer (*sql.Tx, ctx) |
|---|---|---|
| Title | `List`, `ListAll`, `GetByID`, `FindByExternalID`, `ListOriginCountries` (distinct `origin_country` + count, excludes NULL/empty, ordered by count desc then code asc — mirrors Genre's `ListWithCounts`), search in `title_search.go` | `Create`, `Update`, `UpdateLastWatchedAt`, `ReplaceNames`, `AddMissingNames`, `Merge`, `Delete`, `BatchDelete`, `BatchUpdateStatus` |
| Season | `GetByID` | `GetOrCreate`, `UpdateRating`, `UpdateTotalEpisodes`, `Upsert` |
| Episode | `GetBySeasonID` | `GetOrCreate`, `ToggleWatched`, `BatchMarkWatched`, `UpdateMetadata`, `UpsertBatch`, `MarkWatched`, `UpdateLastWatchedAt` |
| WatchEvent | `CountByTitleID`, `ListByTitle` | `Create`, `BatchCreate` |
| Task | `GetByID`, `ListPending`, `ListDead`, `ListPaginated`, `CountByStatus`, `CountDead` | `Enqueue`, `EnqueueWithDelay`, `FetchDue`, `Complete`, `Fail`, `RetryDead`, `ResetRunning`, `Delete`, `DeleteBatch`. Task kinds: `TaskTypeEnrichment`, `TaskTypeRefresh`, `TaskTypeCoverFetch`, `TaskTypeAniListPushSeason`, `TaskTypeAniListPushMovie`. |
| SeasonExternalID | `Get` (primary part), `ListParts`, `ListPartsForTitle` | pool (`SeasonExternalIDRepository`): `Add` (append part, dedup by id), `DeletePart` (one), `Delete` (all), `Reorder` (`sort_order` by index), `UpdatePartMeta` (score/episode_count/start_date). tx (`SeasonExternalIDWriter`): `Stamp`/`Add` append (`ON CONFLICT(season_id,provider,external_id) DO NOTHING`) — entry point for merge (`TitleWriter.Merge`, keeps both parts) and S1 backfill (`stampSeasonAniListID`); `Delete`/`DeletePart`. Multiple parts per season (split-cour). Provider key: `repository.ProviderAniList`. Migration 027 (PK `(season_id,provider,external_id)` + per-part meta cols). |
| Genre | `ListWithCounts` | `ReplaceForTitle` |
| Setting | `Get` | `Set`, `Delete` |
| MatchEvent | `ListRecent(ctx, limit)` — returns events with cover_url join | `Create(ctx, titleID, kind, detail)` on `*sql.Tx` via `MatchEventWriter` |
| SeasonAuditDismissal | — | `Dismiss(ctx, src, tgt)` direct pool write (`season_audit.go`) |
| StatsRepository | `TotalWatchMinutes`, `TopGenres`, `CurrentStreak`, `BestStreak` |
| ActivityRepository | `List` (paginated watch events) |
| HistoryRepository | `GetByTitleID` (per-title watch log) |

TitleFilter: Limit/Offset/UpToDate/WatchingBehind/SeriesStatus/Sort/Order/Genres/GenreOp/OriginCountries/MyRatingMin/TMDBRatingMin. Genres in `title_genres` join table (migration 016), loaded separately (MaxOpenConns=1). `OriginCountries []string` → OR'd `origin_country IN (...)`; `MyRatingMin *int` → `my_rating >= ?`; `TMDBRatingMin *float64` → `tmdb_rating >= ?`. Applied in `List` and `searchTitles`, NOT `fuzzySearch` (same precedent as genre/release filters). `titles.origin_country` (migration 031) is a nullable ISO-3166-1 alpha-2 uppercase column. `series_status` values: `returning`/`ended`/`cancelled`/`in_production` (fixed by migration-001 inline CHECK — SQLite can't ALTER it without rebuilding `titles`, so the value stays `in_production`). `in_production` is the "announced but not yet aired" bucket, surfaced as the **Not started** filter chip. TMDB→value in `mapTMDBSeriesStatus` (background.go, sole write path — refresh/scrobble/library): `Ended`→ended, `Canceled`→cancelled, `Returning Series`→returning, `In Production`/`Planned`/`Pilot`→in_production, else NULL. NULL rows backfill via `POST /admin/refresh-all`.

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
| POST | `/api/webhook/jellyfin/{secret}` | HandleJellyfin | No (secret in URL) |
| GET | `/api/covers/{filename}` | Serve | No |
| GET | `/api/titles` | List | Yes |
| GET | `/api/genres` | List | Yes |
| GET | `/api/countries` | Countries | Yes |
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
| POST | `/api/titles/{titleID}/seasons/{seasonID}/anilist` | AddAniListID | Yes |
| DELETE | `/api/titles/{titleID}/seasons/{seasonID}/anilist/{externalID}` | RemoveAniListID | Yes |
| PUT | `/api/titles/{titleID}/seasons/{seasonID}/anilist/order` | ReorderAniList | Yes |
| POST | `/api/push/subscribe` | Subscribe | Yes |
| DELETE | `/api/push/subscribe` | Unsubscribe | Yes |
| GET | `/api/stats` | Get | Yes |
| GET | `/api/stats/activity` | List | Yes |
| GET | `/api/titles/{id}/history` | Get | Yes |
| GET | `/api/settings` | Get | Yes |
| GET | `/api/anilist/auth` | Authorize | Yes |
| POST | `/api/anilist/token` | SaveToken | Yes |
| DELETE | `/api/anilist/token` | Disconnect | Yes |
| GET | `/api/match-events` | List (default 30, cap 100 via `?limit=N`) → `{"events":[{id,title_id,kind,detail,created_at,cover_url}]}` | Yes |
| GET | `/api/admin/season-audit` | List → `{"proposals":[...]}` | Yes |
| POST | `/api/admin/season-audit/accept` | Accept `{source_title_id,target_title_id,season_number}` | Yes |
| POST | `/api/admin/season-audit/dismiss` | Dismiss `{source_title_id,target_title_id}` | Yes |
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

`GET /api/titles` extra query params: `origin_country` (repeated), `my_rating_min` (int 1-10), `tmdb_rating_min` (float 0-10). Invalid/out-of-range values silently ignored.

## Frontend (Preact)

Design tokens: `frontend/src/theme.ts` (JS) + `frontend/src/tokens.css` (CSS custom properties). CSS Modules for all components. `clsx` for conditional classes. API client in `frontend/src/api.ts`. Types in `frontend/src/types.ts`.

Shared utilities split between `frontend/src/utils.ts` (formatters, name resolvers, AniList URL helpers — `getName`, `getAlternativeNames`, `languageLabel`, `formatDate`, `formatWatchtime`, `computeAniListUrl`, `hexToRgba`…) and the typed `frontend/src/utils/` subdirectory:

`getName` → single best display name (fr→en→romaji/ja→first). `getAlternativeNames` → dedup'd `title.names` minus the shown name and `original_title`, sorted by language; rendered as TitleDetail's "Autres titres" section. `languageLabel(code)` → `{flag, label}` for a name's language.

| Module | Purpose |
|---|---|
| `utils/badge.ts` | PWA app icon badge: `updateBadge()` reads `/api/titles/review-count` and calls `navigator.setAppBadge`. User toggle persisted in `localStorage` (`badge-enabled`). Called from `app.tsx` on auth + `MatchReview.tsx` after each action. |
| `utils/haptic.ts` | `navigator.vibrate` wrapper with `HAPTIC_SHORT` / `HAPTIC_MEDIUM` / `HAPTIC_LONG` patterns. |
| `utils/episodeRanges.ts` | Groups consecutive watched episodes into ranges (e.g. `S1 E1–4`) for the activity feed and per-title history. Folds duplicate episode_numbers (rewatches, dual webhook firings) into the current group. |
| `utils/providers.ts` | `PRIME_PROVIDER_IDS = {9, 119}` (Amazon Prime FR). `isOnPrime(providers)` (takes a title's `watch_providers` list) returns true when it contains any PRIME id (excludes rent/buy id 10). |
| `lib/country.ts` | `countryFlag`/`countryName`/`countryLabel` for an ISO-3166-1 alpha-2 code, via native `Intl.DisplayNames` (no library dependency). Backs the FilterDrawer country chips and `CountryCount` (`types.ts`) options from `/api/countries`. |

### API & routing conventions (read before adding any URL literal)

**`apiFetch(path)` and `useApi(path)` automatically prepend `BASE = '/api'`.** The `path` argument MUST start with `/` and MUST NOT start with `/api` — otherwise the URL becomes `/api/api/...` and 404s silently. The `apiFetch` signature enforces this at compile time for string literals (`P extends \`/api${string}\` ? never : P`) and at runtime via a `console.error` in dev. Same for `useApi`.

**Three legitimate raw `fetch('/api/...')` exceptions** (must keep the prefix because they bypass `apiFetch`'s 401-redirect):
- `app.tsx` and `Login.tsx` — `/api/config` (pre-auth bootstrap)
- `ErrorBoundary.tsx` — `/api/client-errors` (must report even if app is broken)
- `AdminAniList.tsx` — `/api/anilist/auth` full-page navigation (OAuth init)

**SPA routes are singular, API routes are plural.** TitleDetail is `/title/:id`, the API is `/api/titles/:id`. Same for any future resource. SPA route literals MUST go through `ROUTE_PATHS` / `routeTo` in `frontend/src/routes.ts` — never inline a path string. `routeTo.title(id)` builds `/title/${id}`, `ROUTE_PATHS.title` is `/title/:id` for `<Route path="...">` registration.

### Hooks

| Hook | File | Purpose |
|---|---|---|
| `useApi` | `hooks/useApi.ts` | Fetch wrapper with loading/error/mutate |
| `useTitleStore` | `store.ts` | Zustand: paginated title fetch, filter, sort (localStorage), loadMore, cache. `TitleFilter` fields: `origin_country?: string[]`, `my_rating_min?: string`, `tmdb_rating_min?: string`. Serialized via shared `appendFilterParams` helper (also used by both search-store param builders) — add new filter params there, not per-callsite. |
| `useSearchStore` | `store.ts` | Zustand: search query/results/scroll, TMDB toggle |
| `useServiceWorker` | `hooks/useServiceWorker.ts` | Registers `/sw.js` once authenticated (gates on `isAuthed` so unauthenticated visits don't install the SW). |
| `usePush` | `hooks/usePush.ts` | Push subscription via the SW registration (VAPID) |
| `useLongPress` | `hooks/useLongPress.ts` | Long-press detection (pointer events, configurable threshold/tolerance) |

### Components

| Component | File | Purpose |
|---|---|---|
| TitleHistory | `components/TitleHistory.tsx` | Watch history overlay per title (uses `episodeRanges` to fold consecutive episodes) |
| Navbar | `components/Navbar.tsx` | 4-tab bottom nav |
| SearchBar | `components/SearchBar.tsx` | Sticky bottom search input bound to `useSearchStore` (auto-focus, optional TMDB toggle). Mounted in `app.tsx` only on `/search` (gated by `isSearch === pathname === '/search'`). The merge flow hides the TMDB toggle (`showTMDBToggle={!mergeSourceId}`). |
| FilterDrawer | `components/FilterDrawer.tsx` | Collapsible filter drawer (sort/status/type/series status/release date/genres/country/rating). Status chips are the canonical status filter (no separate tab strip): All, Plan, Watching, Caught up, Completed, Dropped. Country: multi-select chips (OR semantics), options from `/api/countries`, flag+name via `lib/country.ts`. Rating: two rating-minimum `<select>`s (My rating, TMDB) — each ANDs with other filters; when both set, a title must clear BOTH. Series status chips (All/Returning/Ended/Cancelled/Not started) show only when Type=Series. Collapsed view surfaces active filter chips. |
| TitleCard | `components/TitleCard.tsx` | Horizontal card with progress + quick mark badge. Stamps a `TypeBadge` (size `sm`). |
| PosterCard | `components/PosterCard.tsx` | Poster grid card (2:3, gradient overlay). Stamps a `TypeBadge` overlay. |
| TypeBadge | `components/TypeBadge.tsx` | Movie/series glyph (with `typeIcons.tsx` config). Used by `PosterCard` and `TitleCard` to distinguish the two at a glance in lists. Accepts `radarrId` / `sonarrId` to display a top border accent line (Radarr yellow `#ffc230` for movies, Sonarr cyan `#00c0ff` for series) when present in Arr. |
| SectionRow | `components/SectionRow.tsx` | Library "rich row" — label + subtext + 3-poster peek, used for `// COMING UP` and `// CONTINUE WATCHING` shortcuts above the title grid. |
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
| PosterStrip | `components/PosterStrip.tsx` | Horizontal scrollable poster strip (delegates card to `PosterTile`) |
| PosterTile | `components/PosterTile.tsx` | Compact poster card with optional progress bar / badge / sublabel — used in strips and preset library grids |
| PrimeBadge | `components/PrimeBadge.tsx` | Blue "prime" badge rendered on `PosterTile` (Continue Watching / Coming Up) and `TitleDetail` when `isOnPrime(title)` is true. Lazy: appears only after the title's next TMDB refresh. |
| SeasonAniListStrip | `components/SeasonAniListStrip.tsx` | Active-season AniList info strip (community score, link, fix-match pencil) |
| ConfirmationDrawer | `components/ConfirmationDrawer.tsx` | Confirm/cancel drawer for destructive bulk actions |
| ErrorBanner | `components/ErrorBanner.tsx` | Inline error banner with optional retry |
| ErrorBoundary | `components/ErrorBoundary.tsx` | App-level React error boundary |

### Pages & Routes

| Route | Page | File |
|---|---|---|
| `/` | Library | `pages/Library.tsx` |
| `/coming-up` | ComingUp | `pages/ComingUp.tsx` (preset library: upcoming titles, full grid) |
| `/continue-watching` | ContinueWatching | `pages/ContinueWatching.tsx` (preset library: in-progress titles, full grid) |
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
| `/admin/season-audit` | AdminSeasonAudit | `pages/AdminSeasonAudit.tsx` |
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

### Dependabot
`.github/dependabot.yml` (weekly grouped version updates, immediate security PRs) + `.github/workflows/dependabot-*` (auto-merge, CI fix trigger).


