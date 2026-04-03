# Implement GitHub Issue

**IMPORTANT:** When this skill loads, create a task (TaskCreate) for EACH step below and complete them in order. Mark each task `in_progress` before starting it and `completed` when done. No step may be skipped silently.

1. **Gather context from memory first**: read `docs/patterns.md` for project conventions, file paths, and patterns. If no issue number was specified, pick the highest-priority Todo from the board list in `MEMORY.md` (Urgent > High > Medium > Low, then lowest issue number). Use this to identify the target issue and understand the codebase. Fall back to GitHub (issue details, comments, related PRs) only if memory is insufficient or more info is needed.
2. **Evaluate complexity — plan mode if needed**: After gathering context, assess whether the issue is straightforward (clear scope, existing patterns to follow) or complex (feasibility study, architectural decisions, multiple valid approaches, unclear requirements). If complex → enter plan mode (`EnterPlanMode`) and get user approval before coding. If straightforward → proceed directly.
3. Update local main (`git checkout main && git pull`), then create a feature branch from main (follow CLAUDE.md branch naming convention). Do not push yet — push with the actual code in step 10.
4. Move the GitHub issue on the project board to In Progress.
5. Follow TDD using the `superpowers:test-driven-development` skill: write failing tests → implement → all tests pass → refactor.
6. Run `superpowers:verification-before-completion`: all tests pass, lint clean (including PHP CS Fixer on staged `.php` files).
7. Update CHANGELOG.md with the change.
8. If the change affects deployment, CLI commands, or documented APIs, grep `docs/` for impacted terms and update relevant docs.
9. Update `docs/patterns.md` with any new entities, enums, services, hooks, components, pages, or routes added. Skip if nothing new.
10. Provide the user a quick summary of what's been done and ask for approval before resuming.
11. Commit and push.
12. Request code review using `superpowers:requesting-code-review`. Fix review comments using `superpowers:receiving-code-review`. Repeat until review is clean. If review modified code, run linters and tests until everything is clean.
13. Create a PR via `gh pr create` (only after review is clean).
14. Wait for CI to pass (`gh pr checks <number> --watch`), then merge: `gh pr merge <number> --squash`. Only proceed to next point when PR is merged.
15. Clean up: switch back to `main`, pull, and delete the local feature branch (`git branch -d`).
