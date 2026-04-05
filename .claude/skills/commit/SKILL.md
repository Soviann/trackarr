---
name: commit
description: Create clean, well-scoped git commits with French conventional commit messages.
user_invocable: true
---

# Commit

## Steps

1. **Gather context** (parallel): `git status`, `git branch --show-current`, `git log --oneline -5`. Skip `git diff HEAD` if you made the changes.
2. **Clean staging**: `git reset HEAD -- . 2>/dev/null`
3. **Secrets check**: scan for `.env`, credentials, API keys. Warn user if found.
4. **Evaluate scope** — split if unrelated changes detected. Propose plan:
   > 1. **fix(scope): ...** — `file1.go`, `file2.go`
   > 2. **feat(scope): ...** — `component.tsx`
   > Commit separately or all at once?
5. **Stage and commit** via HEREDOC:
   ```bash
   git add file1.go file2.go && git commit -m "$(cat <<'EOF'
   type(scope): titre

   Détails techniques optionnels.
   EOF
   )"
   ```
6. **Repeat** for remaining groups.

## Message format

`type(scope): title` — French 3rd-person imperative.

**Types:** `feat` · `fix` · `chore` · `refactor` · `docs`

**Scope:** branch/ticket name for feature work · `claude` for CLAUDE.md · module name otherwise.

**Title = visible impact**, not implementation detail.

| BAD | GOOD |
|-|-|
| `fix: utilise PATCH au lieu de PUT` | `fix: corrige la perte des tomes` |
| `feat: ajoute CoverSearchService` | `feat: ajoute la recherche de couvertures` |

## Rules

- Trailer: `Co-Built-By: Claude (<random funny quip>)` — vary each time
- `--no-ff` for merge commits
- Never commit `docs/superpowers/plans/`
