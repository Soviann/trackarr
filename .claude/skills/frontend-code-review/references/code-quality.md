# Rule Catalog — Code Quality

## CSS Modules for all styling

IsUrgent: True

Use `*.module.css` for component styles. No inline styles except dynamic values from `theme.ts` (e.g., `coverBackground()`). Use `clsx` for conditional class composition.

## TypeScript strict mode

IsUrgent: True

No `any` types. No `@ts-ignore`. Use proper type narrowing and discriminated unions.

## Design tokens consistency

Use `tokens.css` custom properties for colors, spacing, typography. Use `theme.ts` only for JS values needed in SVG attributes or dynamic computations.

## Component conventions

- Preact functional components only
- Props interface defined and exported per component
- No magic strings — use constants from shared modules
