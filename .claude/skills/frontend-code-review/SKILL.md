---
name: frontend-code-review
description: "Review frontend files (.tsx, .ts, .js) against PlexTracker conventions. Use before committing frontend changes, when reviewing staged diffs, or when the user asks to review named .tsx/.ts/.js files."
---

# Frontend Code Review

Review frontend code against these checklists:

- [references/code-quality.md](references/code-quality.md) — code quality rules
- [references/performance.md](references/performance.md) — performance rules
- [references/business-logic.md](references/business-logic.md) — business logic rules

## Process

1. Read the target files (staged changes or named files)
2. Check each rule from the three reference files above
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
