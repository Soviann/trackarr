# GitHub Actions CI/CD — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the basic CI and ghcr-based release with a full pipeline: parallel CI, changelog-based GitHub Releases, and SSH deploy to Synology NAS with rollback and diagnostics.

**Architecture:** 3 GitHub Actions workflows (CI, Release, Deploy) + 2 NAS shell scripts (update with rollback, diagnostics). Deploy builds the Docker image directly on the NAS (no container registry). Diagnostics output is uploaded as GitHub artifact on failure.

**Tech Stack:** GitHub Actions, appleboy/ssh-action, Docker Compose, Bash

---

### Task 1: CI workflow — parallel jobs

**Files:**
- Modify: `.github/workflows/ci.yml`

- [ ] **Step 1: Replace ci.yml with 4 parallel jobs**

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:

jobs:
  test-backend:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: "1.24"

      - name: Run Go tests
        run: go test -tags sqlite_fts5 ./... -count=1

  lint-backend:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: "1.24"

      - name: Run golangci-lint
        uses: golangci/golangci-lint-action@v6
        with:
          version: latest

  build-frontend:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-node@v4
        with:
          node-version: "22"
          cache: npm
          cache-dependency-path: frontend/package-lock.json

      - name: Install dependencies
        run: cd frontend && npm ci

      - name: Build
        run: cd frontend && npx vite build

  test-frontend:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-node@v4
        with:
          node-version: "22"
          cache: npm
          cache-dependency-path: frontend/package-lock.json

      - name: Install dependencies
        run: cd frontend && npm ci

      - name: Run tests
        run: cd frontend && npx vitest run
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "refactor(ci): parallélise les jobs CI en 4 jobs indépendants"
```

---

### Task 2: Release workflow — changelog extraction

**Files:**
- Modify: `.github/workflows/release.yml`

- [ ] **Step 1: Replace release.yml with changelog-based GitHub Release**

```yaml
name: Release

on:
  push:
    tags: ["v*"]

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Extract changelog for this version
        id: changelog
        run: |
          TAG="${GITHUB_REF#refs/tags/}"
          NOTES=$(awk "/^## \\[$TAG\\]/{found=1; next} /^## \\[/{if(found) exit} found{print}" CHANGELOG.md)
          {
            echo "notes<<EOF"
            echo "$NOTES"
            echo "EOF"
          } >> "$GITHUB_OUTPUT"

      - name: Create GitHub Release
        run: gh release create "$TAG" --title "$TAG" --notes "$NOTES"
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          TAG: ${{ github.ref_name }}
          NOTES: ${{ steps.changelog.outputs.notes }}
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/release.yml
git commit -m "feat(release): crée des GitHub Releases à partir du changelog"
```

---

### Task 3: Deploy workflow — SSH to NAS

**Files:**
- Create: `.github/workflows/deploy.yml`

- [ ] **Step 1: Create deploy.yml**

```yaml
name: Deploy

on:
  push:
    tags: ["v*"]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - name: Deploy to NAS
        uses: appleboy/ssh-action@v1
        with:
          host: ${{ secrets.NAS_HOST }}
          username: ${{ secrets.NAS_SSH_USER }}
          key: ${{ secrets.NAS_SSH_KEY }}
          port: ${{ secrets.NAS_SSH_PORT }}
          command_timeout: 15m
          script: |
            export PATH="/usr/local/bin:$PATH"
            /volume1/docker/plextracker/scripts/nas-update.sh

      - name: Collect diagnostics on failure
        if: failure()
        uses: appleboy/ssh-action@v1
        with:
          host: ${{ secrets.NAS_HOST }}
          username: ${{ secrets.NAS_SSH_USER }}
          key: ${{ secrets.NAS_SSH_KEY }}
          port: ${{ secrets.NAS_SSH_PORT }}
          script: |
            export PATH="/usr/local/bin:$PATH"
            /volume1/docker/plextracker/scripts/nas-diagnostics.sh 2>&1
```

Note: `command_timeout: 15m` is needed because `docker compose up --build` can take several minutes on the NAS.

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/deploy.yml
git commit -m "feat(deploy): ajoute le déploiement automatique sur le NAS via SSH"
```

---

### Task 4: NAS update script

**Files:**
- Create: `scripts/nas-update.sh`

- [ ] **Step 1: Create nas-update.sh**

```bash
#!/bin/bash
set -o pipefail
# Script de mise à jour automatique — lancé par GitHub Actions (SSH)
# Déploie le dernier tag SemVer (vX.Y.Z) en buildant l'image sur le NAS.

export PATH="/usr/local/bin:$PATH"

APP_DIR="/volume1/docker/plextracker"
LOG_DIR="${APP_DIR}/logs"
LOG_FILE="${LOG_DIR}/update-$(date '+%Y-%m-%d').log"

mkdir -p "$LOG_DIR"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "$LOG_FILE"
}

# Retourne le dernier tag SemVer (vX.Y.Z) trié par version.
latest_tag() {
    git -C "$APP_DIR" tag --sort=-v:refname | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | head -1
}

# Retourne le tag actuellement déployé (celui qui pointe sur HEAD, ou vide).
current_tag() {
    git -C "$APP_DIR" describe --tags --exact-match HEAD 2>/dev/null
}

# Tente un déploiement (build local + up) et vérifie que le conteneur démarre.
# Retourne 0 si succès, 1 si échec.
try_deploy() {
    local tag
    tag=$(current_tag)
    log "Tentative de déploiement pour le tag ${tag:-$(git -C "$APP_DIR" rev-parse --short HEAD)}..."

    cd "$APP_DIR" || { log "ERREUR: impossible d'accéder à ${APP_DIR}"; return 1; }

    docker compose down 2>&1 | tee -a "$LOG_FILE"

    if ! docker compose up -d --build --wait 2>&1 | tee -a "$LOG_FILE"; then
        log "ERREUR: docker compose up --build a échoué (conteneur non healthy)."
        return 1
    fi

    log "Déploiement réussi."
    return 0
}

log "=== Début de la mise à jour ==="

cd "$APP_DIR" || { log "ERREUR: impossible d'accéder à ${APP_DIR}"; exit 1; }

# Récupérer les tags distants
if ! git fetch --tags origin 2>&1 | tee -a "$LOG_FILE"; then
    log "ERREUR: git fetch --tags a échoué."
    exit 1
fi

TARGET_TAG=$(latest_tag)
CURRENT_TAG=$(current_tag)

if [ -z "$TARGET_TAG" ]; then
    log "ERREUR: aucun tag SemVer trouvé."
    exit 1
fi

if [ "$TARGET_TAG" = "$CURRENT_TAG" ]; then
    RUNNING=$(docker compose ps --format '{{.State}}' 2>/dev/null | grep -ci "running" || true)
    if [ "$RUNNING" -ge 1 ]; then
        log "Déjà sur le tag ${TARGET_TAG}, conteneur OK."
        exit 0
    fi
    log "Tag ${TARGET_TAG} déjà déployé mais conteneur non running. Redéploiement..."
fi

log "Mise à jour : ${CURRENT_TAG:-aucun tag} → ${TARGET_TAG}"

# Nettoyer les fichiers non suivis qui bloqueraient le checkout
git -C "$APP_DIR" clean -fd 2>&1 | tee -a "$LOG_FILE"

# Checkout du tag cible
if ! git checkout "$TARGET_TAG" 2>&1 | tee -a "$LOG_FILE"; then
    log "ERREUR: git checkout ${TARGET_TAG} a échoué."
    exit 1
fi

# Tentative de déploiement
if try_deploy; then
    # Supprimer les anciennes images plextracker (garde uniquement la version courante)
    docker images --format '{{.Repository}}:{{.Tag}}' \
        | grep 'plextracker' \
        | grep -v "latest" \
        | xargs -r docker rmi 2>&1 | tee -a "$LOG_FILE"
    docker image prune -f 2>&1 | tee -a "$LOG_FILE"
    log "Images Docker anciennes supprimées."

    log "=== Mise à jour terminée (${TARGET_TAG}) ==="
    exit 0
fi

# Échec du déploiement : rollback vers les tags précédents
log "Le déploiement a échoué pour ${TARGET_TAG}, début du rollback..."

PREVIOUS_TAGS=$(git -C "$APP_DIR" tag --sort=-v:refname | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | grep -v "^${TARGET_TAG}$" | head -5)

for tag in $PREVIOUS_TAGS; do
    log "Rollback vers ${tag}..."
    git -C "$APP_DIR" clean -fd 2>&1 | tee -a "$LOG_FILE"
    git checkout "$tag" 2>&1 | tee -a "$LOG_FILE"

    if try_deploy; then
        log "ATTENTION: rollback effectué — vérifier manuellement la cohérence des migrations si le tag annulé contenait des changements de schéma."
        log "=== Mise à jour terminée (rollback vers ${tag}) ==="
        exit 1
    fi
done

log "ERREUR CRITIQUE: rollback échoué après avoir essayé les tags précédents. Intervention manuelle requise."
log "=== Mise à jour échouée ==="
exit 1
```

- [ ] **Step 2: Make executable**

```bash
chmod +x scripts/nas-update.sh
```

- [ ] **Step 3: Commit**

```bash
git add scripts/nas-update.sh
git commit -m "feat(deploy): ajoute le script de mise à jour NAS avec rollback"
```

---

### Task 5: NAS diagnostics script

**Files:**
- Create: `scripts/nas-diagnostics.sh`

- [ ] **Step 1: Create nas-diagnostics.sh**

```bash
#!/bin/bash
# Diagnostics PlexTracker — collecte les logs de crash pertinents
# Lancé automatiquement par GitHub Actions en cas d'échec de déploiement

export PATH="/usr/local/bin:$PATH"

APP_DIR="/volume1/docker/plextracker"
LOG_DIR="${APP_DIR}/logs"
LOG_FILE="${LOG_DIR}/diagnostics-$(date '+%Y-%m-%d_%H%M%S').log"
LINES=100

while [[ $# -gt 0 ]]; do
    case $1 in
        --lines) LINES="$2"; shift 2 ;;
        *) shift ;;
    esac
done

mkdir -p "$LOG_DIR"

exec > >(tee -a "$LOG_FILE") 2>&1

section() {
    echo ""
    echo "============================================================"
    echo "=== $1"
    echo "============================================================"
    echo ""
}

run_section() {
    local name="$1"
    shift
    section "$name"
    if ! "$@" 2>&1; then
        echo "[AVERTISSEMENT] Section '${name}' a échoué, poursuite du diagnostic..."
    fi
}

echo "=== Diagnostic PlexTracker — $(date '+%Y-%m-%d %H:%M:%S') ==="
echo ""

cd "$APP_DIR" || { echo "ERREUR: impossible d'accéder à ${APP_DIR}"; exit 1; }

# 1. Container status
run_section "ÉTAT DES CONTENEURS" docker compose ps -a

# 2. Docker logs
section "LOGS DOCKER: plextracker (${LINES} dernières lignes)"
docker compose logs --tail="$LINES" --no-color plextracker 2>&1 || echo "(conteneur introuvable ou arrêté)"

# 3. OOM / kill events
section "OOM / KILL (dmesg)"
dmesg 2>/dev/null | grep -iE 'oom|killed' | tail -20 || echo "(dmesg indisponible ou aucun événement)"

# 4. Disk space
run_section "ESPACE DISQUE" df -h /volume1

# 5. Last update log
section "DERNIER LOG DE MISE À JOUR"
LATEST_UPDATE_LOG=$(ls -t "${LOG_DIR}"/update-*.log 2>/dev/null | head -1)
if [ -n "$LATEST_UPDATE_LOG" ]; then
    echo "Fichier: ${LATEST_UPDATE_LOG}"
    echo ""
    tail -"$LINES" "$LATEST_UPDATE_LOG"
else
    echo "(aucun log de mise à jour trouvé)"
fi

echo ""
echo "=== Fin du diagnostic — $(date '+%Y-%m-%d %H:%M:%S') ==="
```

- [ ] **Step 2: Make executable**

```bash
chmod +x scripts/nas-diagnostics.sh
```

- [ ] **Step 3: Commit**

```bash
git add scripts/nas-diagnostics.sh
git commit -m "feat(deploy): ajoute le script de diagnostics NAS"
```

---

### Task 6: CHANGELOG.md

**Files:**
- Create: `CHANGELOG.md`

- [ ] **Step 1: Create CHANGELOG.md**

```markdown
# Changelog

## [v0.1.0] — 2026-04-04

Version initiale avec CI/CD.

### Ajouté
- Pipeline CI GitHub Actions (tests Go, lint, build frontend, tests frontend)
- Création automatique de GitHub Releases à partir du changelog
- Déploiement automatique sur NAS Synology via SSH
- Scripts NAS de mise à jour (avec rollback) et diagnostics
```

- [ ] **Step 2: Commit**

```bash
git add CHANGELOG.md
git commit -m "docs: ajoute le changelog initial"
```

---

### Task 7: Verification

- [ ] **Step 1: Validate workflow YAML syntax**

```bash
cat .github/workflows/ci.yml | python3 -c "import sys, yaml; yaml.safe_load(sys.stdin)" && echo "ci.yml OK"
cat .github/workflows/release.yml | python3 -c "import sys, yaml; yaml.safe_load(sys.stdin)" && echo "release.yml OK"
cat .github/workflows/deploy.yml | python3 -c "import sys, yaml; yaml.safe_load(sys.stdin)" && echo "deploy.yml OK"
```

Expected: all 3 print OK.

- [ ] **Step 2: Validate shell scripts with shellcheck (if available)**

```bash
shellcheck scripts/nas-update.sh scripts/nas-diagnostics.sh
```

- [ ] **Step 3: Verify scripts are executable**

```bash
ls -la scripts/nas-update.sh scripts/nas-diagnostics.sh
```

Expected: `-rwxr-xr-x` permissions.

- [ ] **Step 4: Push to main and verify CI runs**

Push to main, open GitHub Actions, confirm the 4 CI jobs run in parallel and pass.

- [ ] **Step 5: Configure GitHub secrets**

In GitHub repo settings → Secrets and variables → Actions, add:
- `NAS_HOST`
- `NAS_SSH_USER`
- `NAS_SSH_KEY`
- `NAS_SSH_PORT`

- [ ] **Step 6: Test full deploy with a tag**

```bash
git tag -a v0.1.0 -m "v0.1.0"
git push --tags
```

Verify in GitHub Actions:
- Release workflow creates a GitHub Release with changelog content
- Deploy workflow SSHs into NAS and runs update script successfully
