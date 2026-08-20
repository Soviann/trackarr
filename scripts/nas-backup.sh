#!/bin/bash
set -o pipefail
# Script de backup automatique de la BDD — lancé par le planificateur DSM (root)

export PATH="/usr/local/bin:/usr/bin:/bin:$PATH"

APP_DIR="/volume1/docker/plextracker"
DATA_DIR="${APP_DIR}/data"
DB_FILE="${DATA_DIR}/plextracker.db"
BACKUP_DIR="/volume1/google drive/Backup/Plextracker"
LOG_DIR="${APP_DIR}/logs"
LOG_FILE="${LOG_DIR}/backup-$(date '+%Y-%m-%d').log"
RETENTION_DAYS=7

mkdir -p "$LOG_DIR"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" >> "$LOG_FILE"
}

log "=== Début du backup ==="

# Crée le dossier de backup s'il n'existe pas
if [ ! -d "$BACKUP_DIR" ]; then
    mkdir -p "$BACKUP_DIR"
    log "Dossier de backup créé : ${BACKUP_DIR}"
fi

# Vérifie l'existence du fichier de base de données
if [ ! -f "$DB_FILE" ]; then
    log "ERREUR: fichier ${DB_FILE} introuvable"
    exit 1
fi

DUMP_FILE="${BACKUP_DIR}/plextracker-$(date '+%Y-%m-%d_%H%M%S').db.gz"
TEMP_DB="${DATA_DIR}/backup-temp-$$.db"

# Sauvegarde SQLite à chaud et cohérente (gestion du mode WAL)
if command -v sqlite3 >/dev/null 2>&1; then
    sqlite3 "$DB_FILE" ".backup '${TEMP_DB}'" 2>> "$LOG_FILE"
else
    # Fallback si sqlite3 n'est pas dans le PATH standard
    /usr/bin/sqlite3 "$DB_FILE" ".backup '${TEMP_DB}'" 2>> "$LOG_FILE" || cp -f "$DB_FILE" "$TEMP_DB"
fi

if [ $? -ne 0 ] || [ ! -s "$TEMP_DB" ]; then
    log "ERREUR: la sauvegarde SQLite a échoué"
    rm -f "$TEMP_DB"
    exit 1
fi

# Compression gzip
gzip -c "$TEMP_DB" > "$DUMP_FILE" 2>> "$LOG_FILE"
GZIP_STATUS=$?
rm -f "$TEMP_DB"

if [ $GZIP_STATUS -ne 0 ] || [ ! -s "$DUMP_FILE" ]; then
    log "ERREUR: la compression gzip a échoué"
    rm -f "$DUMP_FILE"
    exit 1
fi

DUMP_SIZE=$(du -h "$DUMP_FILE" | cut -f1)
log "Backup créé : ${DUMP_FILE} (${DUMP_SIZE})"

# Rotation : supprime les backups de plus de N jours
DELETED=$(find "$BACKUP_DIR" -name "plextracker-*.db.gz" -mtime +${RETENTION_DAYS} -delete -print 2>> "$LOG_FILE" | wc -l)
if [ "$DELETED" -gt 0 ]; then
    log "Rotation : ${DELETED} backup(s) de plus de ${RETENTION_DAYS} jours supprimé(s)"
fi

log "=== Backup terminé ==="
