# AGENTS.md — Development Guidelines & Rules for AI Agents

Welcome to **Trackarr** (`github.com/Soviann/trackarr`). This document provides universal rules, architecture standards, and development workflows for AI agents and human contributors.

---

## 🏗️ Project Overview
- **Backend**: Go 1.24, `chi` router, SQLite with WAL mode & FTS5 full-text search.
- **Frontend**: Preact 10, TypeScript (strict mode), Vite, CSS Modules, PWA Service Worker.
- **Runtime**: Docker Compose containerized environment.

---

## 🎯 Approach & Core Rules
- **Key Context Sources**: Always check `docs/patterns.md` and topic guides in `docs/` (`docs/INDEX.md`) to understand domain logic, routes, and data flows before proposing changes.
- **Keep Documentation Synchronized**: Use domain reasoning to identify and update all impacted documentation:
  - `docs/patterns.md` when routes, services, repositories, models, or UI components change.
  - `docs/llm.md` when database schemas, models, or API endpoints change.
  - The relevant topic guide in `docs/` (or `docs/dev/` via `docs/INDEX.md`) when adding or modifying domain features.
- **Documentation Verification Pass**: Once documentation is edited, systematically perform a review pass to ensure zero broken references, no missing links, and full consistency between the documentation and the actual implementation.
- **Bundle Documentation with Code**: Never commit documentation updates in a separate trailing commit; always stage and commit documentation changes atomically with the implementation code in the same working commit.
- **Maintain Changelog**: Keep `CHANGELOG.md` updated with notable user-facing changes, improvements, and fixes under `## [Unreleased]` following the [Keep a Changelog](https://keepachangelog.com/fr/1.1.0/) standard.
- **Robustness Over Ease**: Prefer correct, resilient solutions over quick workarounds.
- **No Exploratory Waste**: When working on specific files or symbols, navigate and read targeted files directly.

---

## 💻 Development Commands (Docker-First)
All build, test, and lint commands run inside the Docker container via `Makefile`. Never run `go`, `node`, or `npm` directly on the host.

- `make up` : Start the development container environment (`docker compose -f docker-compose.dev.yml up -d`).
- `make down` : Stop the development environment.
- `make logs` : Tail application and container logs.
- `make shell` : Open a bash shell inside the running application container.
- `make test` : Run all backend Go unit tests with in-memory SQLite and FTS5 tags.
- `make test-front` : Run frontend unit tests (Vitest) and compile the production Vite build into `frontend/dist`.
- `make lint` : Run Go linter (`golangci-lint`).
- `make lint-front` : Run TypeScript compiler type check (`tsc`).
- `make fmt` : Format Go source code with `gofmt`.
- `make build` : Build the standalone Go production binary.
- `make dev-frontend` : Run Vite development server with Hot Module Replacement (HMR).
- `make reset-password` : Reset admin password via CLI (`PASSWORD=...`, `USERNAME=admin`).
- `make backfill-accents` : Backfill poster dominant accent colors (`FORCE=1`).
- `make import` / `make import-dry` : Import data from backup archive (`BACKUP_FILE=...`).
- `make db-reset` : Reset local development SQLite database and restart.

---

## 📐 Coding & Architecture Standards

### Backend (Go)
- **Dependency Injection**: Repository structs receive `database.DBTX` (`*sql.DB` or `*sql.Tx`). Service structs receive repositories and external API clients. Handlers receive services/repositories.
- **SQL Placement & Writer Pattern**:
  - All database queries reside in `internal/repository/`. Handlers and services must never execute raw SQL.
  - Mutating queries are strictly isolated in `*_writer.go` structs whose constructor requires `*sql.Tx` (e.g. `NewTitleWriter(tx)`), enforcing transactional writes at compile time.
- **Error Handling**: Wrap errors with context using `fmt.Errorf("operation description: %w", err)`.
- **Database Concurrency & SQLite Invariants**:
  - SQLite uses WAL mode with `MaxOpenConns = 1` for write safety.
  - **No Nested Transactions (Deadlock Risk)**: Never invoke `database.WithTx` or `database.WithTxContext` from inside an active transaction.
  - **Post-Commit Side Effects**: External HTTP calls, Web Push notifications, and async cascade triggers (e.g. backfill, AniList push) must always run **after** the write transaction has committed.
  - **Cursor Closure**: Always close row cursors (`rows.Close()`) before issuing nested queries.
- **Testing**: Use `testify/assert` and `testify/require` with fresh in-memory SQLite instances (`setupTestDB(t)`).

### Frontend (Preact / TypeScript)
- **Strict Typing**: No `any` types. Shared data interfaces live in `frontend/src/types.ts`.
- **Centralized Routing**: Use `routeTo.*` builders and `ROUTE_PATHS` defined in `frontend/src/routes.ts` for all navigation and link URLs.
- **Styling**: Component-scoped CSS modules (`ComponentName.module.css`). Use theme variables defined in `frontend/src/theme.ts` / `index.css`.
- **PWA & Cache Invalidation**: The Service Worker aggressively caches static assets. When verifying frontend changes in the browser, append a timestamp query parameter (e.g. `?t=$(date +%s)`) to bust the cache.
- **Live Rebuild**: After running `make test-front`, touch `main.go` to trigger `air` re-embedding `frontend/dist`.

---

## 🔍 Visual Verification
After UI/UX changes, visually verify screens using Chrome DevTools or browser tools against `http://localhost:8080`:
1. Check changed screens for correct rendering, responsive layout, and dark-theme contrast.
2. Inspect the browser console for JavaScript or network errors.
3. Test edge states (loading spinners, empty state, error banners, drawer open/close).

---

## 📝 Git & Commit Conventions
- Use **Conventional Commits**:
  - `feat(...)`: New user-facing capability.
  - `fix(...)`: Bug fix or error resolution.
  - `refactor(...)`: Code restructuring without functional change.
  - `docs(...)`: Documentation updates.
  - `test(...)`: Unit or integration tests.
  - `chore(...)`: Maintenance, dependencies, or configuration.
- Include attribution trailer:
  `Co-Built-By: [Agent or Author Name] (<short, witty, funny one-liner quip anchored in the commit>)`
- Merge commits must use `--no-ff`.

