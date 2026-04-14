# Spec — Richer Stats Page & Activity Feed

## Overview

The Stats page gains new insight sections: watchtime in the overview, a genre breakdown chart, streak cards, and a global activity feed. TitleDetail gains a per-title watch history accessible via a dedicated button.

---

## Feature A — Richer Stats Page

### User-Visible Behaviour

**Overview section** gains a fourth card: **"847h watched"** (total time spent watching across all titles, including rewatches). This uses the `total_watch_minutes` data introduced in the Watchtime spec.

**New "Top genres" section** — a horizontal bar chart showing your most-watched genres:
- Each row: genre name on the left, a colored fill bar proportional to count, count number on the right
- Shows the top 10 genres in your library
- Colors cycle through the app's accent palette
- Requires the `title_genres` table from the Search & Filter spec; shows an empty state if unavailable

**New streak cards** — two small cards side by side below the genre chart:
- 🔥 **Current streak:** number of consecutive calendar days with at least one watch event ending today (or yesterday if nothing watched today)
- 🏆 **Best streak:** longest consecutive watch streak ever recorded

### Acceptance Criteria

- Watchtime card displays the correct total, formatted as "Xh" or "Xh Ym"
- Watchtime card shows "—" if nothing has been watched
- Genre chart shows genres ordered by count descending
- Genre chart shows an empty state gracefully if `title_genres` is not populated
- Current streak resets to 0 if no watch event yesterday or today
- Best streak reflects the all-time record from watch_events history

---

## Feature B — Activity Feed (Global)

### User-Visible Behaviour

- The Stats page gains a new **"Activité récente"** section at the bottom
- Events are grouped under **sticky date headers** (Aujourd'hui, Hier, then e.g. "10 avr.")
- Each entry shows: poster thumbnail, title name, episode name + number (or "Film" for movies), and a type badge:
  - Teal "Épisode" badge for episode watches
  - Purple "Film" badge for movie watches
  - Green "Terminé" badge when a series is completed (auto-detected)
- **Load more:** initially shows the last 50 events; a "Load more" button appends the next 50
- Tapping an entry navigates to TitleDetail for that title

### Acceptance Criteria

- Events grouped correctly by date in the user's local timezone
- Date headers are sticky while scrolling within a group
- "Load more" loads the next page without reloading existing entries
- Tapping an entry opens the correct TitleDetail
- "Terminé" badge appears when the last episode of a series was watched

---

## Feature C — Per-Title Watch History

### User-Visible Behaviour

- TitleDetail gains a **"Historique"** button (near the existing action controls)
- Tapping it opens a view listing all watch events for that title, in reverse chronological order
- Each row shows: episode name + number (or "Film"), date and time of the watch
- If an episode was watched more than once, a rewatch badge (×2, ×3…) appears on the right
- For movies watched multiple times, each watch appears as a separate row

### Acceptance Criteria

- "Historique" button appears on all TitleDetail pages
- All watch events are listed, including imports from Simkl and Plex webhooks
- Rewatch badge correctly counts multiple watches of the same episode
- Rows ordered newest first

<details>
<summary>Technical notes</summary>

**No new migration needed** — all data comes from existing `watch_events`, `titles`, and `title_genres` (soft dependency).

**Stats API additions** — extend `GET /api/stats` response:
- `overview.total_watch_minutes` — `SUM(total_watch_minutes)` from titles (Feature 2 dependency)
- `genres` — `[{genre, count}]` from `title_genres` JOIN titles, top 10 (Feature 5 dependency; return empty array if table missing)
- `streaks.current` and `streaks.best` — computed from `watch_events` grouped by calendar date

**New endpoints:**
- `GET /api/stats/activity?limit=50&offset=0` — paginated watch_events JOIN titles JOIN episodes, ordered by `watched_at DESC`; response: `[{title_id, title_name, cover_url, episode_number, episode_name, watched_at, type, is_completion}]`
- `GET /api/titles/{id}/history` — all watch_events for a title, with rewatch_count per episode; response: `[{episode_id, episode_number, episode_name, watches: [{watched_at}]}]`

**Frontend:**
- Stats.tsx: add three new sections (watchtime card, genre bars, streak cards, activity feed)
- Activity feed uses the existing paginated pattern from MatchReview (load more button)
- TitleDetail: "Historique" button opens a new bottom sheet or navigates to a sub-route

**Soft dependencies:** Watchtime card requires Feature 2 (`total_watch_minutes` column). Genre chart requires Feature 5 (`title_genres` table). Both should show graceful empty states if the dependency is not yet deployed.

</details>
