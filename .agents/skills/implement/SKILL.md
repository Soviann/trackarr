# Implement GitHub Issue

**IMPORTANT:** Create a task (TaskCreate) for EACH step. Mark `in_progress` → `completed` sequentially.

1. **Gather context**: read `docs/patterns.md`. If no issue number specified, pick highest-priority Todo from board. Fall back to GitHub only if needed.
2. **Evaluate complexity**: straightforward → proceed. Complex (architectural decisions, unclear requirements) → `EnterPlanMode`, get approval.
3. Update local main (`git checkout main && git pull`), create feature branch.
4. Move GitHub issue to In Progress on project board.
5. **TDD** via `superpowers:test-driven-development`: failing tests → implement → green → refactor.
6. **Verify** via `superpowers:verification-before-completion`: `make test && make lint`.
7. Update `CHANGELOG.md`.
8. If change affects docs, update relevant files in `docs/`.
9. Update `docs/patterns.md` with new entities/routes/components. Skip if nothing new.
10. Summary to user → ask approval before proceeding.
11. Commit and push.
12. Code review via `superpowers:requesting-code-review`. Fix issues via `superpowers:receiving-code-review`. Repeat until clean. Re-run tests if code changed.
13. Create PR via `gh pr create` (only after review is clean).
14. Wait for CI: `gh pr checks <n> --watch`. Merge: `gh pr merge <n> --squash`.
15. Switch to `main`, pull, delete local feature branch.
