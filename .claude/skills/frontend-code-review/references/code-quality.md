# Rule Catalog — Code Quality

## Styling — CSS Modules

IsUrgent: True

`*.module.css` only. No inline styles except dynamic values from `theme.ts` (e.g., `coverBackground()`). `clsx` for conditional classes.

## Type safety

IsUrgent: True

- No `any`, no `@ts-ignore` — narrow with discriminated unions.
- No `value!` (non-null assertion) without a guard immediately above.
- No `as Foo` casts to unrelated types — fix the underlying type.
- Public functions declare return types.

## Strict equality and `const`

`===` / `!==` only. `const` by default, `let` when reassigned, never `var`.

## Async correctness

IsUrgent: True

- No floating promises in event handlers / effects — `await`, `void`, or `.catch()`.
- Never `array.forEach(async fn)` — use `for...of` + `await` or `Promise.all(array.map(fn))`.
- Sequential `await` in a loop with independent iterations → `Promise.all`.

## Error handling

- No empty `catch {}`.
- `JSON.parse(x)` in `try/catch`.
- `throw new Error("...")`, never bare strings or objects.
- API errors via `useApi` → `ErrorBanner`. No silent swallow.

## State immutability

IsUrgent: True

Zustand setters return new objects/arrays — never `.push` / `.splice` in place. Preact props are immutable.

## Preact idioms

IsUrgent: True

- `useEffect` / `useCallback` / `useMemo` deps must be exhaustive.
- `key={index}` on dynamic lists is a smell — use a stable ID.
- Don't compute derived state in `useEffect` + `setState` — derive during render or with `useMemo`.

## Design tokens

`tokens.css` for colors / spacing / typography. `theme.ts` only for JS values needed in SVG attributes or computations.

## Component conventions

- Preact functional components only.
- Props interface defined and exported per component.
- No magic strings — use shared constants.

## Naming

camelCase: vars/functions. PascalCase: types/components/classes. SCREAMING_SNAKE: module-level true constants.
