---
name: commit
description: Creates clean, well-scoped git commits with French conventional messages. Handles staging, commit splitting, message formatting, and secrets/safety checks. Use when the user runs /commit, asks to commit, or when uncommitted changes need saving after completing work.
argument-hint: "[title]"
allowed-tools: Bash(git add *) Bash(git commit *) Bash(git status *) Bash(git diff *) Bash(git log *) Bash(git branch *) Bash(git reset *)
---

# Commit

## Steps

**1. Gather context** — parallel:
- `git status`
- `git branch --show-current` (scope)
- `git log --oneline -5` (style)
- `git diff HEAD` only if you did NOT make the changes yourself

**2. Clean staging** — `git reset HEAD -- . 2>/dev/null` (unstages without losing work; prevents leftover staged files).

**3. Secrets check** — scan for `.env`, credentials, API keys, tokens. If found: **warn the user**, let them decide. Don't silently skip or block.

**4. Evaluate scope** — one commit or several? Split signals: unrelated features, bug fix + new feature, config + business logic, CLAUDE.md + code.

If multiple groups, **propose a split before staging**:
> 1. **fix(SIQ-123): ...** — `file1.php`, `file2.php`
> 2. **feat(SIQ-123): ...** — `file3.php`, `template.twig`
> 3. **chore(claude): ...** — `CLAUDE.md`
> Commit separately or all at once?

Flag hunk-level staging (`git add -p`) when a single file has changes for different commits.

**5. Stage and commit** — only this commit's files/hunks, always via HEREDOC:
```bash
git add file1.php file2.php && git commit -m "$(cat <<'EOF'
type(scope): titre

Détails techniques optionnels.
EOF
)"
```

**6. Repeat** for remaining groups if splitting.

## Message format: `type(scope): title`

**Types:** `feat` (new capability) · `fix` (bug) · `chore` (maintenance/config/deps) · `refactor` (structure, no behavior change) · `docs` (documentation)

**Scope:** branch/ticket name for feature work (e.g. `SIQ-652`) · `claude` for CLAUDE.md · relevant module name otherwise. Never use ticket scope for non-ticket changes.

**Title = why** (effect/problem solved), **not what** (mechanics). Rewrite if it names a file/class/function/mechanism instead of the benefit. French 3rd-person imperative (`ajoute`, `corrige`, `supprime`).

| BAD | GOOD |
|-|-|
| `fix: utilise PATCH au lieu de PUT` | `fix: corrige la perte des tomes` |
| `feat: ajoute CoverSearchService` | `feat: ajoute la recherche de couvertures` |
| `refactor: extrait getFieldPriority` | `refactor: simplifie la résolution de priorité` |
| `chore: supprime le hook SessionStart` | `chore: évite une reprise de session déclenchée trop tard` |

**Body:** optional, for technical details/reasoning. On title rejection retry: fix only the title, always preserve and adapt the body.

## Rules

- Never commit `docs/plans/` — run `git reset docs/plans/` if present
- No `Co-Authored-By` trailer
- `--no-ff` for merge commits
- Include `reference.php` when changed after `composer require/update`
