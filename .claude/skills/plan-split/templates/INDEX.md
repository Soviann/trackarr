# <feature-title> — Plan (Index)

> Working doc. Siblings: `phase-NN-<slug>.md` (NN = zero-padded 2-digit) + `conventions.md` in `docs/plans/YYYY-MM-DD-<feature>/`. Commit policy → CLAUDE.md + `.claude/skills/commit/`.

## Context

<3–5 sentences: why this plan, triggering constraint, prior state, cross-cutting knowledge needed before opening any phase. Phase-specific context → phase files.>

## PO summary

<2–3 non-technical sentences: what changes for the user/product. Fast validation by the human partner.>

## How to resume

1. Read this INDEX in full.
2. Open Next phase file (from handoff, or `phase-01-*.md` if starting fresh).
3. Execute `## Tasks` in order per `## Dispatch` below — do **not** ask the user for model/mode.
4. Phase close → `/phase-finish <path-to-phase-file>`.

## Dispatch

<Plan-level execution pattern. Executing session MUST follow without prompting. Pick ONE line below, delete the other.>

- **subagent-driven** (`superpowers:subagent-driven-development`) — <why this fits the plan's size/shape>.
- **inline** (`superpowers:executing-plans`) — <why this fits the plan's size/shape>.

> Per-phase overrides (rare) in phase `## Execution override`. Absent = this default.

## Progress tracker

- [ ] Phase 1 — <title>
- [ ] Phase 2 — <title>
- [ ] Phase 3 — <title>

<Append `(SHA: ____)` when phase produces an intermediate commit. Omit for single-final-commit plans.>

## Target spec

<Optional — restructuring plans only. Declarative end state: file tree, mapping table, schema. Place before the phase list.>

## Commit discipline

<Plan-specific choices only: per-phase commit titles (if any), no-commit windows (broken intermediates), single final commit or per-phase. Rules (format, language, type-list, gating) → CLAUDE.md + `.claude/skills/commit/`. Don't restate.>

## Final verification

<End-to-end commands covering every touched repo + live smoke. Run after last phase closes. One fenced block.>

```
<commands>
```

## Phases

- Phase 1: [phase-01-<slug>.md](phase-01-<slug>.md)
- Phase 2: [phase-02-<slug>.md](phase-02-<slug>.md)
- Phase 3: [phase-03-<slug>.md](phase-03-<slug>.md)
- Shared conventions: [conventions.md](conventions.md)

<Omit `conventions.md` link only if no shared rules — rare.>
