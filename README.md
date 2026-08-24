# Trackarr

<div align="center">
  <img src="frontend/public/favicon.svg" alt="Trackarr Logo" width="128" height="128" />
  <h3>Lightweight Self-Hosted Media Tracker & Watchlist Manager</h3>
  <p>Track movies, TV series, and anime with automated Jellyfin/Plex scrobbling, multi-part anime season mapping, AI-assisted title reconciliation, and *arr automation.</p>

  [![CI](https://github.com/Soviann/trackarr/actions/workflows/ci.yml/badge.svg)](https://github.com/Soviann/trackarr/actions/workflows/ci.yml)
  [![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
  [![Go Version](https://img.shields.io/badge/Go-1.24-00ADD8?logo=go)](go.mod)
  [![Docker](https://img.shields.io/badge/Docker-Multi--Arch-2496ED?logo=docker)](https://ghcr.io/soviann/trackarr)
</div>

---

## ⚡ Why Trackarr?

- 🚀 **Ultra-Lightweight Go Backend**: Single binary with embedded SQLite WAL and embedded web assets. Idles at **under 30 MB of RAM**, perfectly suited for low-power Synology NAS, Unraid, TrueNAS, Raspberry Pi, and VPS.
- 📱 **PWA Mobile-First for Android**: Tailored specifically for Android devices (Chrome / Android PWA engine) with native share sheet integration (`Share to Trackarr`), haptic feedback, swipe gestures, and instant pull-to-refresh.
- 🎬 **Universal Scrobbling**: Real-time webhook scrobbling from **Jellyfin** and **Plex** with automatic watch time, progress tracking, and catch-up badges.
- 🌸 **AniList 2-Way Sync & Multi-Part Anime**: Seamlessly handles anime seasons split across multiple AniList entries (e.g. Cour 1 / Cour 2) with automated bidirectional rating, progress, and status synchronization.
- 🤖 **AI-Assisted Title Matching**: Multi-tiered reconciliation engine (Plex/Jellyfin IDs ➔ TMDB ➔ TVDB ➔ AniList ➔ Google Gemini AI fuzzy resolution) for zero-headache metadata accuracy.
- 📥 ***arr Automation (Radarr & Sonarr)**: Check library status directly on title detail sheets and push missing movies/shows straight to your Radarr/Sonarr download queues.
- 🎨 **4 Modern Themes**: In-app theme switcher featuring *Cyber Cyan*, *Sunset Coral*, *Emerald Teal*, and *Vault Amber*.

---

## 💡 Project Scope & Philosophy

Trackarr was originally built as a personal homelab media tracker and watchlist manager. It is designed to be ultra-lightweight, single-user focused, and tailored for self-hosters with a deliberate set of integrations (Plex, Jellyfin, AniList, Radarr, Sonarr, Prowlarr).

While the project is open-source and shared with the community, development is centered around keeping the codebase fast, robust, and maintaining this focused vision.

---

## 🚀 Quickstart with Docker Compose

Create a `docker-compose.yml` file:

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
      # Optional: Pre-configure OAuth / TMDB keys (can also be configured via /admin UI)
      # - TMDB_API_KEY=your_tmdb_api_key
      # - GOOGLE_CLIENT_ID=your_client_id.apps.googleusercontent.com
      # - GOOGLE_ALLOWED_EMAIL=you@gmail.com
    volumes:
      - ./data:/data
      # Optional: Mount custom init scripts (runs on container startup)
      # - ./my-init-scripts:/docker-entrypoint-init.d:ro
```

Start the container:

```bash
docker compose up -d
```

Open `https://<your-trackarr-url>` (or `http://<server-ip>:8080`) in your browser to access the initial setup wizard.

### 🔌 Custom Startup & Init Scripts

Trackarr can execute custom `.sh` scripts automatically during container startup:
- **Pre-start scripts (user level)**: Place `.sh` scripts in a directory mounted to `/docker-entrypoint-init.d/` or inside `./data/init.d/` in your persistent data folder. They execute as `appuser` right before Trackarr starts (e.g. SQLite backups, pre-fetching offline databases, webhook notifications).
- **Pre-init scripts (root level)**: Place `.sh` scripts inside a mounted `/docker-entrypoint-init.d/root.d/` directory to run system-level tasks before dropping privileges.

---

## 🛠️ Development & Local Setup

Trackarr runs entirely inside Docker for consistent, reproducible builds across macOS, Linux, and Windows:

```bash
# Clone repository
git clone https://github.com/Soviann/trackarr.git
cd trackarr

# Start development container
make up

# Start Vite frontend with hot-module reload (HMR)
make dev-frontend

# Run full test suite
make test        # Go backend unit tests
make test-front  # Vitest frontend tests + build verification
make lint        # golangci-lint
make lint-front  # TypeScript type-checking
```

---

## 📚 Documentation

Detailed guides and specifications are available in the [`docs/`](docs/INDEX.md) folder:

- [**Overview & Project Scope**](docs/overview.md) — Core capabilities, non-goals, and mobile PWA installation.
- [**User Guide & Troubleshooting**](docs/user-guide.md) — Gestures, library organization, rating system, and troubleshooting.
- [**Integrations & Webhooks**](docs/integrations.md) — Jellyfin & Plex webhook setup, AniList OAuth configuration, and Radarr/Sonarr setup.
- [**Matching Pipeline & AI**](docs/dev/matching-pipeline.md) — How titles are matched and reconciled via Gemini AI.
- [**AniList Synchronization**](docs/dev/anilist-sync.md) — Multi-part anime mapping and synchronization lifecycle.
- [**Deployment & Backups**](docs/deployment.md) — Production deployment on NAS/VPS and SQLite backup guidelines.
- [**Architecture & Patterns**](docs/patterns.md) — System architecture, database conventions, and design patterns.

---

## 🔒 Security

For security vulnerability disclosures, please review our [Security Policy](SECURITY.md).

---

## 📄 License

Trackarr is open-source software licensed under the [MIT License](LICENSE).
