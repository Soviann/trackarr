# Rule Catalog — Performance

## Zustand store subscriptions

IsUrgent: True

Use selector functions with Zustand to avoid re-renders from unrelated state changes. Never subscribe to the entire store.

Wrong: `const store = useTitleStore()`
Right: `const titles = useTitleStore(s => s.titles)`

## Memoize expensive props

IsUrgent: True

Wrap object/array props in `useMemo` to prevent child re-renders. Wrap callbacks in `useCallback`.

## Image optimization

Lazy-load cover images below the fold. Use `loading="lazy"` on `<img>` elements. `CoverPlaceholder` component handles missing covers.
