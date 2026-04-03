---
name: commit-title-visible-impact
enabled: true
event: bash
pattern: git\s+commit\s
action: block
---

**STOP — Review your commit title before proceeding.**

The commit title MUST describe **visible impact** (what changed for the user/system), NOT implementation details (code changes, class names, method names).

**Self-check the first line of your -m message:**
- Does it mention a class, method, function, or variable name? → REWRITE
- Does it describe HOW the code changed (internal mechanism)? → REWRITE
- Does it describe WHAT the user/system gains? → OK

**BAD (implementation detail):**
- `fix(comic): utilise PATCH au lieu de PUT`
- `feat(cover): ajoute CoverSearchService`
- `refactor(lookup): extrait getFieldPriority`

**GOOD (visible impact):**
- `fix(comic): corrige la perte des tomes à la modification`
- `feat(cover): ajoute la recherche de couvertures`
- `refactor(lookup): simplifie la résolution de priorité par champ`

If your title leaks implementation details, rewrite it now. Do NOT proceed with a bad title.
