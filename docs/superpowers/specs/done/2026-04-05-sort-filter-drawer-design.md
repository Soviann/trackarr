# Sort in Filter Drawer

## Overview

Add a sort selector to the filter drawer so users can choose how the title list is ordered, instead of the fixed "last updated" sort.

## UX

### Sort section

A "Sort" section appears in the filter drawer, **above the existing filter sections**. It contains 5 sort options displayed as chip buttons (same style as existing filter chips):

| Label | Sort field | Default direction |
|---|---|---|
| Last updated | `updated_at` | desc |
| Title | `original_title` | asc |
| Year | `year` | desc |
| Rating | `my_rating` | desc |
| Date added | `created_at` | desc |

### Interactions

- Tapping a chip selects it as the active sort (highlighted). It applies its default direction.
- Tapping the **already active** chip flips the sort direction.
- An arrow indicator on the active chip shows the current direction (up = asc, down = desc).

### Visibility

- The sort section is **hidden when a search is active**. Search results always use relevance-based sorting.
- When search is cleared, the sort section reappears with the user's last selection.

### Persistence

- Sort field + direction are saved in localStorage.
- Default on first visit: "Last updated", descending (preserves current behavior).

## Data flow

1. Frontend store includes `sort` and `order` in query params: `?sort=updated_at&order=desc`
2. Sort params are **not sent** when a search term is present.
3. Backend handler reads `sort` and `order` params, validates `sort` against an allowlist of column names, validates `order` as `asc` or `desc`. Falls back to `updated_at desc` if invalid or missing.
4. Repository builds `ORDER BY t.{sort} {order}` in the SQL query.

## Edge cases

- **Null ratings:** Titles without a rating sort last when sorting by rating (regardless of direction).
- **Null years:** Titles without a year sort last when sorting by year (regardless of direction).
- **Pagination continuity:** Sort applies before pagination. Changing sort resets offset to 0.
