---
name: commit-title-visible-impact
enabled: true
event: bash
pattern: git\s+commit\s
action: block
---

**STOP — Review your commit title.**

The title MUST describe **visible impact**, NOT implementation details. See CLAUDE.md § Git for rules and examples.

Self-check:
- Mentions a class/method/variable name? → REWRITE
- Describes HOW code changed internally? → REWRITE
- Describes WHAT the user/system gains? → OK
