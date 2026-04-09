# CLAUDE.md — Mandatory rules

## Project

PlexTracker — Personal media tracking app. Go 1.24 / SQLite / chi / Preact 10 / Vite / Docker. Single developer.

## Approach

- Act over ask. Only read files you will edit.
- Act on user instructions directly — no exploratory glob/grep when the user already says where/what.
- Don't verify existence of items already known from the plan or memory.
- Always prefer the robust/correct solution over the lazy/easy one, even if harder or more tedious. If trade-offs exist, present them to the user as a PO: describe user-visible impact of each option, not implementation effort.
- Key context: CLAUDE.md, MEMORY.md, `docs/patterns.md` — update patterns.md when adding routes/services/components/commands. Update `docs/user-guide.md` when adding user-facing features.
- CLAUDE.md and patterns.md must be optimized for LLM use and token efficiency, without loss of information/instruction.
- Complex tasks: plan → approval → implement. Large changes: verifiable chunks.

## Token Cost Optimization

- **Parallelize independent tool calls** in one turn (multiple Read/Grep/Glob = 1 round-trip, not N). Only serialize when output of call A is needed for call B.
- **Combine sequential Bash** with `&&` when simple and output-independent (`make lint && make test`). Don't combine when you need to read A's output to decide B, or when failure diagnosis/timeouts differ.
- **No exploratory search when the target is known.** User named a file/symbol → Read/Grep it directly. Skip verifying existence of items already known from plan/memory.
- **Direct tools > subagents for simple lookups.** A subagent = a full Opus conversation billed on top. Use Grep/Glob/Read for: known targets, single-file reads, specific symbol lookups, <3 expected queries. Use subagents (Explore, general-purpose) only for: open-ended research spanning many files, tasks whose raw output would pollute main context, genuinely parallel independent investigations.
- **Force cheap models on mechanical subagents.** When dispatching a subagent for file search, simple refactor, or mechanical lookup, pass `model: "haiku"` or `"sonnet"` in the Agent call.
- **Prefer `gh --json field1,field2`** over MCP GitHub tools for simple queries.

## Audits

`docs/audits/YYYY-MM-DD.md` (active) → `docs/audits/done/` (completed). Work top-to-bottom by session; mark items done inline (strikethrough); update file then commit per session. No implement skill overhead.

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
- Linters/formatters: run before committing (not after each file edit), only on modified files.
- DRY: extract at 3+ occurrences (or 2 if complex). Exception: when abstraction obscures intent or coupling > duplication cost.
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

- Format: `<type>(scope): description` — types: `feat|fix|chore|refactor|docs`, French 3rd-person imperative
- Commit title = visible impact, not implementation detail. Technical details in body.
  - `fix`: the problem solved. BAD: `utilise LEFT JOIN au lieu de subquery`. GOOD: `corrige l'absence de médias dans la liste`
  - `feat`: the capability added. BAD: `ajoute MergeService`. GOOD: `permet de fusionner des titres en double`
  - `refactor`/`chore`: the improvement. BAD: `extrait buildQuery`. GOOD: `simplifie la construction des requêtes de liste`
- Trailer: `Co-Built-By: Claude (<random funny quip>)` — vary each time
- Skip `git diff` when you made the edits. Merges: `--no-ff`.

## Language

Commits/docs: French. Code: English. CLAUDE.md: English.
