include .env
-include .env.local
export

DC = docker compose -f docker-compose.dev.yml
EXEC = $(DC) exec app

.PHONY: help up down logs shell test test-front lint lint-front fmt dev-frontend build migrate
.PHONY: import import-dry db-reset backfill-accents
.PHONY: ssh-import ssh-import-dry ssh-db-reset ssh-logs

# SSH helper: sources NAS_* from .env.local and runs a command over SSH
NAS_SSH = bash -c 'set -a && source <(grep "^NAS_" .env.local) && set +a && \
	sshpass -p "$$NAS_PASSWORD" ssh -p $$NAS_PORT $$NAS_USERNAME@$$NAS_HOST "$(1)"'

.DEFAULT_GOAL := help
help: ## Show this help
	@grep -E '(^[a-zA-Z_-]+:.*?##.*$$)|(^##)' Makefile | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[32m%-25s\033[0m %s\n", $$1, $$2}' | sed -e 's/\[32m##/[33m/'

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

test-front: ## Run frontend tests (unit + production build)
	$(EXEC) bash -c "cd frontend && npx vitest run && npx vite build"

lint: ## Run Go linter
	$(EXEC) golangci-lint run ./...

lint-front: ## Run frontend type check
	$(EXEC) bash -c "cd frontend && npx tsc"

fmt: ## Format Go code
	$(EXEC) gofmt -w .

dev-frontend: ## Start Vite dev server (inside container)
	$(EXEC) bash -c "cd frontend && npx vite --host 0.0.0.0"

build: ## Build production binary
	$(EXEC) go build -tags sqlite_fts5 -o ./tmp/plextracker .
migrate: ## Run database migrations
	$(EXEC) ./tmp/plextracker migrate

# ── Local (dev) ───────────────────────────────────

import: ## Import Simkl backup locally (BACKUP_FILE=path)
	$(EXEC) ./tmp/plextracker import $(BACKUP_FILE)

import-dry: ## Dry-run Simkl import locally (BACKUP_FILE=path)
	$(EXEC) ./tmp/plextracker import --dry-run $(BACKUP_FILE)

backfill-accents: ## Backfill accent_hex sur tous les titres avec cover (idempotent sauf FORCE=1)
	$(EXEC) ./tmp/plextracker backfill-accents $(if $(FORCE),--force,)

db-reset: ## Reset la BDD locale (supprime + restart pour re-migrer)
	$(EXEC) sh -c 'rm -f $${DATA_DIR}/plextracker.db'
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

ssh-db-pull: ## Pull la BDD du NAS vers le local (nettoie le local avant)
	@echo "Arrêt de l'application locale..."
	-$(DC) stop app
	@echo "Nettoyage de la base de données locale..."
	rm -f data/plextracker.db data/plextracker.db-wal data/plextracker.db-shm
	@echo "Téléchargement de la base de données depuis le NAS (db + wal + shm)..."
	@bash -c 'set -a && source <(grep "^NAS_" .env.local) && set +a && \
		NAS_DB_PATH=/volume1/docker/plextracker/data/plextracker.db && \
		sshpass -p "$$NAS_PASSWORD" scp -O -P $$NAS_PORT $$NAS_USERNAME@$$NAS_HOST:$$NAS_DB_PATH data/plextracker.db; \
		sshpass -p "$$NAS_PASSWORD" scp -O -P $$NAS_PORT $$NAS_USERNAME@$$NAS_HOST:$${NAS_DB_PATH}-wal data/plextracker.db-wal 2>/dev/null || true; \
		sshpass -p "$$NAS_PASSWORD" scp -O -P $$NAS_PORT $$NAS_USERNAME@$$NAS_HOST:$${NAS_DB_PATH}-shm data/plextracker.db-shm 2>/dev/null || true'
	@echo "Démarrage de l'application locale..."
	$(DC) start app
	@echo "Database pulled from NAS and local app started"

ssh-logs: ## Pull logs du conteneur plextracker du NAS vers data/plextracker.log (LINES=all par défaut)
	@mkdir -p data
	@echo "Téléchargement des logs depuis le NAS..."
	@$(call NAS_SSH,/usr/local/bin/docker logs $${LINES:+--tail $${LINES}} plextracker 2>&1) > data/plextracker.log
	@echo "Logs enregistrés dans data/plextracker.log ($$(wc -l < data/plextracker.log) lignes)"

ssh-db-push: ## Push la BDD locale vers le NAS (ATTENTION: écrase la BDD de prod)
	@bash -c 'set -a && source <(grep "^NAS_" .env.local) && set +a && \
		sshpass -p "$$NAS_PASSWORD" scp -P $$NAS_PORT data/plextracker.db $$NAS_USERNAME@$$NAS_HOST:/volume1/docker/plextracker/data/plextracker.db'
	@$(call NAS_SSH,/usr/local/bin/docker restart plextracker)
	@echo "Database pushed to NAS and container restarted"
