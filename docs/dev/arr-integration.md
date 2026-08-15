# Arr (Radarr & Sonarr) Technical Specification

Technical specification for media downloader integrations (Radarr for movies, Sonarr for series).

## Architecture & Source Files
- Service & Proxy: `internal/service/arr.go` (`ArrService`)
- Background Worker: `internal/service/background_arr.go` (`handleArrPush`, `saveArrID`)
- HTTP Handler: `internal/handler/arr.go` (`ArrHandler`)
- Router Endpoints: `internal/router/router.go` (`/api/arr/*`, `/api/admin/arr`)
- Frontend Components: `frontend/src/components/ArrBadge.tsx`, `frontend/src/components/TypeBadge.tsx`
- Frontend Pages: `frontend/src/pages/ArrQueue.tsx`, `frontend/src/pages/AdminArr.tsx`

## Configuration Sources
Settings are loaded with fallback priority:
1. Environment variables: `RADARR_URL`, `RADARR_API_KEY`, `SONARR_URL`, `SONARR_API_KEY`.
2. Database settings: `radarr_url`, `radarr_api_key`, `sonarr_url`, `sonarr_api_key`, `radarr_root_folder`, `radarr_quality_profile`, `sonarr_root_folder`, `sonarr_quality_profile`.

## Push Workflow & ID Lookup
When a title is queued for push (`TaskTypeRadarrPush` or `TaskTypeSonarrPush`):
1. **Radarr**: Lookup term `term=tmdb:<tmdb_id>` against `/api/v3/movie/lookup`.
2. **Sonarr**: Lookup term `term=tvdb:<tvdb_id>` against `/api/v3/series/lookup`.
3. If entry already exists in Arr:
   - Updates monitoring, quality profile, and root folder via `PUT /api/v3/movie/{id}` or `PUT /api/v3/series/{id}`.
4. If entry is new:
   - Posts add payload via `POST /api/v3/movie` or `POST /api/v3/series` with `addOptions{searchForMovie: true, searchForMissingEpisodes: true}`.
5. Persists the returned ID:
   - Sets `titles.radarr_id = ?` or `titles.sonarr_id = ?`.

## Frontend Status Representation
- **Cards (PosterCard / TitleCard)**: `TypeBadge.tsx` displays colored top accent borders (`#ffc230` yellow for Radarr, `#00c0ff` cyan for Sonarr).
- **Badges**: `ArrBadge.tsx` renders current state:
  - `In Queue`: Title actively downloading.
  - `Downloaded`: Available on disk.
  - `Monitored`: Tracked for future releases.
  - `Ignored`: User marked title to bypass Arr prompts (`arr_ignored = 1`).
