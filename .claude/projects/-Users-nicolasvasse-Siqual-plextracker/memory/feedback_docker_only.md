---
name: Docker-only commands
description: All dev commands (go, node, npm) must run inside the Docker container, never on host
type: feedback
---

Never run `go`, `node`, `npm`, `npx`, `golangci-lint` directly on the host machine. Always use the Makefile which wraps `docker compose exec app ...`.

**Why:** User doesn't have Go installed on host and explicitly requested container-only execution. Consistency matters — if it works locally it works in CI.

**How to apply:** Use `make test`, `make lint`, `make shell`, etc. for all development commands. Only `git`, `gh`, `docker`, `docker compose`, and `make` run on host.
