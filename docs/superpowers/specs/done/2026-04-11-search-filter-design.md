# Spec — Library Search & Genre Filter

## Overview

Search now searches your own library first, with an optional toggle to also pull results from TMDB. Genre filtering is added to the FilterDrawer with a searchable checklist, title counts, and AND/OR logic for multi-genre selection.

---

## Feature A — Library-First Search with TMDB Toggle

### User-Visible Behaviour

- Typing in the Search tab now returns results from your own library by default
- Results show the same card format as the Library page (poster, title, year, status)
- The FilterDrawer (accessible via the filter icon in the search bar) gains a new **"Search on TMDB"** toggle, off by default
- When the toggle is turned on:
  - TMDB results are fetched and displayed below your library results
  - A "TMDB Results" section header separates the two lists
  - TMDB results show an "Add" badge instead of a status badge
  - Library results that are also TMDB hits are not duplicated
- The toggle state persists during the session (resets on page reload)
- When there are no library results and the toggle is off, an empty state suggests turning on the toggle

### Acceptance Criteria

- Default search returns only library results
- Toggle on: TMDB results appear below library results with a clear separator
- No duplicate entries (a title in both library and TMDB appears only in the library section)
- Existing filters (type, status, sort, release date) continue to apply to library results
- TMDB results are not affected by library-side filters (except type: movie/series)
- Empty library results show a hint to enable the TMDB toggle

---

## Feature B — Genre Filter

### User-Visible Behaviour

- The FilterDrawer gains a new **"Genre"** section below the existing filters
- A small search input at the top of the section lets you type to narrow the genre list (e.g. type "sci" to surface "Science Fiction")
- Each genre row shows: genre name on the left, title count in your library on the right
- Genres are listed by count descending by default; selected genres float to the top
- Tapping a genre toggles its selection (multi-select supported)
- When **≥2 genres are selected**, an AND / OR toggle appears next to the "Genre" section header:
  - **AND** — shows only titles tagged with *all* selected genres
  - **OR** — shows titles tagged with *any* selected genre (default when switching from 1 to 2 selected)
- Active genre filters are shown as removable tags in the FilterDrawer and reflected in the result count
- Clearing all genres removes the filter entirely

### Acceptance Criteria

- Genre section appears in FilterDrawer on both Library and Search pages
- Genre counts are accurate and match the current library
- Searching the genre list works (case-insensitive, partial match)
- Selected genres float to the top of the list
- Single genre selected: results contain that genre
- Two genres + AND: results contain both genres
- Two genres + OR: results contain either genre
- Removing all genre selections restores the full (filtered by other criteria) library
- Genre filter composes correctly with existing filters (type, status, release date)

<details>
<summary>Technical notes</summary>

**Migration: `016_title_genres.up.sql`**
- Create `title_genres(title_id INTEGER NOT NULL REFERENCES titles(id) ON DELETE CASCADE, genre TEXT NOT NULL)`
- Index on `(genre)` and `(title_id)`
- Populate from existing JSON: `INSERT INTO title_genres SELECT id, value FROM titles, json_each(titles.genres) WHERE genres IS NOT NULL`
- Drop `genres` JSON column from `titles` after migration

**Genre filter SQL:**
- AND: `WHERE (SELECT COUNT(*) FROM title_genres tg WHERE tg.title_id = t.id AND tg.genre IN (?, ?)) = 2`
- OR: `WHERE EXISTS (SELECT 1 FROM title_genres tg WHERE tg.title_id = t.id AND tg.genre IN (?, ?))`

**Genre list endpoint:** `GET /api/genres` — returns `[{genre, count}]` ordered by count DESC; used to populate the FilterDrawer checklist.

**Library-first search:** The existing FTS search (`title_names_fts`) already searches the library. The TMDB toggle triggers a client-side call to the existing `GET /api/tmdb/search` endpoint and renders results below.

**TitleFilter** struct in `internal/repository/` gains `Genres []string` and `GenreOperator string ("AND"|"OR")` fields.

</details>
