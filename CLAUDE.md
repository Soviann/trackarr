# CLAUDE.md — Mandatory rules

## Project

PlexTracker — Personal media tracking app. Go 1.24 / SQLite / chi / Preact 10 / Vite / Docker. Single developer.

## Approach

- Act over ask. Only read files you will edit.
- Always prefer the robust/correct solution over the lazy/easy one, even if harder or more tedious. If trade-offs exist, present them to the user as a PO: describe user-visible impact of each option, not implementation effort.
- Key context: CLAUDE.md, MEMORY.md, `docs/patterns.md` — update patterns.md when adding routes/services/components/commands. Update `docs/user-guide.md` when adding user-facing features.
- CLAUDE.md and patterns.md must be optimized for LLM use and token efficiency, without loss of information/instruction.
- Complex tasks: plan → approval → implement. Large changes: verifiable chunks.
## Plans & Specs

Location: `docs/superpowers/plans/` and `docs/superpowers/specs/`. Completed → `done/` subfolder.
Audience = PO (non-technical). Structure around: user-visible behavior, UX/UI flows, screen descriptions, acceptance criteria. No code snippets, no implementation details, no language/framework references. Technical notes only in collapsed section if essential for scope estimation.
Versioning: git history is sufficient (single developer). Don't version-number files. Keep immutable once approved — if scope changes mid-implementation, add a `## Revision — YYYY-MM-DD` header with a one-liner. Name descriptively (`notification-push.md` not `plan-007.md`).

## Commands

**All via Makefile (runs inside Docker).** Never `go`/`node`/`npm` on host. Host-only: `git`, `gh`, `docker`, `make`.

`up` `down` `logs` `shell` `test` `test-front` `lint` `fmt` `build` `dev-frontend` `migrate` `import BACKUP_FILE=...` `import-dry BACKUP_FILE=...`

## Deploy

`.github/workflows/deploy.yml` → SSHes NAS → `nas-update.sh`. Release: update `CHANGELOG.md` + push `v*` tag. Hotfix: `gh workflow run deploy.yml`.

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

## Visual Verification

After UI/UX changes, verify in-browser via Chrome DevTools MCP:
1. Login with `DEBUG_LOGIN*` credentials from `.env.local`
2. Confirm changed screens work as expected
3. Check console for errors/warnings
4. Navigate other pages to catch regressions
5. If broken: fix → re-verify. Never claim done without visual confirmation.

## Git

- `<type>(scope): description` — `feat|fix|chore|refactor|docs`, French 3rd-person imperative
- Title = visible impact, not implementation detail
- Trailer: `Co-Built-By: Claude (<random funny quip>)` — vary each time
- Skip `git diff` when you made the edits. Merges: `--no-ff`.

## Language

Commits/docs: French. Code: English. CLAUDE.md: English.
