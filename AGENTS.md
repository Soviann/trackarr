# AGENTS.md — Mandatory rules

<!-- TEMPLATE:START — managed by sync-template-config.
     Cross-project rules only: approach, plans process, quality, token optimization,
     git workflow, language. Anything language- or framework-specific (linters,
     naming, translation systems, DTO patterns, framework idioms, language-specific
     plugins) belongs in the PROJECT block, not here.
     If unsure whether a rule belongs here or in PROJECT, ask the user before adding. -->

## Approach
- Surface uncertainty: ask when unsure, name competing interpretations, flag simpler alternatives instead of picking silently. Read only files you'll edit.
- Key context: AGENTS.md, MEMORY.md, `docs/patterns.md` — update `docs/patterns.md` with anything future sessions would need Explore/Grep to rediscover.
- AGENTS.md and `docs/patterns.md` must be optimized for LLM use and token efficiency, without loss of information/instruction.
- Complex tasks: plan → approval → implement. Large changes: verifiable chunks.
- Act on user instructions directly — no exploratory glob/grep when user names the target.
- Don't verify existence of items already known from plan or memory.

## Plans
- Path defined per project (see PROJECT block). Plans are temporary unless the project rules say otherwise. Never commit unless the project explicitly retains them.
- Generation: `superpowers:writing-plans` (single plan file; no phase splitting).
- Execution: `phased-development:run-plan <path>` — drives the plan task-by-task. (The old `plan-split`/`plan-exec`/`phase-finish` skills were removed in phased-development 0.6.0.)

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

## Coding Standards
- Existing files: fix only your changes — no drive-by reformatting.
- No magic strings: constants or enums for domain values reused across files.
- Language-specific rules (linters, naming, framework idioms, translation systems, DTO patterns) belong in the PROJECT block.

## Git
- All commits go through `/commit` (commit-conventions plugin from soviann-tools) — the skill owns format, splitting, and safety checks.
- Merges: `--no-ff`

## Language
LLM-destined files (AGENTS.md, SKILL.md, plan/spec files, agent files): English. Everything else (commits, docs/comments, user-facing strings): per project preference (see PROJECT block).

## Recommended Plugins
`soviann-tools/*` (commit-conventions, parsing-decisions, phased-development, session-continuity, soviann-conventions) — default workflow. Plus: `context7`, `superpowers`, `pr-review-toolkit`, `hookify`, `code-simplifier`.

<!-- TEMPLATE:END -->

<!-- PROJECT:START -->

## Project
PlexTracker — Personal media tracking app. Go 1.24 / SQLite / chi / Preact 10 / Vite / Docker. Single developer.

## Approach
- Always prefer the robust/correct solution over the lazy/easy one. If trade-offs exist, present them as a PO: user-visible impact, not implementation effort.
- Update `docs/patterns.md` when adding routes/services/components/commands. Update the relevant topic doc in `docs/` (see `docs/INDEX.md`) when adding user-facing features.

## Plans
Location: `docs/superpowers/plans/` (gitignored, local-only). Plans are throwaway working notes — never commit, no `done/` archive. Git history is the durable record.
Overrides `superpowers:writing-plans`: save via Write tool to `docs/superpowers/plans/`, never to `~/.claude/` or any global path.

## Audits
`docs/audits/YYYY-MM-DD.md` (active) → `docs/audits/done/` (completed). Work top-to-bottom by session; mark items done inline (strikethrough); update file then commit per session.

## Commands
**All via Makefile (runs inside Docker).** Never `go`/`node`/`npm` on host. Host-only: `git`, `gh`, `docker`, `make`.

`up` `down` `logs` `shell` `test` `test-front` `lint` `fmt` `build` `dev-frontend` `migrate` `import BACKUP_FILE=...` `import-dry BACKUP_FILE=...`

## Deploy
`.github/workflows/deploy.yml` → SSHes NAS → `nas-update.sh`. Release: push `v*` tag (no CHANGELOG to maintain; release notes, when needed, come from `git log <prev-tag>..HEAD --oneline` or GitHub Releases auto-generation). Hotfix: `gh workflow run deploy.yml`.

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
After UI/UX changes, verify in-browser via `cmux browser --surface surface:7 ...` against `http://localhost:8080` (prod is `surface:28`; already logged in, never open a new browser surface): confirm changed screens, check console for errors, navigate other pages for regressions. If broken: fix → re-verify. Never claim done without visual confirmation. The PWA service worker caches aggressively — append `?t=$(date +%s)` to the URL to bust the cache after a frontend change. Frontend changes need air to re-embed `frontend/dist`: touch `main.go` after `make test-front` to force a Go rebuild.

## Git
- Trailer: `Co-Built-By: [Model Name] (<random funny quip>)` — vary each time (e.g. Gemini, Claude)

## Recommended Plugins
`cc-skills-golang`. Browser automation: cmux browser (no plugin needed; Chrome DevTools MCP is disabled).

<!-- PROJECT:END -->
