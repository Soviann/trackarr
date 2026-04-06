---
name: session-resume
description: Use at session start when .claude/session-handoff.md exists in the project — reads prior session state and asks the user how to proceed
---

# Session Resume

Read a handoff file from a previous session and help the user decide what to do next.

## When to Use

**At session start**, check if `.claude/session-handoff.md` exists in the project root. If it does, this skill activates. If it doesn't, do nothing.

## Process

1. **Read** `.claude/session-handoff.md`
2. **Present the full content** to the user — include every section, especially Gotchas & Constraints and Key Decisions (these are the most likely to be lost). Keep it concise but complete.
3. **Ask**: "Continue from where we left off?"
4. **If yes**: check out the branch from the handoff, delete `.claude/session-handoff.md`, then start working on the first Next Step
5. **If no**: ask "Want me to delete the handoff file, or keep it for later?" — then act accordingly

## Rules

- **Never skip details.** The handoff was written to be token-efficient already. Present all of it.
- **Always ask before acting.** Don't start working on Next Steps until the user confirms.
- **Always clean up.** Delete the handoff file after the user acknowledges it (whether continuing or explicitly discarding). A stale handoff file will confuse future sessions.
- **Don't re-summarize.** The handoff is already structured. Present it as-is or with minimal reformatting, don't rewrite it in your own words.
