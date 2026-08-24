# Deployment & Updates Guide

[← Back to Index](INDEX.md)

This guide details how to deploy and automatically maintain **Trackarr** on any Docker-capable platform: Linux servers, Synology NAS, Unraid, TrueNAS, Raspberry Pi, or bare-metal environments.

---

## 📑 Table of Contents
1. [Quick Start (Docker Compose)](#1-quick-start-docker-compose)
2. [Environment Variables](#2-environment-variables)
3. [The 3 Deployment & Update Solutions](#3-the-3-deployment--update-solutions)
   - [Option 1: Automated Updates via Watchtower (Recommended)](#option-1-automated-updates-via-watchtower-recommended)
   - [Option 2: Standalone Update Script with Rollback (`update.sh`)](#option-2-standalone-update-script-with-rollback-updatesh)
   - [Option 3: Manual Update via Docker Compose or Container Manager GUI](#option-3-manual-update-via-docker-compose-or-container-manager-gui)
4. [Bare-Metal / Systemd Deployment](#4-bare-metal--systemd-deployment)
5. [Reverse Proxy & HTTPS Setup](#5-reverse-proxy--https-setup)
6. [Historical Import from Simkl (Optional)](#6-historical-import-from-simkl-optional)

---

## 1. Quick Start (Docker Compose)

Create a `docker-compose.yml` file in your preferred directory (e.g. `/opt/trackarr` or `/volume1/docker/trackarr`):

```yaml
services:
  trackarr:
    image: ghcr.io/soviann/trackarr:latest
    container_name: trackarr
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      - PUID=1000
      - PGID=1000
      - TZ=Europe/Paris
      - DATA_DIR=/data
      - TMDB_API_KEY=your_tmdb_key_here
      - GEMINI_API_KEY=your_gemini_key_here
      - ANILIST_CLIENT_ID=your_anilist_client_id
      - ANILIST_CLIENT_SECRET=your_anilist_client_secret
    volumes:
      - ./data:/data
```

Start the container:
```bash
docker compose up -d
```

> **Synology DSM Note**: In **Container Manager**, you can deploy this directly via GUI: go to **Project ➔ Create**, name the project `trackarr`, set the path (e.g. `/docker/trackarr`), and paste the `docker-compose.yml` above.

Trackarr will automatically initialize the SQLite database in `./data/trackarr.db`, apply all migrations, and listen on port `8080` (accessible at `https://<your-trackarr-url>` or `http://<server-ip>:8080`).

### 🚀 Initial First-Boot Setup Wizard (`/setup`)

When launching Trackarr for the first time with a fresh database:
1. Open `https://<your-trackarr-url>` (or `http://<server-ip>:8080`) in your web browser. Trackarr automatically directs you to the **Onboarding Wizard** (`/setup`).
2. Create your administrator username and password (minimum 8 characters).
3. **Emergency Recovery Key**: Trackarr generates a unique, single-use cryptographic recovery key (`TRCK-XXXX-XXXX-XXXX`). Copy and store this key safely. It is stored as an irreversible bcrypt hash in SQLite and will never be displayed again.
4. **Permanent Security Locking**: As soon as the setup is completed, the `/setup` route is **permanently disabled** (HTTP 403 Forbidden).

> ℹ️ *Password Recovery*: If you ever forget your password, you can reset it directly from the browser login screen using your emergency recovery key, or from the server terminal via `docker exec -t trackarr trackarr reset-password --password="MyNewPassword"`.

---

## 2. Environment Variables

| Variable | Description | Default | Required? |
|---|---|---|---|
| `DATA_DIR` | Directory where SQLite database and cover art are stored | `/data` | Yes |
| `PUID` / `PGID` | User and Group IDs for filesystem permissions | `1000` / `1000` | No |
| `TZ` | Server timezone for next episode air dates | `UTC` | Recommended |
| `LISTEN_ADDR` | Web server listening address inside the container | `:8080` | No |
| `JWT_SECRET` | Secret key for session cookies | Auto-generated if omitted | No |
| `TMDB_API_KEY` | The Movie Database API key (v3 auth) | — | Recommended |
| `GEMINI_API_KEY` | Google Gemini AI key (fuzzy match & auto-verification) | — | Recommended |
| `ANILIST_CLIENT_ID` | AniList OAuth Client ID | — | Optional |
| `ANILIST_CLIENT_SECRET`| AniList OAuth Client Secret | — | Optional |
| `TVDB_API_KEY` | TheTVDB API Key | — | Optional |
| `GOOGLE_CLIENT_ID` | Google OAuth Client ID | — | Optional |
| `GOOGLE_ALLOWED_EMAIL` | Restrict Google Login to this specific email | — | Optional |
| `VAPID_PUBLIC_KEY` | Web Push public key | — | Optional |
| `VAPID_PRIVATE_KEY` | Web Push private key | — | Optional |
| `VAPID_SUBJECT` | Web Push contact mailto (`mailto:user@domain.com`) | — | Optional |

> ℹ️ *Tip: All API keys (TMDB, AniList, Radarr, Sonarr, Prowlarr) can also be configured dynamically at runtime from the **Admin Dashboard** (`/admin`).*

---

## 3. The 3 Deployment & Update Solutions

Trackarr publishes multi-architecture images (`linux/amd64` and `linux/arm64`) to the GitHub Container Registry (`ghcr.io/soviann/trackarr:latest`). Choose the update strategy that best fits your infrastructure:

---

### Option 1: Automated Updates via Watchtower (Recommended)

[Watchtower](https://containrrr.dev/watchtower/) automatically monitors GHCR for new Trackarr releases, pulls the new image, recreates the container, and shuts down the old version smoothly. Database migrations run automatically at startup.

Add Watchtower to your `docker-compose.yml`:

```yaml
services:
  trackarr:
    image: ghcr.io/soviann/trackarr:latest
    container_name: trackarr
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      - PUID=1000
      - PGID=1000
      - TZ=Europe/Paris
      - DATA_DIR=/data
    volumes:
      - ./data:/data
    labels:
      - "com.centurylinklabs.watchtower.enable=true"

  watchtower:
    image: containrrr/watchtower:latest
    container_name: trackarr-watchtower
    restart: unless-stopped
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    environment:
      - WATCHTOWER_CLEANUP=true
      - WATCHTOWER_LABEL_ENABLE=true
      - WATCHTOWER_POLL_INTERVAL=86400  # Check once every 24 hours
```

---

### Option 2: Standalone Update Script with Rollback (`update.sh`)

For NAS devices (Synology Task Scheduler, Unraid User Scripts, TrueNAS cron, or Webhook triggers), you can use an automated shell script that creates a safety backup of your SQLite database before updating and rolls back if the new container fails health checks.

Create `update.sh` alongside your `docker-compose.yml`:

```bash
#!/bin/bash
set -euo pipefail

APP_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$APP_DIR"

echo "[$(date '+%Y-%m-%d %H:%M:%S')] === Starting Trackarr Update ==="

# 1. Pull latest image
echo "Pulling latest image from GHCR..."
docker compose pull trackarr

# 2. Backup database before deployment
if [ -f "$APP_DIR/data/trackarr.db" ]; then
    echo "Creating pre-update database backup..."
    cp -f "$APP_DIR/data/trackarr.db" "$APP_DIR/data/trackarr.db.pre-update.bak"
fi

# 3. Restart container with wait/healthcheck
echo "Recreating container..."
if docker compose up -d --remove-orphans trackarr; then
    echo "Container updated successfully."
    rm -f "$APP_DIR/data/trackarr.db.pre-update.bak"
    docker image prune -f
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] === Update Complete ==="
    exit 0
else
    echo "ERROR: Container startup failed! Rolling back database..."
    if [ -f "$APP_DIR/data/trackarr.db.pre-update.bak" ]; then
        cp -f "$APP_DIR/data/trackarr.db.pre-update.bak" "$APP_DIR/data/trackarr.db"
    fi
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] === Update Failed (Rollback applied) ==="
    exit 1
fi
```

Make it executable:
```bash
chmod +x update.sh
```

- **Synology NAS**: You can schedule `bash /path/to/update.sh` in DSM **Control Panel ➔ Task Scheduler** (daily or weekly user-defined script as `root`).
- **Linux VPS / Other NAS**: Add `0 4 * * * /path/to/update.sh` to your system `crontab` or Unraid *User Scripts*.

---

### Option 3: Manual Update via Docker Compose or Container Manager GUI

If you prefer full control over when updates are applied:

#### Via Docker Compose CLI:
```bash
cd /opt/trackarr
docker compose pull
docker compose up -d --remove-orphans
docker image prune -f
```

#### Via Synology Container Manager (GUI):
1. Open **Container Manager** ➔ **Project**.
2. Select your `trackarr` project (created in step 1).
3. Click **Action** ➔ **Update** (or check *Pull latest image* and restart).
4. Container Manager pulls the latest GHCR image and restarts the container.

#### Via Unraid Docker GUI:
1. Open the **Docker** tab.
2. Click **Check for Updates**.
3. Click **Update** next to `trackarr`.

#### Via Portainer:
1. Open **Stacks** ➔ Select `trackarr`.
2. Click **Editor** ➔ **Update the stack**.
3. Toggle **Re-pull image and redeploy**.

---

## 4. Bare-Metal / Systemd Deployment

To run Trackarr natively as a Linux systemd service:

### Prerequisites:
- Go 1.24+
- SQLite3 with FTS5 headers (`libsqlite3-dev` on Debian/Ubuntu)
- Node.js 20+ (for building frontend)

### Build Steps:
```bash
# 1. Clone repository
git clone https://github.com/Soviann/trackarr.git
cd trackarr

# 2. Build frontend
cd frontend && npm install && npm run build && cd ..

# 3. Compile standalone Go binary with embedded frontend
go build -tags sqlite_fts5 -o trackarr .
```

### Systemd Service Unit (`/etc/systemd/system/trackarr.service`):
```ini
[Unit]
Description=Trackarr Media Tracker
After=network.target

[Service]
Type=simple
User=trackarr
Group=trackarr
WorkingDirectory=/var/lib/trackarr
ExecStart=/usr/local/bin/trackarr
EnvironmentFile=/etc/trackarr.env
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

---

## 5. Reverse Proxy & HTTPS Setup

Trackarr operates smoothly behind reverse proxies (Synology Reverse Proxy, Nginx, Caddy, Traefik, Cloudflare Tunnel).

### Synology DSM Reverse Proxy (GUI):
1. Open **Control Panel ➔ Login Portal ➔ Advanced ➔ Reverse Proxy**.
2. Click **Create**:
   - **General Tab**:
     - **Source**: Protocol: `HTTPS`, Hostname: `trackarr.yourdomain.com` (or your DDNS), Port: `443`, Enable HSTS.
     - **Destination**: Protocol: `HTTP`, Hostname: `localhost` (or `127.0.0.1`), Port: `8080`.
   - **Custom Header Tab**:
     - Click **Create ➔ WebSocket** to automatically add `Upgrade` and `Connection` headers for PWA sync.
3. In **Control Panel ➔ Security ➔ Certificate**, assign your Let's Encrypt SSL certificate to the newly created reverse proxy entry.

### Nginx Example:
```nginx
server {
    server_name trackarr.yourdomain.com;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # WebSocket support (for live notifications / sync)
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

### Caddy Example:
```caddy
trackarr.yourdomain.com {
    reverse_proxy localhost:8080
}
```

---

## 6. Historical Import from Simkl (Optional)

> ℹ️ *Trackarr starts with a clean slate and tracks watches automatically via Webhooks and search. Importing historical data from Simkl or backup archives is completely optional.*

If you have a Simkl JSON/ZIP backup:
```bash
# Dry-run preview (verifies items without writing to DB)
docker exec -t trackarr trackarr import --dry-run /path/to/backup.zip

# Perform real import
docker exec -t trackarr trackarr import /path/to/backup.zip
```
