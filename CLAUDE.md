# CLAUDE.md — Mandatory rules

## Project

PlexTracker — Personal media tracking app. Go 1.24 / SQLite / chi / Preact 10 / Vite / Docker. Single developer.

## Approach

- Act over ask. Only read files you will edit.
- Key context: CLAUDE.md, MEMORY.md, `docs/patterns.md` — update patterns.md when adding routes/services/components/commands. Update `docs/user-guide.md` when adding user-facing features.
- CLAUDE.md and patterns.md must be optimized for LLM use and token efficiency, without loss of information/instruction.
- Complex tasks: plan → approval → implement. Large changes: verifiable chunks.
- Plans: `docs/superpowers/plans/`.

## Commands

**All via Makefile (runs inside Docker).** Never `go`/`node`/`npm` on host. Host-only: `git`, `gh`, `docker`, `make`.

`up` `down` `logs` `shell` `test` `test-front` `lint` `fmt` `build` `dev-frontend` `migrate` `import BACKUP_FILE=...` `import-dry BACKUP_FILE=...`

## Environment

`.env` (committed, defaults) + `.env.local` (gitignored, secrets). Keys: `GOOGLE_CLIENT_ID`, `GOOGLE_ALLOWED_EMAIL`, `JWT_SECRET`, `TMDB_API_KEY`, `ANILIST_CLIENT_ID`, `ANILIST_CLIENT_SECRET`, `GEMINI_API_KEY`, `VAPID_*`.

## Gotchas

- SQLite `MaxOpenConns=1`: close row cursors before nested queries
- `node_modules` in Docker volume: host IDE shows TS errors (expected, builds fine in container)
- Vite dev server not started by `make up` — run `make dev-frontend` separately
- Auth cookie: `HttpOnly`, `SameSite=Lax`, no `Secure` in dev

## Standards

- `gofmt` + `golangci-lint`. TypeScript strict. Preact functional components.
- No magic strings — constants/enums. DB queries in `repository/` only.
- Handlers: struct with repos (DI), methods = HTTP handlers.
- Errors: `fmt.Errorf("context: %w", err)`. Tests: `testify/assert`, in-memory SQLite.

## Git

- `<type>(scope): description` — `feat|fix|chore|refactor|docs`, French 3rd-person imperative
- Title = visible impact, not implementation detail
- Trailer: `Co-Built-By: Claude (<random funny quip>)` — vary each time
- Skip `git diff` when you made the edits. Merges: `--no-ff`.

## Language

Commits/docs: French. Code: English. CLAUDE.md: English.
