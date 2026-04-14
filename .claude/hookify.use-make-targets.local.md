---
name: use-make-targets
enabled: true
event: bash
pattern: ^\s*(go|npm|node|ssh|sshpass)\s+
action: block
---

**Use or create a Makefile target — never run these commands directly.**

- `go`, `npm`, `node` → run inside Docker via `make`
- `ssh`, `sshpass` → credentials come from `.env.local` via `make`; never guess or hardcode access credentials

**If the right target doesn't exist: add it to the Makefile (read `.env.local` for credentials), then call `make <target>`.**

Allowed on host: `git`, `gh`, `docker`, `make`

Existing targets: `up` `down` `logs` `shell` `test` `test-front` `lint` `fmt` `build` `dev-frontend` `migrate` `import` `import-dry`
