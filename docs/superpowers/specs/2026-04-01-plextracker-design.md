# PlexTracker — Design Spec

Personal media tracking app that replaces Simkl. Plugs into Plex via webhooks to automatically track watch progress, with manual entry support and external links/sync to IMDB and AniList.

## Goals

- Replace Simkl as the central watch tracker
- Auto-track everything watched on Plex (movies, series, anime, documentaries)
- Know at a glance: what am I watching, where am I at, what do I need to finish
- Rate titles and seasons, with easy access to IMDB and AniList for external rating/sync
- Self-hosted on Synology DS920+ alongside Plex, zero ongoing cost

## Non-Goals

- Not a media database — no synopsis, cast, reviews (IMDB/AniList handle that)
- No desktop design — mobile-only viewport
- No offline support
- No multi-user — single user, Google OAuth with allowed email

## Architecture

Single Go binary with embedded SQLite, serving a Preact SPA. Deployed as one Docker container on Synology.

```
Plex Server ──webhook──▶ PlexTracker (Go binary)
                              │
Phone (browser) ◀──SPA───────┤
                              │
                         SQLite DB (/data/plextracker.db)
                              │
                    ┌─────────┼─────────┐
                    ▼         ▼         ▼
                  TMDB    AniList    Gemini AI
               (metadata) (sync)   (fallback match)
                    │
                    ▼
                  IMDB
               (links only)
```

### Tech Stack

- **Backend**: Go, SQLite, chi or echo router
- **Frontend**: Preact SPA, embedded in Go binary at build time
- **Font**: DM Sans (Google Fonts), single font for all text
- **Auth**: Google OAuth, single allowed email, JWT in HttpOnly Secure SameSite=Strict cookie
- **Push**: Web Push API (VAPID)
- **Deployment**: Docker Compose on Synology Container Manager

## Data Model

### titles

| Column | Type | Notes |
|--------|------|-------|
| id | int, PK | |
| type | enum | movie, series, anime |
| year | int | |
| cover_url | string, nullable | Local path, served from /data/covers/. Downloaded from TMDB on title creation/refresh |
| imdb_id | string, nullable | e.g. "tt1234567" |
| anilist_id | int, nullable | anime only |
| tmdb_id | int, nullable | |
| tvdb_id | int, nullable | |
| plex_rating_key | string, nullable | Plex internal ID |
| my_rating | int, nullable | 1-10 |
| status | enum | watching, completed, dropped, plan_to_watch. Note: "up to date" is a derived display state (watching + all aired episodes watched + series returning), not stored |
| series_status | enum, nullable | returning, ended, cancelled, in_production |
| created_at | datetime | |
| match_status | enum | confirmed (Plex IDs or user-confirmed), pending_review (Gemini high-confidence), unconfirmed (Gemini low-confidence or fallback) |
| updated_at | datetime | |

### title_names

| Column | Type | Notes |
|--------|------|-------|
| id | int, PK | |
| title_id | FK → titles | |
| name | string | |
| language | string | IETF tags (`en`, `fr`, `ja`) + `x-romaji` for AniList romaji. Skip native Japanese script. |
| is_primary | bool | Displayed by default in lists |

### seasons

| Column | Type | Notes |
|--------|------|-------|
| id | int, PK | |
| title_id | FK → titles | |
| season_number | int | |
| total_episodes | int, nullable | From TMDB/AniList. NULL = unknown |
| my_rating | int, nullable | 1-10 |

### episodes

| Column | Type | Notes |
|--------|------|-------|
| id | int, PK | |
| season_id | FK → seasons | Season row auto-created when first episode of a new season is inserted |
| episode | int | |
| name | string, nullable | |
| air_date | date, nullable | From TMDB/AniList. NULL = treat as aired |
| watched | bool | default false |
| watched_at | datetime, nullable | |
| plex_rating_key | string, nullable | |

### watch_events

| Column | Type | Notes |
|--------|------|-------|
| id | int, PK | |
| title_id | FK → titles | |
| episode_id | FK → episodes, nullable | null for movies |
| source | enum | plex, manual |
| plex_payload | json, nullable | raw webhook data for debugging |
| created_at | datetime | |

## API Routes

All routes require valid JWT cookie except where noted.

```
Auth:
  POST   /api/auth/google              OAuth callback, sets JWT cookie (unauthenticated)
  POST   /api/auth/logout              Clears JWT cookie

Titles:
  GET    /api/titles                   List (?status=&type=&search=&match_status=)
  GET    /api/titles/:id               Detail with seasons, episodes, names
  POST   /api/titles                   Manual add (IMDB URL or search)
  PATCH  /api/titles/:id               Update status, type, rating, match_status

Seasons:
  PATCH  /api/titles/:id/seasons/:id   Update season rating

Episodes:
  PATCH  /api/titles/:id/episodes/:id  Toggle watched / update
  POST   /api/titles/:id/episodes/batch-watch  Batch mark episodes as watched

Match Review:
  GET    /api/titles/review            List pending_review + unconfirmed titles
  POST   /api/titles/review/batch-confirm  Batch confirm matches

Covers:
  GET    /api/covers/:filename         Serve cached cover image from /data/covers/

Webhook:
  POST   /api/webhook/plex             Plex webhook (unauthenticated, see below)

AniList:
  GET    /api/anilist/auth              Redirect to AniList OAuth page
  GET    /api/anilist/callback          OAuth callback, stores token in settings

Push:
  POST   /api/push/subscribe           Register push subscription
  DELETE /api/push/subscribe           Unregister push subscription

Settings:
  GET    /api/settings                  Get settings (AniList connected status, etc.)

```

### settings

| Column | Type | Notes |
|--------|------|-------|
| key | string, PK | |
| value | string | |

Used for: `anilist_access_token`, `anilist_token_expires_at`, `push_subscription` (JSON), and any future config.

## Plex Webhook

### Event Handling

- **Plex docs**: [Webhooks](https://support.plex.tv/articles/115002267687-webhooks/), [API](https://developer.plex.tv/pms/#section/API-Info)
- **Listen to**: `media.scrobble` only (fired at ~90% watch completion)
- **Ignore**: `media.play`, `media.pause`, `media.resume`, `media.stop`, `media.rate`
- **Endpoint**: `POST /api/webhook/plex` (unauthenticated, local network only)

### Processing Flow

1. Receive POST, parse multipart JSON payload
2. Extract: title, type, season/episode, Plex `ratingKey`, external IDs (IMDB, TMDB, TVDB)
3. Title lookup by `plex_rating_key` or external IDs
4. If new title → run Media Matching Pipeline, fetch full episode list + multilingual names, create title + seasons + episodes
5. Mark episode/movie as watched, log watch_event. Re-watches: log a new watch_event but don't update episode watched/watched_at
6. Auto-update `titles.status`:
   - All episodes watched + series_status is ended/cancelled → `completed`
   - All episodes watched + series_status is returning → keep `watching` (up to date)
7. If last episode of a season or movie → send push notification prompting rating

## Media Matching Pipeline

Cascading strategy to resolve external IDs when a new title is encountered.

### Step 1 — Plex metadata IDs
Plex agents often embed IMDB, TMDB, TVDB IDs in the webhook payload. Use directly if present.

### Step 2 — Cross-reference mapping
[anime-offline-database](https://github.com/manami-project/anime-offline-database) maps between IMDB ↔ TMDB ↔ TVDB ↔ AniList ↔ MAL. Downloaded JSON, updated weekly by the background job.

### Step 3 — TMDB API search
If Step 1 yielded no IDs, search TMDB by title + year. Returns IMDB IDs.

### Step 4 — AniList GraphQL search
For anime, search AniList by title (romaji, English, native).

### Step 5 — Gemini AI verification/fallback
- If Steps 3 or 4 produced a match, Gemini verifies it: receives source info (Plex title, year, type) + candidate match (TMDB/AniList title, year, IDs), returns confidence-based YES/NO with reason
- High confidence → accept, set `match_status = pending_review`
- Low confidence → set `match_status = unconfirmed`
- If no match found, Gemini attempts fuzzy resolution from available info → `unconfirmed`
- Steps 1-2 matches (direct Plex IDs or cross-reference) → `confirmed`
- API key rotation pool — rotate to next key on rate limit (same pattern as bibliotheque)

### At every step
Store newly found IDs on the title, use them to look up remaining missing IDs before proceeding to the next step.

### Full pipeline failure
If no step produces a match, create the title anyway using available Plex metadata (title, year, type, plex_rating_key). Set `match_status = unconfirmed`. User can fix IDs later from the Match Review screen.

### Episode list fetch
On new title detection, fetch the full episode list:
- Series/documentaries → TMDB API
- Anime → AniList GraphQL API

This pre-populates all episodes so the user immediately sees what's unwatched.

### Title names
Fetch multilingual names from TMDB (en, fr, etc.) and AniList (romaji, English, native Japanese). Store in `title_names` with the appropriate language tag.

## External Integrations

### IMDB
- **Link only** — direct URL to the IMDB title page (`https://www.imdb.com/title/{imdb_id}/`)
- "Save & rate on IMDb" button saves rating locally first, then opens IMDB page

### AniList
- **Link** — direct URL to the AniList title page
- **Rating sync** — via AniList GraphQL API with OAuth
- AniList match card displayed on title detail; tapping it confirms the match and syncs rating
- Manual confirmation required for matches from Steps 3-5 of the pipeline
- **OAuth flow**:
  1. "Connect AniList" button in settings (or prompted on first sync attempt)
  2. Redirects to AniList OAuth authorization page
  3. AniList redirects back with access token (implicit grant, no refresh token)
  4. Token stored in `settings` table (`anilist_access_token`, `anilist_token_expires_at`)
  5. Tokens are long-lived (~1 year). On expiry, prompt user to re-authenticate
  6. Sync button on title detail sends rating to AniList via GraphQL mutation

## Manual Entry

### Add Screen (paste or search)
- Input: paste a URL or search by name
- Supported URLs: IMDB (`https://www.imdb.com/title/tt1234567/`), TVDB (`https://thetvdb.com/series/...` or `/movies/...`), AniList (`https://anilist.co/anime/12345/...`)
- Parse ID from URL → run matching pipeline to resolve all external IDs
- Show matched title with cover for confirmation
- Choose status: Watching, Already watched, Plan to watch
- "Already watched" → marks all episodes as watched, sets status to `completed`, triggers rating prompt
- Fetches full episode list on add (same as Plex flow)

### Share Target (PWA)
PlexTracker registers as a [Web Share Target](https://developer.chrome.com/docs/capabilities/web-apis/web-share-target) so users can share directly from IMDB, TVDB, AniList apps or browser to PlexTracker.

- Registered via `share_target` in the PWA manifest
- Accepts shared URLs (text/url) from Android's share sheet
- On receive: opens PlexTracker to a **validation page** that:
  1. Parses the shared URL to extract the external ID (same logic as Add screen)
  2. Runs the matching pipeline to resolve all external IDs
  3. Shows a title detail preview: cover, name, year, type, resolved external IDs
  4. If the title already exists in the library → shows current status and links to existing title detail
  5. If new → status picker (Watching, Already watched, Plan to watch) + "Add to library" button
- This validation page is also reused for the **Match Review** flow (confirm/fix match from the review queue)

## Background Job

Single periodic job running inside the Go process. Runs daily.

**"Title Refresh":**
1. Iterate all non-completed titles (watching, plan_to_watch) + any title with missing data (no cover, no episode list, no series_status)
2. Fetch current series status from TMDB/AniList
3. If status changed (e.g. returning → ended) → notify user via push notification
4. If ended/cancelled + all episodes watched → update to `completed`
5. Fetch latest episode list → create new episodes not yet in DB
6. Fetch cover image if missing, download and store in `/data/covers/`
7. Fetch multilingual title names if missing (TMDB: en, fr; AniList: romaji, English)
8. Update cover URLs and title names if changed
9. Refresh anime-offline-database JSON

**API rate limiting:**
- Throttle all external API calls (TMDB, AniList, Gemini)
- Sequential processing with delays between requests
- Respect per-API rate limits to avoid IP bans
- Gemini: key rotation pool, rotate on rate limit hit

## UI Design

### Principles
- Mobile-first and mobile-only — target device: Android (Pixel 9 Pro, Chrome)
- No iOS support needed
- Dark mode first (light theme deferred to later release)
- Font: DM Sans (Google Fonts)
- No desktop layout

### Screens

**Library (Home)**
- Title cards with cover, name, type, year, progress bar
- Quick action: circular checkmark button showing next episode number — one tap to mark watched
- Movies show "Watch" button instead of episode number
- Filter tabs: All, Watching, Up to date, Completed, Dropped, Plan to watch
- Top search bar: filters within the currently selected tab
- When marking a movie or season finale → rating prompt slides up

**Match Review**
- Accessible via badge/link showing count of pending_review + unconfirmed titles
- List of titles needing review, grouped by status (unconfirmed first)
- Each card shows: title name, matched external IDs, Gemini confidence reason
- Actions per title: Confirm, Fix match (re-run pipeline or manual ID entry)
- Batch confirm action for reviewing multiple titles at once

**Title Detail**
- Cover, multilingual names, type, year, series status
- Manual status change: user can set status to watching, completed, dropped, plan_to_watch
- Change type (movie, series, anime) if auto-detected incorrectly
- Changing status to `dropped` triggers the rating prompt (same as completing a series)
- Changing status to `completed` marks all episodes as watched and triggers the rating prompt
- Star rating (1-10) for overall title + per season, with "clear" link to unrate
- External links: IMDB (link) + AniList (link + sync button as match card)
- Season tabs
- Episode list with toggleable checkmarks (tap to watch/unwatch)

**Search (bottom nav)**
- Global search across all titles regardless of status filter

**Add**
- Paste IMDB URL or search by name
- Shows matched title with cover
- Choose status: Watching, Already watched, Plan to watch

**Stats**
- TBD — watch history stats, counts by type/status, rating distribution

**Rating Prompt (bottom sheet)**
- Triggered on movie/season finale completion (manual or Plex auto-mark), or when manually changing status to `dropped`
- Title name as link to detail page (dedicated line)
- "Season X completed!" or "Movie watched!"
- Star selector (1-10)
- "Save rating" — saves locally
- "Save & rate on IMDb" — saves locally, opens IMDB page
- "Save & sync AniList" — saves locally, syncs via API
- "Skip for now"

### Bottom Navbar
4 tabs: Library, Search, Add, Stats

### Push Notifications
- Triggered when Plex auto-marks a movie or season finale as watched
- Triggered when background job detects a series status change (ended, cancelled, new season announced)
- Tapping notification opens the rating prompt (for completions) or title detail (for status changes)

## Data Import

### Simkl One-Shot Import

One-time import of existing watch history, ratings, and progress from a Simkl backup export. Triggered via `make import` over SSH (same pattern as bibliotheque), not from the web UI.

**Input file**: `Simkl_backup_DD.MM.YYYY_HH.MM.zip` — contains a single `SimklBackup.json`.

**Backup structure**: Three top-level arrays: `movies`, `anime`, `shows`.

Each item provides:
- Title name, year, runtime
- Status: `completed`, `watching`, `plantowatch`, `hold`, `notinteresting`
- Rating (1-10, nullable)
- `last_watched_at` timestamp
- External IDs: `imdb`, `tmdb`, `tvdb`, `anilist`, `mal`, `simkl`
- For shows/anime: `seasons[].episodes[]` listing watched episode numbers only
- For anime: `anime_type` (`tv`, `movie`, `ova`, `ona`, `special`, `music video`)

**Status mapping**:

| Simkl | PlexTracker |
|-------|-------------|
| `completed` | `completed` |
| `watching` | `watching` |
| `plantowatch` | `plan_to_watch` |
| `hold` | `watching` |
| `notinteresting` | `dropped` |

**Import process** (Go CLI command, e.g. `plextracker import <path-to-zip>`):

1. Unzip and parse `SimklBackup.json`
2. For each item, create a `titles` row:
   - Map Simkl type → PlexTracker type (`movies` → movie, `shows` → series, `anime` → anime). For anime, `anime_type = movie` maps to movie, all others map to anime.
   - Store all available external IDs (imdb_id, tmdb_id, tvdb_id, anilist_id)
   - Set `match_status = confirmed` (IDs come directly from Simkl's own cross-references)
   - Map status per table above
   - Import `user_rating`
3. Create one `title_names` entry (Simkl title, language `en`, `is_primary = true`)
4. For shows/anime with watched episodes: create `seasons` and `episodes` rows for watched episodes only, mark them as watched with `watched_at = last_watched_at`
5. For completed movies: log a `watch_event` with `source = manual` and `created_at = last_watched_at`
6. Support `--dry-run` flag for preview without writes
7. Skip duplicates on re-run (match by imdb_id or tmdb_id)

**What the import does NOT do** (deferred to enrichment):
- Fetch full episode lists (Simkl only exports watched episodes)
- Fetch episode names or air dates
- Fetch multilingual title names
- Download cover images
- Fetch series_status (returning, ended, etc.)

**Makefile target**: `make import` — the zip file is placed manually on the NAS at `/volume1/downloads/`. The Makefile copies it into the container, runs the CLI command inside Docker, and cleans up. Supports `DRY_RUN=1`.

### Post-Import Enrichment

No dedicated enrichment task. The existing daily **Title Refresh** job handles it naturally — imported titles will have missing covers, episode lists, names, and series_status, and the refresh job already fetches all of that. The only change is that Title Refresh should also process titles with missing data (no cover, no full episode list), not just non-completed titles. Given ~7,500 imported titles, full enrichment will take multiple daily runs — that's fine, it's a one-time backfill that resolves itself over time.

## Auth

- Google OAuth login
- Single allowed email via `GOOGLE_ALLOWED_EMAIL` env var
- JWT issued on login, stored in HttpOnly Secure SameSite=Strict cookie (not localStorage — app is publicly accessible)
- All API endpoints require valid JWT except Plex webhook endpoint
- No user table — single user app

## Deployment

Docker Compose on Synology DS920+ (Container Manager).

```yaml
services:
  plextracker:
    image: ghcr.io/nicolasvasse/plextracker:latest
    container_name: plextracker
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      - ./data:/data
    environment:
      - GOOGLE_CLIENT_ID=
      - GOOGLE_ALLOWED_EMAIL=
      - GEMINI_API_KEY=        # comma-separated for rotation pool
      - TMDB_API_KEY=
      - ANILIST_CLIENT_ID=
      - ANILIST_CLIENT_SECRET=
      - VAPID_PUBLIC_KEY=
      - VAPID_PRIVATE_KEY=
      - VAPID_SUBJECT=mailto:
```

- Single container, ~20-30MB memory
- SQLite DB persisted in `/data` volume
- Plex webhook: `http://<nas-ip>:8080/api/webhook/plex`
- CI builds image → ghcr.io, deploy via `docker compose pull && up -d`
