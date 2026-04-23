---
name: plan-split
description: Use when the user runs /plan-split, asks for a plan, or is about to call `writing-plans` directly.
---

# plan-split

**All steps required.** No direct `writing-plans` call. No silent "small enough".

## Announce

"I'm using the plan-split skill to generate a split plan."

## Thresholds

> Tune here if results drift. One place — no scattered equivalents.

**Cut a phase** when ANY apply:
- ~10–15 distinct files touched
- \> 500 cumulative lines, or any single file \> 300 lines
- Executor model changes mid-phase
- Parallel subagent dispatch required

**Dispatch `writing-plans` to a subagent** (Flow step 2) when ANY apply:
- Forecast from spec: ≥ 10 phases or ≥ ~800 lines of plan prose
- Observable: spec \> 50% of current context

Past these points, inline writing triggers mid-generation compaction. Dispatch keeps the driver lean; subagent returns a path, driver reads it at Gate 1.

## Flow

1. `superpowers:brainstorming` **only if** spec has open choices (undecided APIs, multiple shapes, unresolved scope). Skip when subsystems/outputs are named.
2. `superpowers:writing-plans` — path override is **mandatory**: `docs/plans/YYYY-MM-DD-<feature>.md`. Never let `writing-plans` pick its default location. If dispatch threshold hits → `Agent(subagent_type: "general-purpose")` with that path override + spec pointer + "draft flat file only; do not split". Subagent returns path; driver reads at Gate 1.
3. **Gate 1 — unified draft.** Present, wait typed "yes". STOP otherwise.
4. **Phase load review.** Apply thresholds per phase; propose re-split if exceeded, wait approval. Repeat until clean.
5. < 2 phases → keep unified file; tell user "Run via `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans`"; STOP.
6. ≥ 2 phases → present target tree `docs/plans/YYYY-MM-DD-<feature>/INDEX.md` + `phase-NN-<slug>.md` per phase (NN = zero-padded 2-digit index: `01`, `02`, …, `10`, …). **Gate 2 — split proposal.** Wait typed "yes". STOP otherwise.
7. Create folder; delete monolithic draft (no half-state).
8. `INDEX.md` from `templates/INDEX.md`. `## Dispatch` pins plan-level pattern. `## How to resume` tells cold sessions what to read. Per-phase overrides live in phase `## Execution override` — never restate INDEX default there.
9. Each `phase-NN-<slug>.md` from `templates/phase.md`:
   - `- [ ]` atomic steps (per `writing-plans`). No prose in tasks.
   - Shared scaffolding **references** `conventions.md#<anchor>`. Never copy.
   - Omit `## Execution override` unless phase deviates from `INDEX.md#dispatch`.
   - \> 10 phases: emit phase files across multiple turns (5–8 per turn). One giant turn triggers mid-generation compaction.
10. `conventions.md` from `templates/conventions.md` when ≥ 2 phases share a rule/command (pre-commit block, hygiene grep, cross-phase deps). Shared = extract. Not a size gate. Commit format is NOT extracted — owned by `.claude/skills/commit/`.
11. **Self-check** before STOP:
    - Every phase link in INDEX resolves.
    - No `<placeholder>` strings remain.
    - Every phase has `## Validation conditions`.
    - No phase's `## Execution override` restates INDEX default.
12. Show final tree. Tell user: "Run via `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans`, starting from `phase-01-*.md`." STOP.

Non-"yes" (silence, "looks ok", "continue", "later") is NOT approval.

## Red flags — STOP

| Trigger | Reality |
|---------|---------|
| "Skip writing-plans / skip a gate / auto mode covers it" | Step 2 is the gate. Each typed "yes" stands alone. |
| "Splitting breaks tracker/resume" | Tracker in `INDEX.md`; handoff points to phase file; `## How to resume` guides cold sessions. |
| "One phase enough" / "Merge 2 smalls" / "Coordination > benefit" | Run thresholds. Reloading a monolith per phase costs more. |
| "Draft INDEX + phases directly, skip unified" | Unified → Gate 1 → split. No shortcut. |
| "Write N phases inline to skip dispatch" | Mid-generation compaction. |
| "Restate INDEX dispatch in every phase" | `INDEX.md#dispatch` is single source. `## Execution override` = deviations only. |
| "Copy pre-commit block per phase" | Lives in `conventions.md`. Phase references. |
| "Prose tasks" | `- [ ]` atomic per writing-plans. |
