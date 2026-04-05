# Patterns & Codebase Map

Update when adding routes, services, components, or commands.

## Status: T32 complete (Phase 8 — quality & documentation)

## Backend (Go)

### CLI Commands

| Command | File | Purpose |
|---|---|---|
| `serve` | `cmd/serve.go` | HTTP server |
| `import` | `cmd/import.go` | Simkl backup import |

### Models

`internal/model/` — Title (TitleType, TitleStatus, SeriesStatus, MatchStatus, NextEpisode), TitleName, Season (EpisodeCount, WatchedCount for listing), Episode, WatchEvent (WatchEventSource), Setting.

### Services

| Service | File | Purpose |
|---|---|---|
| PlexService | `internal/service/plex.go` | Webhook scrobble processing, delegates to pipeline |
| PushNotifier | `internal/service/push.go` | Interface (PushService + noopNotifier). Web Push VAPID |
| BackgroundService | `internal/service/background.go` | Daily title refresh (TMDB sync, auto-complete, push triggers) |
| SimklImporter | `internal/service/simkl.go` | Simkl backup import (zip/JSON) |
| Pipeline | `internal/service/matching/pipeline.go` | Orchestrates Steps 1-5 of media matching |
| TMDBClient | `internal/service/matching/tmdb*.go` | TMDB API: client (tmdb.go), search, details, covers |
| AniListClient | `internal/service/matching/anilist*.go` | AniList GraphQL: client (anilist.go), search, sync, covers |
| CrossRefDB | `internal/service/matching/crossref.go` | anime-offline-database ID cross-referencing |
| GeminiClient | `internal/service/matching/gemini.go` | Gemini AI match verification + fuzzy resolve, key rotation |

### Matching Pipeline Steps

1. **Plex IDs** — use TMDB/IMDB from webhook metadata → `confirmed`
2. **Cross-reference** — anime-offline-database lookup �� `confirmed`
3. **TMDB search** — by title+year → proceed to Step 5
4. **AniList search** — anime only → proceed to Step 5
5. **Gemini verification** — high confidence → `pending_review`, low → `unconfirmed`

Confidence levels: `ConfidenceHigh`, `ConfidenceMedium`, `ConfidenceLow` (constants in `pipeline.go`). Graceful degradation: nil clients are skipped, pipeline falls through to next step.

Each step sets `MatchSource` on the result (`plex_ids`, `crossref`, `tmdb_search`, `anilist_search`, `gemini_fuzzy`, `none`). Stored on Title alongside `OriginalTitle` (raw Plex name) for Match Review provenance display.

After matching: fetch multilingual names (TMDB en/fr, AniList romaji), download cover (TMDB first, AniList `coverImage.extraLarge` fallback). AniList covers prefixed `al-` to avoid filename collisions.

### External APIs

### External APIs

| API | File | Purpose |
|---|---|---|
| TMDB | `internal/service/matching/tmdb.go` | Metadata, episodes, covers, names |
| AniList | `internal/service/matching/anilist.go` | Anime search, episodes, rating sync |
| Gemini | `internal/service/matching/gemini.go` | Match verification/fallback |
| anime-offline-database | `internal/service/matching/crossref.go` | Cross-reference ID mapping |
| Google OAuth | `internal/handler/auth.go` | Auth |
| Web Push | `internal/service/push.go` | VAPID notifications |

### Database

`internal/database/` — `Open()`, `Migrate()`, `WithTx(db, fn)` transaction helper, `DBTX` interface (shared by `*sql.DB` and `*sql.Tx`). SQLite with WAL, `MaxOpenConns=1`.

### Repositories

`internal/repository/` — All repos use `database.DBTX` interface (works with `*sql.DB` and `*sql.Tx`). TitleRepository (PaginatedResult, TitleFilter with Limit/Offset/UpToDate/WatchingBehind/SeriesStatus/Sort/Order), SeasonRepository, EpisodeRepository, WatchEventRepository, SettingRepository, StatsRepository. All DB queries live here. Title search in `title_search.go`. `List()` returns paginated light response (no episodes, season counters + next_episode). `ListAll()` returns full data for background jobs. `GetByID()` returns full detail with episodes.

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
| GET | `/api/titles` | List | Yes | `?sort=updated_at\|original_title\|year\|my_rating\|created_at&order=asc\|desc` |
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

Full OpenAPI 3.0 spec: `docs/openapi.yaml`.

## Frontend (Preact)

Design tokens in `frontend/src/theme.ts` (JS) + `frontend/src/tokens.css` (CSS custom properties). CSS Modules (`*.module.css`) for all components. `clsx` for conditional classes. `theme.ts` retained for SVG attributes and dynamic values (`coverBackground()`, `accentWash()`). Shared utils in `frontend/src/utils.ts` (`getName`, `getTypeLabel`, `getStatusLabel`, `formatDate`, `watchedCount`, `totalEpisodes`). API client in `frontend/src/api.ts`. Types in `frontend/src/types.ts`.

### Hooks

| Hook | File | Purpose |
|---|---|---|
| `useApi` | `hooks/useApi.ts` | Fetch wrapper with loading/error/mutate |
| `useTitleStore` | `store.ts` | Zustand store: paginated title fetch, filter, sort (localStorage-persisted), loadMore, cache |
| `usePush` | `hooks/usePush.ts` | Service worker registration + push subscription |

### Components

| Component | File | Purpose |
|---|---|---|
| Navbar | `components/Navbar.tsx` | 4-tab bottom nav (amber/teal/green/lavender) |
| FilterDrawer | `components/FilterDrawer.tsx` | Collapsible filter drawer (sort/status/type/series status), shared by Library+Search. Sort hidden during search |
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
| `/validate` | Validate | `pages/Validate.tsx` |
| `/match-review` | MatchReview | `pages/MatchReview.tsx` |

## Quality

### Linting

`.golangci.yml` — errcheck, gocritic, govet. Run `make lint`.

### Backend Tests

`make test` — `testify/assert+require`, in-memory SQLite. Table-driven tests for repository (TitleRepository: filters, pagination, search, external IDs, status counts) and handlers (error paths: invalid ID, not found, bad JSON).

### Frontend Tests

`make test-front` — vitest + jsdom + @testing-library/preact. `vitest.config.ts` (CSS Modules non-scoped). `utils.test.ts` (19 tests), `ErrorBanner.test.tsx` (6 tests).
