# CLAUDE.md — Mandatory rules

<!-- TEMPLATE:START — do not edit manually -->

## Approach
- Act over ask; read only files you'll edit.
- Key context: CLAUDE.md, MEMORY.md, `docs/patterns.md`.
- CLAUDE.md and `docs/patterns.md` must be optimized for LLM use and token efficiency, without loss of information/instruction.
- Complex tasks: plan → approval → implement. Large changes: verifiable chunks.
- Act on user instructions directly — no exploratory glob/grep when user names the target.
- Don't verify existence of items already known from plan or memory.

## Plans

### Format
- **PO summary first**: 2-3 sentences max, non-technical, describing what changes for the user/product.
- **Then phases**: phase N depends on N-1 output. Within each phase, mark tasks `[parallel]` or `[seq]`.
- No prose — actionable instructions only. Each task: target files, method signatures/behavior, test criteria.
- All architectural decisions made in plan. No "decide how to…" — Sonnet executes, doesn't architect.

### Sonnet-readiness (per task)
- Target files listed with expected changes
- Input/output or behavior defined
- Test criteria explicit (what to assert)

### Execution model
- Phase with independent tasks → `subagent-driven-development` (parallel Sonnet sub-agents)
- Phase with dependencies → single sequential agent
- Mechanical same-change × N → parallel sub-agents
- One session per phase. Plan encodes execution — no orchestrator needed.
- **Stop after plan creation.** Never start execution in the same session — present the plan, wait for approval, stop. Execution happens in a new session.

## Quality
- Linters/formatters: before committing only, ONLY on modified files.
- DRY: extract at 3+ occurrences (or 2 if complex). Exception: abstraction obscures intent or coupling > duplication cost.
- Prefer native/library solutions over custom code.

## Token Cost Optimization
- **Parallelize** independent tool calls in one turn. Serialize only when B depends on A's output.
- **Combine Bash** with `&&` when output-independent. Don't combine when you need A's output to decide B, or when failure diagnosis/timeouts differ.
- **No exploratory search when target is known.** Named file/symbol → Read/Grep directly. Skip verifying items known from plan/memory.
- **Direct tools > subagents.** Subagent = full Opus conversation billed on top. Use Grep/Glob/Read for known targets, single-file reads, <3 queries. Use subagents only for open-ended multi-file research or to protect main context.
- **Cheap models on mechanical subagents.** File search, simple refactor, mechanical lookup → `model: "haiku"` or `"sonnet"`.
- **`gh --json field1,field2`** over MCP GitHub tools for simple queries. `minimal_output: true` on MCP list/search calls.

## Git
- Format: `<type>(scope|branch-name): description` — types: `feat|fix|chore|refactor|docs`
- French descriptions: 3rd-person imperative (`ajoute`, `corrige`, `supprime` — not infinitive).
- Commit title = visible impact, not implementation detail. Technical details in body.
  - `fix`: problem solved. BAD: `utilise LEFT JOIN` GOOD: `corrige l'absence de médias`
  - `feat`: capability added. BAD: `ajoute MergeService` GOOD: `permet de fusionner des titres`
- Skip `git diff` when you made the edits — diff only to discover changes you didn't make.
- Merges: `--no-ff`

## Language
Commits + docs/comments: French. Code identifiers: English. CLAUDE.md: English.

## Recommended Plugins
`context7`, `superpowers`, `pr-review-toolkit`, `hookify`, `code-simplifier`.

<!-- TEMPLATE:END -->

<!-- PROJECT:START -->

## Project
PlexTracker — Personal media tracking app. Go 1.24 / SQLite / chi / Preact 10 / Vite / Docker. Single developer.

## Approach
- Always prefer the robust/correct solution over the lazy/easy one. If trade-offs exist, present them as a PO: user-visible impact, not implementation effort.
- Update `docs/patterns.md` when adding routes/services/components/commands. Update `docs/user-guide.md` when adding user-facing features.

## Plans
Location: `docs/superpowers/plans/` and `docs/superpowers/specs/`. Completed → `done/` subfolder. Move plan to `done/` before implementation is committed.
Overrides `superpowers:writing-plans`: save via Write tool to `docs/superpowers/plans/`, never to `~/.claude/` or any global path.
Versioning: git history is sufficient. Don't version-number files. Keep immutable once approved — if scope changes, add `## Revision — YYYY-MM-DD` header. Name descriptively (`notification-push.md` not `plan-007.md`).

## Audits
`docs/audits/YYYY-MM-DD.md` (active) → `docs/audits/done/` (completed). Work top-to-bottom by session; mark items done inline (strikethrough); update file then commit per session.

## Commands
**All via Makefile (runs inside Docker).** Never `go`/`node`/`npm` on host. Host-only: `git`, `gh`, `docker`, `make`.

`up` `down` `logs` `shell` `test` `test-front` `lint` `fmt` `build` `dev-frontend` `migrate` `import BACKUP_FILE=...` `import-dry BACKUP_FILE=...`

## Changelog
Update `CHANGELOG.md` after every meaningful change (feat, fix, perf, security). Add under `## [Unreleased]` as you go. Release = move Unreleased block to versioned heading + push `v*` tag.

## Deploy
`.github/workflows/deploy.yml` → SSHes NAS → `nas-update.sh`. Release: push `v*` tag. Hotfix: `gh workflow run deploy.yml`.

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
After UI/UX changes, verify in-browser via Chrome DevTools MCP: login with `DEBUG_LOGIN*` from `.env.local`, confirm changed screens, check console for errors, navigate other pages for regressions. If broken: fix → re-verify. Never claim done without visual confirmation.

## Git
- Trailer: `Co-Built-By: Claude (<random funny quip>)` — vary each time

## Recommended Plugins
`chrome-devtools-mcp`, `cc-skills-golang`.

<!-- PROJECT:END -->
