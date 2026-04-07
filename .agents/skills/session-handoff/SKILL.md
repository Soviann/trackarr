---
name: session-handoff
description: Use when ending a session with unfinished work, when context is getting large, when the system warns about compression, or when the user asks to hand off
---

# Session Handoff

Write `.claude/session-handoff.md` so the next session picks up seamlessly.

## When to use

- User asks: `/handoff`, "write a handoff", "save session state"
- **Proactive** (offer once): context compression warning, long conversation with incomplete work, user signals session end with work remaining
- **Don't write** if all work is committed, tests pass, and no next steps remain

## Process

1. Gather state: `git branch --show-current`, what was done, what's pending
2. Draft using template below
3. Show to user for review
4. Write to `.claude/session-handoff.md` after approval

## Template

```markdown
# Session Handoff

**Date:** YYYY-MM-DD
**Branch:** branch-name
**Issue:** #N (if applicable)

## Accomplished
- What was completed

## In Progress
- What's started but not finished (file paths, current state)

## Next Steps
1. Prioritized list

## Key Decisions
- **Decision:** rationale

## Gotchas & Constraints
- Surprising discoveries, quirks

## Blockers
- What's stuck and why
```

**Always present:** Accomplished, Next Steps. **Omit if empty:** other sections.

## Rules

- One file, overwritten each time. Never committed.
- Concise but complete — file paths over descriptions, no code snippets, no project context (CLAUDE.md covers that).
- Don't update MEMORY.md or write plan files as part of handoff.
