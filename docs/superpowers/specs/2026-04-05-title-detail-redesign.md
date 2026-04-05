# Title Detail Page Redesign

## Context

The title detail page for movies is mostly empty — a short 160px hero with title overlay, then black void until the action bar. Series/anime have episode lists but movies show nothing. The page needs to surface rich metadata (synopsis, genres, runtime, cast, external ratings) and consolidate all actions into a clean drawer pattern.

## Scope

Redesign the detail page layout for all title types (movie, series, anime). Fetch additional TMDB data. This applies to the existing `/title/:id` route.

## Layout Structure

### 1. Hero Cover (pure visual)

Full-width backdrop image, ~260px tall. No text overlay — just the cover art with a gradient fade to the background at the bottom. Only the back button (top-left) remains on the hero.

When no cover exists, the existing `CoverPlaceholder` (type-colored gradient + icon) fills the hero.

### 2. Identity Zone (poster + metadata)

Below the hero, overlapping slightly into the gradient:

- **Mini poster** (80x120px, rounded corners, shadow) on the left — shows the full cover art at small scale
- **Title info** beside the poster:
  - Title name (large, white)
  - Meta line: type label, year, runtime (e.g. "Movie · 1958 · 1h 09m")
  - For series: also show series status (e.g. "Series · 2020 · Returning")
  - Genre pills below (rounded, dark surface background)

No rating in the meta line — it has its own card below.

### 3. Content Cards (stacked)

Each card is a rounded surface (`#161616`) with subtle border, containing one content group:

**a) Ratings Card**
- Left: "My rating" label + large score (e.g. "4/10" in amber)
- Right: External ratings — TMDB score (teal). For anime: also AniList score (blue)
- Note: IMDb score is not available via TMDB API — only the IMDb link. The IMDb button in the action drawer links out.
- If no personal rating: show a "Not rated" placeholder or dash
- External ratings are display-only, not tappable

**b) Synopsis Card**
- "Synopsis" label
- Overview text from TMDB, truncated to ~3 lines with "Show more" toggle (expand/collapse)

**c) Cast & Crew Card**
- "Cast & Crew" label
- Text-only list: name on the left, role on the right
- Director first, then top cast (limit to ~5 entries)
- No avatar photos

**d) Details Card**
- "Details" label
- Key-value rows: Status (colored), Date added, Match source, Original title (if different from display name)

### 4. Series/Anime Additions

For series and anime, below the content cards:
- **Progress bar** (existing) — season progress with animated fill
- **Season tabs** (existing) — horizontal scrollable pills
- **Episode list** (existing) — sortable episodes with watched toggle

These remain unchanged from the current implementation.

### 5. Action Drawer (collapsible, fixed above navbar)

Replaces both the hero overlay buttons and the old fixed ActionBar. Same UX pattern as `FilterDrawer` on the Library page:

- **Handle bar** with "Actions" label and chevron — tap to toggle
- **Quick actions row**: Rate (amber), IMDb (gold), AniList (blue, anime only)
- **Manage row**: Edit (opens EditSheet), Fix match (opens RematchSheet)
- For series/anime: "Mark next" button (existing next-episode functionality) in quick actions
- Default state: collapsed (just the handle). Drawer opens upward on tap.
- The drawer slides up with the same `max-height` transition as FilterDrawer.

### 6. Navbar

Existing bottom navbar remains fixed at the very bottom, unchanged.

## New Data to Fetch from TMDB

The backend needs to fetch and store additional fields when enriching a title:

### Movie (`/movie/{id}`)
- `overview` — synopsis text
- `genres` — array of genre objects (id + name)
- `runtime` — duration in minutes
- `vote_average` — TMDB community score
- `credits` (appended) — `cast` array (actor name + character) and `crew` array (filter for Director)

### TV (`/tv/{id}`)
- `overview` — synopsis text
- `genres` — array of genre objects
- `episode_run_time` — array of typical runtimes
- `vote_average` — TMDB community score
- `credits` (appended) — same as movie
- `created_by` — show creators

### AniList (anime titles)
- Rating/score is already available via AniList sync — surface it in the ratings card

### Storage

New fields on the Title model:
- `overview` (text, nullable)
- `runtime` (integer, nullable, minutes)
- `tmdb_rating` (real, nullable)
- `genres` (text, nullable — JSON array of genre name strings)
- `credits` (text, nullable — JSON array of `{name, role}` objects, limited to director + top 5 cast)

These are populated during the enrichment pipeline (after matching) and refreshed during the daily background sync.

## UI Labels (English)

All labels in the UI use English:
- "My rating", "Synopsis", "Cast & Crew", "Details", "Show more", "Actions"
- "Quick actions", "Manage", "Rate", "Edit", "Fix match"
- Status values: "Watching", "Completed", "Dropped", "Plan to Watch"

## Interactions

- **Tap "Rate"** → opens existing RatingPrompt bottom sheet
- **Tap "IMDb"** → opens IMDb page in new tab
- **Tap "AniList"** → opens existing AniListSheet (for anime)
- **Tap "Edit"** → opens existing EditSheet (type/status)
- **Tap "Fix match"** → opens existing RematchSheet
- **Tap "Mark next"** → marks next unwatched episode (series/anime only)
- **Tap mini poster** → could open full-screen cover view (stretch goal, not in v1)
- **Tap "Show more" on synopsis** → expands/collapses the full text

## Acceptance Criteria

- Movie detail page shows: hero cover, mini poster, title/meta/genres, ratings (personal + external), synopsis, cast & crew, details, action drawer
- Series/anime detail page shows all the above plus progress bar, season tabs, episode list
- Anime titles show AniList score in ratings card and AniList button in action drawer
- Action drawer follows the same pattern as FilterDrawer (handle, collapse/expand, transition)
- Hero is clean — only back button, no text overlay, no action buttons
- New TMDB data (overview, genres, runtime, vote_average, credits) is fetched during enrichment
- Page works well on mobile (375px width), content is scrollable, action drawer + navbar are fixed
- Existing functionality preserved: rating, editing, rematch, AniList confirm, episode toggle
