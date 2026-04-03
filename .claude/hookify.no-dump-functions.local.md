---
name: no-dump-functions
enabled: true
event: file
pattern: \b(fmt\.Print(ln|f)?|log\.Print(ln|f)?)\s*\(.*(?:debug|dump|TODO|FIXME)
action: block
---

**Debug print statements are forbidden in committed code**

Temporary `fmt.Println` / `log.Println` for debugging must not be written to files.
Use structured logging (`log.Printf` with context) or remove before committing.
