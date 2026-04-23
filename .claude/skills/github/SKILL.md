---
name: github
description: "GitHub operations via gh CLI — issues, PRs, CI runs, code review. Use when the user asks to view/merge PRs, check CI status, list or comment on issues, watch workflow runs, or trigger the deploy workflow."
---

# GitHub

Use `gh` CLI for GitHub operations. Repo: `Soviann/plextracker`.

## Conventions

- Always use `--json field1,field2` for structured output (cheaper than MCP tools)
- PR titles: French, visible impact (not implementation detail)
- Squash merge: `gh pr merge <n> --squash`
- CI check before merge: `gh pr checks <n> --watch`
- Deploy: push `v*` tag or `gh workflow run deploy.yml`

## Quick reference

```bash
gh issue list --json number,title,state
gh pr list --json number,title,state,mergeable
gh pr checks <n> --watch
gh pr view <n> --json title,body,additions,deletions
gh run list --limit 5 --json databaseId,status,conclusion
gh run view <id> --log-failed
```
