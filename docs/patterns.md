# Patterns & Codebase Map

Update when adding routes, services, components, or commands.

## Status: T33 complete (Phase 9 — refactoring & UX)

## Backend (Go)

### CLI Commands

| Command | File | Purpose |
|---|---|---|
| `serve` | `cmd/serve.go` | HTTP server (configurable with `DISABLE_BACKGROUND_TASKS`) |
| `import` | `cmd/import.go` | Simkl backup import |

### Models

`internal/model/` — Title (TitleType, TitleStatus, SeriesStatus, MatchStatus, NextEpisode; `total_watch_minutes int` tracks cumulative watch time), TitleName, Season (EpisodeCount, WatchedCount for listing), Episode, WatchEvent (WatchEventSource), Setting.

### Services

| Service | File | Purpose |
|---|---|---|
| PlexService | `internal/service/plex.go` | Webhook scrobble processing, delegates to `TitleService` and `LibraryService` |
| TitleService | `internal/service/title.go` | Centralizes title logic (creation from Plex, rematching, URL resolution, manual merging) |
| LibraryService | `internal/service/library.go` | User library logic (marking watched, auto-complete, rating, notifications). Orchestrates `BackfillService`. |
| BackfillService | `internal/service/backfill.go` | Automates episode backfill (fetching metadata, marking previous episodes) |
| PushNotifier | `internal/service/push.go` | Interface (PushService + noopNotifier). Web Push VAPID |
| BackgroundService | `internal/service/background.go` | Daily title refresh (TMDB sync, auto-complete, push triggers) |
| SimklImporter | `internal/service/simkl.go` | Simkl backup import (zip/JSON) |
| Pipeline | `internal/service/matching/pipeline.go` | Orchestrates Steps 1-5 of media matching. Supports URL resolution (TMDB, IMDb, AniList, TVDB slugs). Parallel TMDB+TVDB fetch with fusion rules in `enrichFromIDs`. |
| TMDBClient | `internal/service/matching/tmdb*.go` | TMDB API: client (tmdb.go), search, details, covers, find-by-id |
| TVDBClient | `internal/service/matching/tvdb*.go` | TheTVDB v4 API: client (tvdb.go, JWT auth with auto-relogin), details, covers, search, slug resolution. Injected via `Pipeline.SetTVDB()`. |
| AniListClient | `internal/service/matching/anilist*.go` | AniList GraphQL: client (anilist.go), search, sync, covers |
| CrossRefDB | `internal/service/matching/crossref.go` | anime-offline-database ID cross-referencing |
| GeminiClient | `internal/service/matching/gemini.go` | Gemini AI match verification + fuzzy resolve + anime season identification, key rotation |

### Matching Pipeline Steps

1. **Plex IDs** — use TMDB/IMDB from webhook metadata → `confirmed`
2. **Cross-reference** — anime-offline-database lookup → `confirmed` (now handles multiple TMDB IDs for movies vs TV)
3. **TMDB search** — by title+year → proceed to Step 5

4. **AniList search** — anime only → proceed to Step 5
5. **Gemini verification** — high confidence → `pending_review`, low → `unconfirmed`

Confidence levels: `ConfidenceHigh`, `ConfidenceMedium`, `ConfidenceLow` (constants in `pipeline.go`). Graceful degradation: nil clients are skipped, pipeline falls through to next step.

Each step sets `MatchSource` on the result (`plex_ids`, `crossref`, `tmdb_search`, `anilist_search`, `gemini_fuzzy`, `none`). Stored on Title alongside `OriginalTitle` (raw Plex name) for Match Review provenance display.

After matching: parallel TMDB + TVDB fetch via `sync.WaitGroup` goroutines → fusion rules (overview: longest wins; genres: union; cover: TMDB first, TVDB fallback, AniList last). Names merged from both sources (TMDB wins on duplicate lang). TVDB covers stored as `tvdb_<id>.jpg`. AniList covers prefixed `al-`.

**TVDB fusion fields**: `tvdb_rating` (score×10, stored as INTEGER), `tvdb_id` already existed. No divergence UI in v1 (TMDB wins on release date conflicts >1 year; server logs the diff).

**TVDB URL resolution**: `ParseURLFull()` returns `TVDBSeriesSlug`/`TVDBMovieSlug` from `thetvdb.com/series/<slug>` or `/movies/<slug>`. `Pipeline.ResolveURL` calls `GetSeriesBySlug`/`GetMovieBySlug` → numeric ID → `Run()`. Graceful: if TVDB client nil, returns user-visible error.

### External APIs

| API | File | Purpose |
|---|---|---|
| TMDB | `internal/service/matching/tmdb.go` | Metadata, episodes, covers, names |
| TVDB | `internal/service/matching/tvdb.go` | Metadata, covers, slug resolution, JWT auth |
| AniList | `internal/service/matching/anilist.go` | Anime search, episodes, rating sync |
| Gemini | `internal/service/matching/gemini.go` | Match verification/fallback |
| anime-offline-database | `internal/service/matching/crossref.go` | Cross-reference ID mapping |
| Google OAuth | `internal/handler/auth.go` | Auth |
| Web Push | `internal/service/push.go` | VAPID notifications |

### Database

`internal/database/` — `Open()`, `Migrate()`, `WithTx(db, fn)` transaction helper, `DBTX` interface (shared by `*sql.DB` and `*sql.Tx`). SQLite with WAL, `MaxOpenConns=1`.

### Repositories

`internal/repository/` — All repos use `database.DBTX` interface (works with `*sql.DB` and `*sql.Tx`). TitleRepository (PaginatedResult, TitleFilter with Limit/Offset/UpToDate/WatchingBehind/SeriesStatus/Sort/Order), SeasonRepository, EpisodeRepository, WatchEventRepository (`CountByTitleID(titleID) (int, error)`), SettingRepository, StatsRepository. All DB queries live here. Title search in `title_search.go`. `List()` returns paginated light response (no episodes, season counters + next_episode). `ListAll()` returns full data for background jobs. `GetByID()` returns full detail with episodes.

### Handlers

`internal/handler/` — auth, title, episode, season, cover, webhook, push, anilist_auth, settings, stats, tmdb, spa. DI via struct with repos. `internal/handler/httputil/` — WriteJSON, ReadJSON, ParseIDParam, ParseQueryInt, APIError, HandlerFunc (`func(w,r) error`), WrapHandler.

### Routes

| Method | Path | Handler | Auth |
|---|---|---|---|
| GET | `/api/health` | Health | No |
| GET | `/api/config` | PublicConfig | No |
| POST | `/api/auth/google` | GoogleCallback | No (rate-limited) |
| POST | `/api/auth/dev` | DevLogin | No (debug only, rate-limited) |
| POST | `/api/auth/logout` | Logout | No |
| POST | `/api/webhook/plex/{secret}` | HandlePlex | No (secret in URL) |
| GET | `/api/covers/{filename}` | Serve | No |
| GET | `/api/titles` | List | Yes | `?sort=updated_at\|original_title\|release_date\|my_rating\|created_at&order=asc\|desc&decade=2020&release_from=YYYY-MM-DD&release_to=YYYY-MM-DD&include_no_release=false` |
| GET | `/api/titles/{id}` | GetByID | Yes |
| POST | `/api/titles` | Create | Yes |
| PATCH | `/api/titles/{id}` | Update | Yes |
| POST | `/api/titles/{id}/rematch` | Rematch | Yes | Set IDs + enqueue enrichment |
| GET | `/api/tmdb/search` | Search | Yes | `?query=...&type=movie\|tv` |
| PATCH | `/api/titles/{titleID}/episodes/{episodeID}` | ToggleWatched | Yes |
| POST | `/api/titles/{titleID}/episodes/batch-watch` | BatchMarkWatched | Yes |
| PATCH | `/api/titles/{titleID}/seasons/{seasonID}` | UpdateRating | Yes |
| POST | `/api/push/subscribe` | Subscribe | Yes |
| DELETE | `/api/push/subscribe` | Unsubscribe | Yes |
| GET | `/api/stats` | Get | Yes |
| GET | `/api/settings` | Get | Yes |
| GET | `/api/anilist/auth` | Authorize | Yes |
| POST | `/api/anilist/token` | SaveToken | Yes |
| DELETE | `/api/anilist/token` | Disconnect | Yes |
| GET | `/api/admin/tasks` | ListTasks | Yes |
| POST | `/api/admin/tasks/{id}/retry` | RetryTask | Yes |
| DELETE | `/api/admin/tasks/{id}` | DeleteTask | Yes |
| POST | `/api/admin/tasks/batch-delete` | DeleteTasksBatch | Yes | Body `{"ids": [1, 2]}` |
| POST | `/api/admin/refresh-all` | RefreshAll | Yes |

Full OpenAPI 3.0 spec: `docs/openapi.yaml`.

## Frontend (Preact)

Design tokens in `frontend/src/theme.ts` (JS) + `frontend/src/tokens.css` (CSS custom properties). CSS Modules (`*.module.css`) for all components. `clsx` for conditional classes. `theme.ts` retained for SVG attributes and dynamic values (`coverBackground()`, `accentWash()`). Shared utils in `frontend/src/utils.ts` (`getName`, `getTypeLabel`, `getStatusLabel`, `formatDate`, `watchedCount`, `totalEpisodes`). API client in `frontend/src/api.ts`. Types in `frontend/src/types.ts`.

### Hooks

| Hook | File | Purpose |
|---|---|---|
| `useApi` | `hooks/useApi.ts` | Fetch wrapper with loading/error/mutate |
| `useTitleStore` | `store.ts` | Zustand store: paginated title fetch, filter, sort (localStorage-persisted), loadMore, cache |
| `useSearchStore` | `store.ts` | Zustand store: search results persistence and scroll position |
| `usePush` | `hooks/usePush.ts` | Service worker registration + push subscription |

### Components

| Component | File | Purpose |
|---|---|---|
| Navbar | `components/Navbar.tsx` | 4-tab bottom nav (amber/teal/green/lavender) |
| FilterDrawer | `components/FilterDrawer.tsx` | Collapsible filter drawer (sort/status/type/series status/release date), shared by Library+Search. Sort hidden during search. Release date section: decade dropdown, date range inputs, include-no-release toggle |
| TitleCard | `components/TitleCard.tsx` | Horizontal card with progress + quick mark badge |
| PosterCard | `components/PosterCard.tsx` | Poster grid card (2:3 aspect, gradient overlay) |
| StatusBadge | `components/StatusBadge.tsx` | Colored status pill |
| SeasonTab | `components/SeasonTab.tsx` | Season pill with progress/check |
| EpisodeRow | `components/EpisodeRow.tsx` | Episode row with toggle watched |
| ActionDrawer | `components/ActionDrawer.tsx` | Collapsible drawer with quick actions (next ep, rate, IMDb, AniList) + manage (edit, fix match) |
| BottomSheet | `components/BottomSheet.tsx` | Reusable slide-up sheet with backdrop |
| RatingPrompt | `components/RatingPrompt.tsx` | 10-star rating with save/IMDb/AniList buttons |
| EditSheet | `components/EditSheet.tsx` | Edit type/status |
| AniListSheet | `components/AniListSheet.tsx` | AniList match confirm/fix |
| RematchSheet | `components/RematchSheet.tsx` | TMDB search + manual IDs to fix wrong match |
| MatchReviewCard | `components/MatchReviewCard.tsx` | Match review card with ID chips + confirm/fix |
| CoverPlaceholder | `components/CoverPlaceholder.tsx` | Type-colored gradient + icon for titles without cover (movie=blue, series=teal, anime=lavender). `coverBackground()` helper for CSS background string |

### Pages & Routes

| Route | Page | File |
|---|---|---|
| `/` | Library | `pages/Library.tsx` |
| `/search` | Search | `pages/Search.tsx` |
| `/add` | Add | `pages/Add.tsx` |
| `/stats` | Stats | `pages/Stats.tsx` |
| `/login` | Login | `pages/Login.tsx` |
| `/title/:id` | TitleDetail | `pages/TitleDetail.tsx` |
| `/admin` | Admin | `pages/Admin.tsx` |
| `/admin/validate` | Validate | `pages/Validate.tsx` |
| `/admin/tasks` | AdminTasks | `pages/AdminTasks.tsx` |
| `/admin/notifications` | AdminNotifications | `pages/AdminNotifications.tsx` |
| `/match-review` | MatchReview | `pages/MatchReview.tsx` |

## Quality

### Linting

`.golangci.yml` — errcheck, gocritic, govet. Run `make lint`.

### Backend Tests

`make test` — `testify/assert+require`, in-memory SQLite. Table-driven tests for repository (TitleRepository: filters, pagination, search, external IDs, status counts) and handlers (error paths: invalid ID, not found, bad JSON).

### Frontend Tests

`make test-front` — vitest + jsdom + @testing-library/preact. `vitest.config.ts` (CSS Modules non-scoped). `utils.test.ts` (19 tests), `ErrorBanner.test.tsx` (6 tests).
