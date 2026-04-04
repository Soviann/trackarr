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
