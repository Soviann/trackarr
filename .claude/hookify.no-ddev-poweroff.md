---
name: no-ddev-poweroff
enabled: true
event: bash
pattern: ddev\s+poweroff
action: block
---

**`ddev poweroff` blocked** — it stops ALL DDEV projects, not just this one. Never run without explicit user permission.
