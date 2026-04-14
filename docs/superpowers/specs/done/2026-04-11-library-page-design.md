# Spec — Library Page Enhancements

## Overview

The Library page gains three new features: a "Coming up" strip for soon-airing episodes, a "Continue Watching" strip for in-progress titles, and a bulk selection mode for mass status changes and deletions.

Both strips are **collapsible and lazy-loaded** — their content is not fetched on page load. The user must expand a strip to trigger the load. This keeps the Library page fast regardless of library size.

---

## Feature A — Coming Up Strip

### User-Visible Behaviour

- A collapsible "Coming up" strip appears at the very top of the Library, above "Continue Watching"
- Collapsed by default; the header shows the section title and a count badge (e.g. "Coming up · 5")
- Tapping the header expands the strip and triggers a load
- Shows a horizontal scrollable row of poster cards, ordered soonest first
- **Which titles appear:**
  - Watching titles that have a known upcoming episode air date
  - Plan to Watch titles that have a known upcoming premiere or air date
- Each card shows: poster, title name, next episode label (e.g. "S2 · E6"), and an air date badge on the poster:
  - Today → amber "Today" badge
  - This week → teal day name (e.g. "Fri.")
  - Later → grey "in 12d"
- Tapping a card navigates to TitleDetail

### Acceptance Criteria

- Strip is collapsed and not loaded on page load
- Count badge reflects the number of upcoming titles
- Cards are ordered by air date ascending (soonest first)
- Air date badge accurately reflects the time until air
- Tapping a card opens the correct TitleDetail

---

## Feature B — Continue Watching Strip

### User-Visible Behaviour

- A collapsible "Continue Watching" strip appears below "Coming up", above the main library grid
- Collapsed by default; the header shows the count badge (e.g. "Continue Watching · 4")
- Tapping the header expands the strip and triggers a load
- Shows a horizontal scrollable row of poster cards
- **Which titles appear:** Watching titles with at least one unwatched episode
- Ordered by most recently watched first (`last_watched_at DESC`)
- Each card shows: poster, title name, next episode label (e.g. "S2 · E5"), amber progress bar (total episodes watched ÷ total episodes in series)
- Tapping a card navigates to TitleDetail

### Acceptance Criteria

- Strip is collapsed and not loaded on page load
- Only Watching titles with ≥1 unwatched episode appear
- Ordered by most recently watched
- Progress bar reflects overall series progress (not just current season)
- Watching titles continue to appear in the main library grid below

---

## Feature C — Bulk Operations

### User-Visible Behaviour

- A "Select" button is always visible in the Library top bar (alongside existing filter/sort controls)
- **Entering selection mode:** tap "Select" → checkboxes appear on every card in the grid and list view, a "Select all · X of N titles" row appears below the top bar, the "Select" button is highlighted
- **Selecting titles:** tap any card to toggle its checkbox; tap "Select all" to select everything currently visible (respecting active filters)
- **Action bar:** once ≥1 title is selected, an action bar appears at the bottom of the screen with two buttons:
  - **Status** → opens a bottom sheet to pick a new status (Watching / Completed / Dropped / Plan to Watch); applies to all selected titles simultaneously
  - **Delete** → shows a confirmation dialog ("Delete 3 titles? This cannot be undone."); on confirm, permanently removes the titles from the library
- **Exiting selection mode:** tap "Select" again or tap "Cancel" in the top bar
- Single-title delete is also added to the ActionDrawer (for consistency, even when not in bulk mode)

### Acceptance Criteria

- "Select" button is always visible on the Library page
- Checkboxes appear on both grid (poster cards) and list (title cards) views
- "Select all" selects all titles matching current filters, not the entire library
- Bulk Status change applies to every selected title
- Bulk Delete requires explicit confirmation and permanently removes selected titles
- Single-title delete available in ActionDrawer
- Exiting selection mode deselects all

<details>
<summary>Technical notes</summary>

**Migration: `017_next_air_date.up.sql`** — adds two columns to `titles`:
- `next_air_date DATE` (nullable) — ISO date of next episode air date
- `next_air_episode TEXT` (nullable) — e.g. "S2 E6" label for display

`next_air_date` populated during TMDB background refresh from `next_episode_to_air.air_date` on series; for movies, from `release_date` if status is upcoming and title is Plan to Watch.

**New API endpoints:**
- `GET /api/titles/continue-watching` — returns titles with `status=watching` and unwatched episodes, ordered by `last_watched_at DESC`
- `GET /api/titles/upcoming` — returns titles with `next_air_date >= today`, ordered by `next_air_date ASC`
- `DELETE /api/titles/{id}` — hard delete (cascades to seasons, episodes, watch_events, title_names, title_genres)
- `POST /api/titles/batch-delete` — body `{"ids": [1,2,3]}`; applies same cascade
- `POST /api/titles/batch-status` — body `{"ids": [1,2,3], "status": "completed"}`

Frontend: Library.tsx — collapsible sections with lazy fetch on expand (fetch fires on first open only, result cached until page refresh).

</details>
