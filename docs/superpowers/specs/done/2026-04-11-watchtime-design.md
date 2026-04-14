# Spec — Watchtime Tracking

## Overview

TitleDetail now shows how much time you have spent watching a title, including all rewatches. The total is always up to date and recalculates automatically if metadata changes.

## User-Visible Behaviour

- A "~14h watched" or "2h 15m watched" stat appears on the TitleDetail page, near the existing progress and status info
- For a movie: each time you mark it as watched counts as one full runtime — rewatching it twice means double the time
- For a series: every individual episode watch event (including rewatches) multiplies by the series' average episode runtime
- The total updates immediately each time you mark an episode as watched or unwatched
- If a title is rematched and gets a new runtime, the total is recalculated automatically using the new runtime
- If nothing has been watched yet, no watchtime is displayed (or shown as "—")

## Screen Description

On TitleDetail, below the title name and status badge, a new line reads e.g.:

> **~14h** watched

For a movie marked watched twice:

> **4h 30m** watched

## Acceptance Criteria

- Watchtime is shown on TitleDetail for movies and series
- Marking an episode as watched increases the total
- Unmarking an episode decreases the total
- Rewatching (marking again after unmark) correctly adds to the total
- After a rematch with a different runtime, the displayed total reflects the new runtime
- Simkl import and Plex webhook watches are included in the total
- Zero watchtime shows nothing (no "0m watched" displayed)

<details>
<summary>Technical notes</summary>

- New column: `total_watch_minutes INTEGER NOT NULL DEFAULT 0` on `titles`
- **Migration: `015_watchtime.up.sql`** — adds column with DEFAULT 0 (existing rows start at 0, not backfilled since watch_events history would require a complex one-off query and the column will self-correct as watches are toggled)
- Application-layer updates (not a DB trigger — consistent with how `last_watched_at` works):
  - `LibraryService.MarkWatched` / `ToggleWatched` / `BatchMarkWatched`: add or subtract `title.Runtime` (in minutes)
  - `SimklImporter`: add `runtime` per imported watch event
  - Plex webhook handler: add `runtime` on scrobble
- On rematch: recalculate as `COUNT(watch_events WHERE title_id = ?) × new_runtime` and persist
- Display format: ≥60 min → "Xh Ym", <60 min → "Ym", 0 → hidden
- Uses title-level `runtime` (average episode runtime for series) — per-episode runtime is not stored

</details>
