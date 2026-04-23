# Phase N — <title>

**Plan:** [INDEX.md](./INDEX.md) — read first for Context, Dispatch, conventions.

## Goal

<1–2 sentences: what this phase delivers and why.>

## Target files

- Create: `<exact/path>`
- Modify: `<exact/path>:L<start>-L<end>`

## Tasks

> Task shape (TDD or not, number of steps) follows CLAUDE.md and `superpowers:writing-plans`. Atomic `- [ ]` steps with exact code and exact commands are mandatory; their ordering is task-specific. Non-behavior tasks (config, docs, mechanical refactor) drop test steps.

### N.1 — <title>

**Files:**
- Create: `<exact/path>`
- Modify: `<exact/path>:<line range>`
- Test: `<tests/path>` (omit if no test changes)

- [ ] **<action>**

  ```<lang>
  <exact code — no placeholders>
  ```

- [ ] **Run**: `<exact command>` → `<expected output/signal>`

- [ ] **<next atomic action>** — exact code or command.

- [ ] **Pre-commit block** — see `conventions.md#pre-commit-validation-block`

- [ ] **Commit intent**: `<type>(scope): description`
  Gated by `/phase-finish`.

### N.2 — <title>

<same shape — atomic `- [ ]` steps, exact code, exact commands. No prose.>

## Validation conditions

- `<phase-specific command>` → green
- [manual] <criterion>
- Shared checks: `conventions.md#final-phase-checks`

<Mandatory. `/phase-finish` reads this.>

## Execution override

<Optional — omit unless this phase deviates from `INDEX.md#dispatch`. One line, e.g. "Run implementers with sonnet (cross-cutting types)." Never restate INDEX default.>

## Handoff

Next: `phase-NN-<slug>.md` (NN = zero-padded 2-digit of next phase: 02, 03, …, 10, …)
Gotchas:
- <items NOT already in the plan; don't paraphrase Tasks>
