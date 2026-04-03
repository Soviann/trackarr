---
name: require-docker-exec
enabled: true
event: bash
action: block
conditions:
  - field: command
    operator: regex_match
    pattern: ^(?!git\s)(?!gh\s)(?!docker\s)(?!make\s)[\s\S]*(?:\bgo\s+(?:build|test|run|get|mod|vet|fmt)\b|\bnpm\s|\bnpx\s|\bnode\s|\bgolangci-lint\b)
  - field: command
    operator: not_contains
    pattern: docker compose
---

**Command outside Docker container detected!**

`go`, `npm`, `npx`, `node`, `golangci-lint` must be run inside the Docker container.

**Use:**
- `make test` / `make lint` / `make fmt` / `make build`
- `make shell` then run commands interactively
- `docker compose -f docker-compose.dev.yml exec app <cmd>`
- Or Makefile targets for common operations
