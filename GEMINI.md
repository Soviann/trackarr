# GEMINI.md

@CLAUDE.md

## Agent Bridge

| Claude term | Gemini Mapping |
|---|---|
| `MEMORY.md` | Ignore (Claude-only absolute path) |
| `EnterPlanMode` | Use `enter_plan_mode` tool |
| `TaskCreate`/`Update` | Use `tracker_create_task` tool |
| `superpowers:*` | Use `activate_skill` for `.agents/skills/` |
| Visual Verification | Use Chrome DevTools MCP or skip |
| Commit Trailer | `Co-Built-By: Gemini (<funny quip>)` |

## Strategy

- **Token Efficiency**: Use `activate_skill` for task-specific instructions (commit, release, etc.). Do not preload skills.
- **Language**: French for commits/docs. English for code.
- **Tools**: Makefile (Docker) for all dev tasks.
