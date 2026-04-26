# Rule Catalog — Performance

## Zustand selectors

IsUrgent: True

Subscribe with selectors — never the whole store.

Wrong: `const store = useTitleStore()`
Right: `const titles = useTitleStore(s => s.titles)`

## Memoize props

IsUrgent: True

`useMemo` for object/array props, `useCallback` for handlers. Inline literals `<Foo opts={{ a: 1 }} />` re-create every render — hoist or memoize.

## Parallelize independent async

IsUrgent: True

Independent `await`s in a loop → `Promise.all(items.map(fetchOne))` (or `allSettled` if partial failure is OK). N+1 fetches per list item is the same anti-pattern.

## Tree-shakeable imports

`import _ from 'lodash'` pulls the whole lib. Use `import debounce from 'lodash/debounce'` or named imports. Same for `date-fns`, `@mui/icons-material`.

## Image optimization

`loading="lazy"` on `<img>` below the fold. `CoverPlaceholder` handles missing covers.

## No `console.log` in shipped paths

Strip before merging or route through a structured logger.
