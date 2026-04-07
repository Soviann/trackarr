# GEMINI.md

@CLAUDE.md

## Gemini-specific overrides

- Ignore references to `MEMORY.md`, `superpowers:*` skills, `TaskCreate`/`TaskUpdate`, `EnterPlanMode` — these are Claude Code features.
- "Chrome DevTools MCP" for visual verification: use browser tools available to you, or skip if unavailable.
- `Co-Built-By` trailer in commits: use `Co-Built-By: Gemini (<random funny quip>)` instead.

## Skills (shared with Claude Code)

These are workflow instructions stored in `.agents/skills/`. Read and follow them when the task matches.

@.agents/skills/commit/SKILL.md
@.agents/skills/release/SKILL.md
@.agents/skills/github/SKILL.md
@.agents/skills/technical-writer/SKILL.md
@.agents/skills/frontend-code-review/SKILL.md
@.agents/skills/session-handoff/SKILL.md
@.agents/skills/session-resume/SKILL.md
