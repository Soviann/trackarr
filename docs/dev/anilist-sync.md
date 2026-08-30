# AniList Sync & Multi-Part Seasons Specification

[← Back to Index](../INDEX.md)

---

## Architecture & Source Files
- Relations & Prequel Traversal: `internal/service/matching/anilist_relations.go`
- Client & GraphQL: `internal/service/matching/anilist*.go`
- State Push Service: `internal/service/anilist_push.go`
- Repository: `internal/repository/season_external_ids.go`, `internal/repository/title_relation*.go`
- Task Handlers: `internal/service/task_handlers.go` (`TaskTypeAniListPushSeason`, `TaskTypeAniListPushMovie`)
- Frontend Components: `frontend/src/components/SeasonAniListStrip.tsx`, `frontend/src/components/SeasonSideStories.tsx`, `frontend/src/components/FranchiseRelationsSection.tsx`, `frontend/src/components/RematchSheet.tsx`

## Franchise Relations & Side Stories (`title_relations`)
Database table: `title_relations` (Migration 042, PK: `id`, Unique: `title_id, provider, external_id`):
- Connects anime series to their chronological side stories, movies, OVAs, and spin-offs discovered via AniList `relations.edges`.
- Relations attached to a specific season part are stored with `season_id` (rendering in-line at the end of the parent season).
- Non-series relations (e.g. standalone movies or franchise spin-offs) are surfaced in the dedicated "Univers & Franchise" section.
- Resolves local library matches dynamically (`matched_title_id`) via `LEFT JOIN titles mt ON mt.anilist_id = tr.external_id` for instant watch tracking and navigation.

## AniList Season Chain Traversal
`AniListClient.ResolveSeasonChain(ctx, id)` resolves the root franchise series by traversing `PREQUEL` edges:
- **Formats**: `TV` and `ONA` entries increment the season ordinal counter. `MOVIE`, `OVA`, and `SPECIAL` are traversed without incrementing ordinal.
- **Edge Selection**: `pickPrequel` prefers `TV` > `ONA` > other `ANIME` edges.
- **Guards**: Cycle detector fails on revisited node; maximum traversal depth is capped at 25 (`maxChainDepth`).
- **Return Type**: `SeasonChain{RootID, RootTitle, SeasonNumber, IsRoot, RootIsSeries}`.

## Season Auto-Attach Matrix (`decideSeasonAction`)
Pure function in `internal/service/taskqueue.go`:

| Input Condition | Action Taken | Rationale |
|---|---|---|
| `chain == nil` | `legacy` | Fallback to IMDb ID collision |
| `chain.IsRoot` | `legacyRoot` | Root franchise entry (offset 0, season 1) |
| `!chain.RootIsSeries` | `none` | Never attach a TV series to a movie/special root |
| `parentByIDs != nil` | `mergeInto parentByIDs` at `SeasonNumber-1` | Matches parent via shared IMDb/TMDB ID |
| `parentByRoot != nil` AND has own external IDs | `none` | Distinct franchise spin-off (e.g. Dragon Ball Z) |
| `parentByRoot != nil` AND has NO own external IDs | `mergeInto parentByRoot` at offset | Anonymous sequel merged into parent series |
| No parent found AND has own external IDs | `none` | Standalone entry |
| No parent found AND has NO own external IDs | `createRoot` | Creates parent container then merges |

## Multi-Part Seasons Model (`season_external_ids`)
Database table: `season_external_ids` (Migration 027, PK: `season_id, provider, external_id`):
- Supports split-cour anime seasons (e.g. *Attack on Titan Final Season Part 1 + Part 2*).
- **Columns**: `season_id`, `provider` (always `'anilist'`), `external_id`, `anilist_episode_count`, `anilist_start_date`, `anilist_average_score`, `sort_order`.
- **Deterministic Ordering**: `ORDER BY (sort_order IS NULL), sort_order, (anilist_start_date IS NULL), anilist_start_date, external_id`.

## State Derivation & Push Protocol
`AniListPushService.PushSeasonState(ctx, seasonID)`:
1. Calls `DerivePartStates(titleStatus, parts, watched, seasonTotal)`:
   - Walks parts in sort order. Part *i* covers episodes `(cum, cum + anilist_episode_count]`.
   - The final part absorbs any remaining episodes beyond known counts.
2. Derives per-part status:
   - All episodes in range watched → `COMPLETED`.
   - Dropped + unfinished in range → `DROPPED`.
   - Partial in range → `CURRENT`.
   - 0 episodes watched + title is planning → `PLANNING`.
3. Pushes state to AniList GraphQL API:
   - Score is pushed **only** if status is `COMPLETED` or `DROPPED` (`ShouldPushRating`).
   - If AniList returns HTTP 401: stores `settings.anilist_token_invalid = 'true'` and suppresses subsequent pushes until re-authenticated.
