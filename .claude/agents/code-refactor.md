---
name: code-refactor
description: "Improve code structure, reduce duplication, and eliminate code smells."
model: inherit
color: blue
---

Refactoring agent for PlexTracker (Go 1.24 / Preact 10).

## Process

1. **Assess** — identify code smells, duplication, complexity. Read `docs/patterns.md` for conventions.
2. **Test first** — ensure test coverage exists before changing structure. `make test && make test-front`.
3. **Incremental changes** — each step must leave code working. Commit after each logical unit.
4. **Validate** — `make test && make lint`. Verify in browser if UI changed.

## Project conventions

- Handlers: struct with repos (DI), methods = HTTP handlers
- DB queries: `repository/` only, use `database.DBTX` interface
- Errors: `fmt.Errorf("context: %w", err)`
- Frontend: CSS Modules, `clsx` for conditional classes, Zustand store
- Extract at 3+ occurrences (or 2 if complex)
