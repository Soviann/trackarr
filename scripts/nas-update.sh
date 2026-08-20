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

    mkdir -p "$APP_DIR/antigravity"
    if [ ! -s "$APP_DIR/.env.local" ]; then
        log "ATTENTION: $APP_DIR/.env.local est absent ou vide. Si le conteneur refuse de démarrer, exécutez 'make push-secrets'."
        touch "$APP_DIR/.env.local"
    fi
    if [ -f "$APP_DIR/.env.local" ]; then
        cp -f "$APP_DIR/.env.local" "$APP_DIR/antigravity/.env.local"
    fi

    if ! docker compose build --no-cache plextracker 2>&1 | tee -a "$LOG_FILE"; then
        log "ERREUR: docker compose build a échoué."
        return 1
    fi

    if ! docker compose up -d --wait 2>&1 | tee -a "$LOG_FILE"; then
        log "ERREUR: docker compose up a échoué (conteneur non healthy)."
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

# Checkout du tag cible (force pour ignorer les modifications locales)
if ! git -C "$APP_DIR" checkout -f "$TARGET_TAG" 2>&1 | tee -a "$LOG_FILE"; then
    log "ERREUR: git checkout ${TARGET_TAG} a échoué."
    exit 1
fi
mkdir -p "$LOG_DIR"

# Sauvegarde de sécurité de la base de données avant la tentative
if [ -f "$APP_DIR/data/plextracker.db" ]; then
    cp -f "$APP_DIR/data/plextracker.db" "$APP_DIR/data/plextracker.db.pre-deploy.bak" 2>/dev/null || true
fi

# Tentative de déploiement
if try_deploy; then
    rm -f "$APP_DIR/data/plextracker.db.pre-deploy.bak" 2>/dev/null || true
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

# Restauration de la sauvegarde pre-deploy si présente pour éviter les blocages de schéma
if [ -f "$APP_DIR/data/plextracker.db.pre-deploy.bak" ]; then
    log "Restauration de la base de données pre-deploy pour le rollback..."
    cp -f "$APP_DIR/data/plextracker.db.pre-deploy.bak" "$APP_DIR/data/plextracker.db" 2>/dev/null || true
fi

PREVIOUS_TAGS=$(git -C "$APP_DIR" tag --sort=-v:refname | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | grep -v "^${TARGET_TAG}$" | head -5)

for tag in $PREVIOUS_TAGS; do
    log "Rollback vers ${tag}..."
    git -C "$APP_DIR" checkout -f "$tag" 2>&1 | tee -a "$LOG_FILE"
    mkdir -p "$LOG_DIR"

    if try_deploy; then
        log "Rollback effectué avec succès vers ${tag}."
        log "=== Mise à jour terminée (rollback vers ${tag}) ==="
        exit 1
    fi
done

log "ERREUR CRITIQUE: rollback échoué après avoir essayé les tags précédents. Intervention manuelle requise."
log "=== Mise à jour échouée ==="
exit 1
