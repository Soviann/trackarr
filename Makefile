include .env
-include .env.local
-include Makefile.local
export

DC = docker compose -f docker-compose.dev.yml
EXEC = $(DC) exec app

.PHONY: help up down logs shell test test-front lint lint-front fmt dev-frontend build migrate version
.PHONY: import import-dry db-reset reset-import backfill-accents reset-password

.DEFAULT_GOAL := help
help: ## Show this help
	@grep -E '(^[a-zA-Z_-]+:.*?##.*$$)|(^##)' Makefile | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[32m%-25s\033[0m %s\n", $$1, $$2}' | sed -e 's/\[32m##/[33m/'

up: ## Start development environment
	$(DC) up -d --build

down: ## Stop development environment
	$(DC) down

logs: ## Tail logs
	$(DC) logs -f

shell: ## Shell into application container
	$(EXEC) bash

test: ## Run all Go unit tests
	$(EXEC) go test -tags sqlite_fts5 ./... -v -count=1

test-front: ## Run frontend tests (Vitest + production build)
	$(EXEC) bash -c "cd frontend && npx vitest run && npx vite build"

lint: ## Run Go linter (golangci-lint)
	$(EXEC) golangci-lint run ./...

lint-front: ## Run frontend type check (tsc) and i18n audit
	$(EXEC) bash -c "cd frontend && npx tsc && npm run lint:i18n"

fmt: ## Format Go source code
	$(EXEC) gofmt -w .

dev-frontend: ## Start Vite dev server (inside container with HMR)
	$(EXEC) bash -c "cd frontend && npx vite --host 0.0.0.0"

build: ## Build production binary
	$(EXEC) go build -tags sqlite_fts5 -o ./tmp/trackarr .

version: ## Show application version
	$(EXEC) ./tmp/trackarr version

migrate: ## Run database migrations
	$(EXEC) ./tmp/trackarr migrate

# ── Local Tools & Data Management ───────────────────────────────────

import: ## Import backup locally (BACKUP_FILE=path)
	$(EXEC) ./tmp/trackarr import $(BACKUP_FILE)

import-dry: ## Dry-run backup import locally (BACKUP_FILE=path)
	$(EXEC) ./tmp/trackarr import --dry-run $(BACKUP_FILE)

backfill-accents: ## Backfill poster dominant colors on titles (idempotent unless FORCE=1)
	$(EXEC) ./tmp/trackarr backfill-accents $(if $(FORCE),--force,)

reset-password: ## Reset admin password (PASSWORD=..., USERNAME=admin optional)
	$(EXEC) ./tmp/trackarr reset-password $(if $(PASSWORD),--password="$(PASSWORD)",) $(if $(USERNAME),--username="$(USERNAME)",)

db-reset: ## Reset local SQLite database (delete + restart to re-migrate)
	$(EXEC) sh -c 'rm -f $${DATA_DIR}/trackarr.db $${DATA_DIR}/trackarr.db-wal $${DATA_DIR}/trackarr.db-shm $${DATA_DIR}/plextracker.db $${DATA_DIR}/plextracker.db-wal $${DATA_DIR}/plextracker.db-shm'
	$(DC) restart app

reset-import: ## Reset local database then re-import backup (BACKUP_FILE=path)
	@test -n "$(BACKUP_FILE)" || { echo "BACKUP_FILE is required"; exit 1; }
	$(MAKE) db-reset
	@sleep 5
	$(MAKE) import BACKUP_FILE=$(BACKUP_FILE)
