# Patterns & Codebase Map

Update when adding routes, services, components, or commands.

## Status: Phase 0 (pre-scaffold)

No code yet. Sections below will be populated as implementation progresses.

## Backend (Go)

### CLI Commands

| Command | File | Purpose |
|---|---|---|
| `serve` | `cmd/serve.go` | HTTP server |
| `import` | `cmd/import.go` | Simkl backup import |

### Models

`internal/model/` — Title (TitleType, TitleStatus, SeriesStatus, MatchStatus), TitleName, Season, Episode, WatchEvent (WatchEventSource), Setting.

### External APIs

| API | File | Purpose |
|---|---|---|
| TMDB | `internal/service/matching/tmdb.go` | Metadata, episodes, covers, names |
| AniList | `internal/service/matching/anilist.go` | Anime search, episodes, rating sync |
| Gemini | `internal/service/matching/gemini.go` | Match verification/fallback |
| anime-offline-database | `internal/service/matching/crossref.go` | Cross-reference ID mapping |
| Google OAuth | `internal/handler/auth.go` | Auth |
| Web Push | `internal/service/push.go` | VAPID notifications |

## Frontend (Preact)

Design tokens in `frontend/src/theme.ts`.
