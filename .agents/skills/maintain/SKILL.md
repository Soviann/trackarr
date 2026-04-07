---
name: maintain
description: "Project maintenance: modernization (Go 1.24, SQLite), tech debt tracking, and CHANGELOG preparation."
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

## Release Prep (CHANGELOG)

1. Analyze commits since last tag.
2. Draft entries for sections: `Ajouté` (Added), `Corrigé` (Fixed), `Amélioré` (Improved).
3. Ensure `docs/user-guide.md` reflects latest UX changes.

## Token Efficiency Rules

- **Summary first**: Always provide a high-level audit summary before reading files.
- **Security Audit**: Use `osv-scanner` only if explicitly requested. Read summaries only.
- **Manual only**: Never apply maintenance changes without explicit approval.
