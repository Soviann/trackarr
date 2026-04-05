include .env
-include .env.local
export

DC = docker compose -f docker-compose.dev.yml
EXEC = $(DC) exec app

.PHONY: help up down logs shell test test-front lint fmt dev-frontend build migrate
.PHONY: import import-dry db-reset
.PHONY: ssh-import ssh-import-dry ssh-db-reset

# SSH helper: sources NAS_* from .env.local and runs a command over SSH
NAS_SSH = bash -c 'set -a && source <(grep "^NAS_" .env.local) && set +a && \
	sshpass -p "$$NAS_PASSWORD" ssh -p $$NAS_PORT $$NAS_USERNAME@$$NAS_HOST "$(1)"'

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

up: ## Start dev environment
	$(DC) up -d --build

down: ## Stop dev environment
	$(DC) down

logs: ## Tail logs
	$(DC) logs -f

shell: ## Shell into container
	$(EXEC) bash

test: ## Run all Go tests
	$(EXEC) go test -tags sqlite_fts5 ./... -v -count=1

test-front: ## Run frontend tests
	$(EXEC) bash -c "cd frontend && npx vitest run"

lint: ## Run Go linter
	$(EXEC) golangci-lint run ./...

fmt: ## Format Go code
	$(EXEC) gofmt -w .

dev-frontend: ## Start Vite dev server (inside container)
	$(EXEC) bash -c "cd frontend && npx vite --host 0.0.0.0"

build: ## Build production binary
	$(EXEC) go build -tags sqlite_fts5 -o plextracker .

migrate: ## Run database migrations
	$(EXEC) ./tmp/plextracker migrate

# ── Local (dev) ───────────────────────────────────

import: ## Import Simkl backup locally (BACKUP_FILE=path)
	$(EXEC) ./tmp/plextracker import $(BACKUP_FILE)

import-dry: ## Dry-run Simkl import locally (BACKUP_FILE=path)
	$(EXEC) ./tmp/plextracker import --dry-run $(BACKUP_FILE)

db-reset: ## Reset la BDD locale (supprime + restart pour re-migrer)
	$(EXEC) rm -f /data/plextracker.db
	$(DC) restart app

# ── NAS (via SSH) ─────────────────────────────────

ssh-import: ## Import Simkl backup sur le NAS (BACKUP_FILE=filename in /volume1/downloads)
	@$(call NAS_SSH,/usr/local/bin/docker cp /volume1/downloads/$(BACKUP_FILE) plextracker:/tmp/$(BACKUP_FILE) && \
		/usr/local/bin/docker exec plextracker plextracker import /tmp/$(BACKUP_FILE) && \
		/usr/local/bin/docker exec plextracker rm /tmp/$(BACKUP_FILE))

ssh-import-dry: ## Dry-run Simkl import sur le NAS (BACKUP_FILE=filename in /volume1/downloads)
	@$(call NAS_SSH,/usr/local/bin/docker cp /volume1/downloads/$(BACKUP_FILE) plextracker:/tmp/$(BACKUP_FILE) && \
		/usr/local/bin/docker exec plextracker plextracker import --dry-run /tmp/$(BACKUP_FILE) && \
		/usr/local/bin/docker exec plextracker rm /tmp/$(BACKUP_FILE))

ssh-db-reset: ## Reset la BDD du NAS (supprime + restart pour re-migrer)
	@$(call NAS_SSH,/usr/local/bin/docker exec plextracker rm /data/plextracker.db && \
		/usr/local/bin/docker restart plextracker)
