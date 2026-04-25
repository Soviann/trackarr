---
name: maintain
description: "Project maintenance: modernization (Go 1.24, SQLite) and tech debt tracking. Use when auditing tech debt, modernizing Go/SQLite code, or checking that docs match implementation."
---

# Maintain

Manual-trigger skill for project health and modernization.

## Modernization (Go 1.24 & SQLite)

| Focus | Action |
|---|---|
| **Go 1.24** | Check for generic type aliases, omitempty in types, Go 1.23+ iterators. |
| **SQLite** | Optimize queries (JSON1, CTEs), check for WAL/MaxOpenConns=1 consistency. |
| **Deps** | Scan for outdated modules: `make shell` -> `go list -u -m all`. |

## Technical Debt

- Identify TODOs and temporary hacks.
- Verify test coverage on recent modules (`make test`).
- Ensure `docs/patterns.md` matches current implementation.
- Ensure `docs/user-guide.md` reflects current user-facing behavior.

## Release Prep

Releases ship via `git tag v* && git push --tags`. There is no curated changelog — release notes, when needed, come from `git log <prev-tag>..HEAD --oneline` (conventional commits make this readable).

## Token Efficiency Rules

- **Summary first**: Always provide a high-level audit summary before reading files.
- **Security Audit**: Use `osv-scanner` only if explicitly requested. Read summaries only.
- **Manual only**: Never apply maintenance changes without explicit approval.
