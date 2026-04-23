---
name: frontend-code-review
description: "Review frontend files (.tsx, .ts, .js) against PlexTracker conventions."
---

# Frontend Code Review

Review frontend code against the checklist in [references/](references/).

## Process

1. Read the target files (staged changes or named files)
2. Check each rule from code-quality.md, performance.md, business-logic.md
3. Report using the template below

## Output

### If issues found:
```
# Code review
Found <N> urgent issues:

## 1 <description>
FilePath: <path> line <line>
### Suggested fix
<fix>

---

Found <M> suggestions:
## 1 <description>
...
```

### If clean:
```
## Code review
No issues found.
```

If urgent issues require code changes, ask: "Want me to apply these fixes?"
