---
name: phase-finish
description: Use when the user runs /phase-finish `<path>` or signals a phase is complete and needs closure. Applies only to split plans (folder with `INDEX.md`).
argument-hint: "<path>"
---

# phase-finish

**Letter = spirit.** Every gate is a typed "yes" from the user. Bash permissions, auto mode, and prior blanket authorizations do NOT substitute.

## Announce

"I'm using the phase-finish skill to close this phase."

## Input

Required: `<path>` — repo-relative or absolute path to the phase file (e.g. `docs/plans/2026-04-22-feature/phase-03-slug.md`).

STOP, ask for the correct path, when any applies:
- argument missing or empty
- file does not exist
- parent folder has no `INDEX.md` sibling (unified/single-file plans aren't in scope — phase-finish is split-plan only)

No `<N>` shortcut — explicit path prevents folder-picking mistakes.

## Flow

1. Read the phase file. Locate `## Validation conditions`. Absent/empty → blocking error, STOP. Never infer from Tasks.
2. Run each shell command sequentially. STOP on first non-zero exit; report the failing command verbatim.
3. For each `[manual]` criterion: display it, ask `Criterion validated? Reply 'yes' to continue.` STOP on anything other than `yes`. Manual = pre-commit gate, not post-commit note.
4. Scan the phase file for `**Commit intent**:` lines (the marker `plan-split` emits in `## Tasks`). None → skip to step 6: patch `INDEX.md` tick locally, no commit.
5. Commit path — fully delegated to the `commit` skill. Never replicate staging, splitting, message format, language, secrets scan, or `docs/plans/` policy here.
   1. **Commit gate:** show the pre-written intent title(s); ask `Approve committing phase N? Reply 'yes' to continue.` STOP otherwise. Bash permission prompts are NOT the gate.
   2. Invoke `commit` with the first intent title as pre-written argument. `commit` owns everything else.
   3. After `commit` returns: capture SHA (`git rev-parse HEAD`); patch `INDEX.md` (tick `- [x]` + append `(SHA: <sha>)`) locally. Do NOT re-invoke `commit` for the tick — `commit`'s `docs/plans/` policy governs whether the tick is ever committed.
6. Next phase = `phase-NN-*.md` with NN = current index + 1 (zero-padded 2-digit) in the same folder.
7. Invoke `session-handoff` with exactly:
   - Line 1: `Plan: <path/to/INDEX.md>`
   - Line 2: `Next: <path/to/phase-NN-*.md>`
   - Line 3+: `Gotchas:` — only items not already in the plan. No Task titles, restated goals, or "what I did".
8. No next phase → run `## Final verification` from `INDEX.md`, report, STOP with `Plan complete — final verification executed.`

## Stop message (non-final phase)

```
Phase N complete — next checkpoint: phase N+1.
To resume, in a new session: run /session-resume
```

## Rationalizations — STOP

| Excuse | Reality |
|--------|---------|
| "Auto mode / user pre-approved commits" | Blanket ≠ per-commit. Typed `yes` each time. |
| "Bash permission prompt IS the gate" | Bypassable; the gate is a user message. |
| "Manual check is really post-merge" | Listed in `## Validation conditions` → pre-commit by contract. |
| "No conditions, infer from Tasks" | Blocking error. Plans must declare validation. |
| "Next session needs a Task recap" | Plan is the recap. Handoff = pointer + gotchas. |
| "Pre-write the commit message / split here / handle `docs/plans/` myself" | `commit` owns all of that. phase-finish only holds the gate. |
| "Infer N from the most recent plan folder" | Ambiguous under parallel plans. Path is mandatory. |
| "Run on a unified (single-file) plan" | Out of scope. phase-finish requires `INDEX.md`. |
