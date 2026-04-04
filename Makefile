DC = docker compose -f docker-compose.dev.yml
EXEC = $(DC) exec app

.PHONY: help up down logs shell test test-front lint fmt dev-frontend build migrate import import-dry

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

import: ## Import Simkl backup (BACKUP_FILE=path)
	$(EXEC) ./tmp/plextracker import $(BACKUP_FILE)

import-dry: ## Dry-run Simkl import
	$(EXEC) ./tmp/plextracker import --dry-run $(BACKUP_FILE)
