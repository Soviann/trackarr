# Rule Catalog — Business Logic

## Media type handling

IsUrgent: True

Always handle all three media types: `movie`, `series`, `anime`. Use `TitleType` enum, never raw strings. Anime has special paths (AniList integration, cross-referencing).

## API error handling

IsUrgent: True

All API calls via `api.ts` must handle errors gracefully. Use `useApi` hook's error state. Show `ErrorBanner` on failure, never silent swallow.

## Watch status transitions

Respect valid status transitions. Auto-complete series only when all episodes of last season are watched.
