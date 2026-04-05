---
name: technical-writer
description: "Create or update documentation following PlexTracker conventions."
---

# Technical Writer

## Conventions

- Language: French for docs/guides, English for code comments and CLAUDE.md
- Audience for plans/specs: PO (non-technical) — user-visible behavior, UX flows, acceptance criteria
- Audience for patterns.md: LLM — tables over prose, token-efficient
- Keep a Changelog format for `CHANGELOG.md` (French, sections: Ajouté/Corrigé/Amélioré/Supprimé)

## Files to update

| File | When | Audience |
|---|---|---|
| `docs/user-guide.md` | New user-facing feature | End user |
| `docs/patterns.md` | New route/service/component/command | LLM |
| `docs/openapi.yaml` | API change | Developer |
| `CHANGELOG.md` | Every meaningful change | User/PO |
| `docs/superpowers/plans/*.md` | New feature plan | PO |

## Style

- Tables > prose. Bullet points > paragraphs.
- No code snippets in plans/specs (PO audience).
- File paths as references, not inline code blocks.
- Progressive disclosure: summary first, details if needed.
