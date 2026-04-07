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

## Workflow

- **Robust Commit Protocol**: always use a temporary file for multi-line commit messages to avoid quoting errors with `run_shell_command`:
  1. Write the message to `.gemini_commit_msg.txt` via `write_file`.
  2. Execute `git commit -F .gemini_commit_msg.txt`.
  3. Delete the temporary file.
