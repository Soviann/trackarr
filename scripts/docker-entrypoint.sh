#!/bin/sh
set -e

# Support dynamic PUID / PGID for NAS and homelab setups (Synology, Unraid, TrueNAS)
PUID=${PUID:-1000}
PGID=${PGID:-1000}

if [ "$PUID" != "$(id -u appuser 2>/dev/null)" ] || [ "$PGID" != "$(id -g appuser 2>/dev/null)" ]; then
    groupmod -o -g "$PGID" appuser 2>/dev/null || true
    usermod -o -u "$PUID" -g "$PGID" appuser 2>/dev/null || true
fi

# 1. Pre-init root scripts (/docker-entrypoint-init.d/root.d/*.sh)
if [ -d "/docker-entrypoint-init.d/root.d" ]; then
    for f in /docker-entrypoint-init.d/root.d/*.sh; do
        if [ -f "$f" ]; then
            echo "[entrypoint] Running root pre-init script: $f"
            sh "$f"
        fi
    done
fi

mkdir -p /data
chown -R appuser:appuser /data

# Helper to run scripts as appuser
run_user_script() {
    script_path="$1"
    if [ -f "$script_path" ]; then
        echo "[entrypoint] Running init script as appuser: $script_path"
        gosu appuser sh "$script_path"
    fi
}

# 2. Pre-start scripts mounted in /docker-entrypoint-init.d/*.sh
if [ -d "/docker-entrypoint-init.d" ]; then
    for f in /docker-entrypoint-init.d/*.sh; do
        [ -f "$f" ] && run_user_script "$f"
    done
fi

# 3. Pre-start scripts in persistent data volume (/data/init.d/*.sh)
if [ -d "/data/init.d" ]; then
    for f in /data/init.d/*.sh; do
        [ -f "$f" ] && run_user_script "$f"
    done
fi

exec gosu appuser "$@"
