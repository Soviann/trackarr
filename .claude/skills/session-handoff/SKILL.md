---
name: session-handoff
description: Use when ending a session with unfinished work, when context is getting large, when the system warns about compression, or when the user asks to hand off
---

# Session Handoff

Write a structured handoff file so the next Claude Code session picks up seamlessly.

## When to Use

**User asks:** `/handoff`, "write a handoff", "save session state", "hand off"

**Proactive — you MUST offer** when ANY of these are true:
- System warns about context compression/compaction
- Conversation is very long (many tool calls) and work is incomplete
- User signals session end: "thanks", "that's all", "stopping here", "gotta go", "brb" (if work is incomplete)
- Work is left incomplete: open branch, failing tests, mid-implementation

Proactive offer (ask once, don't nag):
> "Want me to write a session handoff before we stop?"

**Do NOT write a handoff** if all work is committed, tests pass, and no next steps remain. Tell the user there's nothing to hand off.

## The Rule

**One file. One location. One template. Every time.**

- File: `.claude/session-handoff.md` (project root)
- Overwritten each time (git log is history)
- MUST NOT be committed (`.claude/` should be gitignored)

**This skill writes ONLY the handoff file.** Do not:
- Update MEMORY.md as part of the handoff
- Write plan files
- Output the handoff as a chat message instead of a file
- Split information across multiple locations

## Process

1. Gather state: `git branch --show-current`, open issues, what was done, what's pending
2. Draft the handoff using the template below
3. **Show to user** for review before writing
4. Write to `.claude/session-handoff.md` after approval

## Template

```markdown
# Session Handoff

**Date:** YYYY-MM-DD
**Branch:** branch-name (or: branch-a, branch-b if multiple)
**Issue:** #N, #M (if applicable)

## Accomplished
- What was completed this session

## In Progress
- What's started but not finished
- File paths, current state, what's left

## Next Steps
1. Prioritized list of what to do next

## Key Decisions
- **Decision:** rationale

## Gotchas & Constraints
- Surprising discoveries, quirks, limitations

## Blockers
- What's stuck and why
```

**Always present:** Accomplished, Next Steps. **Omit if empty:** In Progress, Key Decisions, Gotchas & Constraints, Blockers.

**Use these exact section names.** Do not rename, reword, or add sections. The template is strict so the future "read" skill can parse it reliably.

**Multi-branch:** list all active branches in header. Group In Progress and Next Steps by branch if work spans multiple.

## Scope Boundaries

If the user asks for a handoff AND something else (e.g., "write a handoff and save X to memory"), treat them as **separate actions**. Write the handoff file following this skill, then handle the other request separately. Never merge memory updates, plan files, or other artifacts into the handoff file.

## Writing Guidelines

- **Concise but complete** — fewer words per point, not fewer points
- **File paths over descriptions** — `src/Service/Foo.php:45` beats prose
- **Don't duplicate git** — state and intent, not commit-by-commit replay
- **No code snippets** — what and where, not how
- **No project context** — the next session has CLAUDE.md and memory already
