# Patterns & Codebase Map

Update when adding routes, services, components, or commands.

## Status: Phase 6 complete

## Backend (Go)

### CLI Commands

| Command | File | Purpose |
|---|---|---|
| `serve` | `cmd/serve.go` | HTTP server |
| `import` | `cmd/import.go` | Simkl backup import |

### Models

`internal/model/` — Title (TitleType, TitleStatus, SeriesStatus, MatchStatus), TitleName, Season, Episode, WatchEvent (WatchEventSource), Setting.

### Services

| Service | File | Purpose |
|---|---|---|
| PlexService | `internal/service/plex.go` | Webhook scrobble processing, delegates to pipeline |
| SimklImporter | `internal/service/simkl.go` | Simkl backup import (zip/JSON) |
| Pipeline | `internal/service/matching/pipeline.go` | Orchestrates Steps 1-5 of media matching |
| TMDBClient | `internal/service/matching/tmdb.go` | TMDB API: search, details, episodes, translations, covers |
| AniListClient | `internal/service/matching/anilist.go` | AniList GraphQL: search, details, rating sync |
| CrossRefDB | `internal/service/matching/crossref.go` | anime-offline-database ID cross-referencing |
| GeminiClient | `internal/service/matching/gemini.go` | Gemini AI match verification + fuzzy resolve, key rotation |

### Matching Pipeline Steps

1. **Plex IDs** — use TMDB/IMDB from webhook metadata → `confirmed`
2. **Cross-reference** — anime-offline-database lookup �� `confirmed`
3. **TMDB search** — by title+year → proceed to Step 5
4. **AniList search** — anime only → proceed to Step 5
5. **Gemini verification** — high confidence → `pending_review`, low → `unconfirmed`

After matching: fetch multilingual names (TMDB en/fr, AniList romaji), download cover.

### External APIs

| API | File | Purpose |
|---|---|---|
| TMDB | `internal/service/matching/tmdb.go` | Metadata, episodes, covers, names |
| AniList | `internal/service/matching/anilist.go` | Anime search, episodes, rating sync |
| Gemini | `internal/service/matching/gemini.go` | Match verification/fallback |
| anime-offline-database | `internal/service/matching/crossref.go` | Cross-reference ID mapping |
| Google OAuth | `internal/handler/auth.go` | Auth |
| Web Push | `internal/service/push.go` | VAPID notifications |

### Repositories

`internal/repository/` — TitleRepository, SeasonRepository, EpisodeRepository, WatchEventRepository, SettingRepository. All DB queries live here.

### Handlers

`internal/handler/` — auth, title, episode, season, cover, webhook, spa. DI via struct with repos.

### Routes

| Method | Path | Handler | Auth |
|---|---|---|---|
| GET | `/api/health` | Health | No |
| POST | `/api/auth/google` | GoogleCallback | No |
| POST | `/api/auth/logout` | Logout | No |
| POST | `/api/webhook/plex` | HandlePlex | No |
| GET | `/api/covers/{filename}` | Serve | No |
| GET | `/api/titles` | List | Yes |
| GET | `/api/titles/{id}` | GetByID | Yes |
| POST | `/api/titles` | Create | Yes |
| PATCH | `/api/titles/{id}` | Update | Yes |
| PATCH | `/api/titles/{titleID}/episodes/{episodeID}` | ToggleWatched | Yes |
| POST | `/api/titles/{titleID}/episodes/batch-watch` | BatchMarkWatched | Yes |
| PATCH | `/api/titles/{titleID}/seasons/{seasonID}` | UpdateRating | Yes |

## Frontend (Preact)

Design tokens in `frontend/src/theme.ts`.
