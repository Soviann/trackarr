# Trackarr Documentation — Master Index

Welcome to the official **Trackarr** documentation suite. Explore the dedicated guides below:

---

## 📖 Table of Contents

### 1. [Overview & Project Scope](overview.md)
- What Trackarr does and does not do.
- Mobile PWA access & installation (Android WebAPK, iOS, Desktop).
- App shortcuts & dynamic badge counter.
- Calendar, Wrapped, Franchise tracking, and streaming provider highlights.

### 2. [User Guide & Troubleshooting](user-guide.md)
- Complete UI walkthrough: Library statuses (*Watching*, *Caught up*, *Completed*, *Plan*, *Dropped*), responsive fluid grid, and collapsible desktop sidebar.
- Dedicated Upcoming Airing Page with at-a-glance next air dates and episode tags (`/coming-up`), and Multi-View Calendar (Month, Week, List) with RFC 5545 iCal subscription feed (`/calendar`).
- Filterable Stats insights, decomposed watch time, Year/Media filters, Top Cast & interactive filmography drawer (`/stats`).
- Trackarr Wrapped annual retrospective story player and immutable archives gallery (`/wrapped`).
- Unified Franchise & History Hub Bar with slide-up bottom sheets, Next Episode Hero, and Binge Estimator.
- Search deduplication, full-card navigation, external inspection links (`TMDB ↗`, `AniList ↗`, `IMDb ↗`), and desktop sticky top header.
- Single-line closed filter handle, scrolling dismissible chips, and compact filter drawer with selects.
- Gestures & shortcuts: Power user shortcuts (`Cmd+K`, `/`, `W`, `1..9`, `Escape`, `Shift+Click`), Universal Undo snackbar, circular `+1` quick mark, multi-select mode, mobile swipe actions, pull-to-refresh.
- Match Review, Rematching, and Season Audit.
- Comprehensive Q&A and troubleshooting based on real-world edge cases.

### 3. [API Setup & Credentials Guide](api-setup.md)
- Step-by-step instructions to get API keys for **TMDB**, **Google Gemini AI**, **AniList**, **TheTVDB**, and the ***arr Stack**.
- Exact form fields, OAuth callback URLs, and permissions.

### 4. [Integrations & Webhooks](integrations.md)
- Real-time scrobbling via **Jellyfin** & **Plex** webhooks (template and deduplication).
- Downstream **Radarr & Sonarr** download webhooks (`POST /api/webhook/arr/{secret}`) with emerald availability badges and Web Push notifications.
- **AniList** bidirectional sync (progress, scores, multi-part season chaining).
- **Radarr & Sonarr** (*arr) integration, Arr Queue, and Prowlarr releases.
- Configurable streaming watch providers (Netflix, Prime Video, Disney+, Apple TV+, Max, Canal+, Crunchyroll, Paramount+, ADN).

### 5. [Deployment & Updates](deployment.md)
- **Option 1: Automated Updates via Watchtower** (Recommended).
- **Option 2: Standalone Update Script with Rollback (`update.sh`)**.
- **Option 3: Manual Update via Docker Compose / Container Manager GUI**.
- Bare-metal Go/Systemd deployment, environment variables, and reverse proxy setup.

### 6. [Maintenance & Operations](maintenance.md)
- 1-Click Web UI Export & Import (JSON, CSV, Trakt.tv sync) with dry-run preview.
- Automated dependency management via Dependabot.
- Automated CI/CD pipelines (unit tests, linting, production Vite compilation, multi-arch GHCR releases, and Dependabot auto-merge).
- Private daemon routines for weekly documentation/i18n audits and feature proposals.
- Online SQLite WAL backup, daily pragma maintenance cron (04:30), and restoration procedures.
- Emergency CLI maintenance commands (`reset-password`, `backfill-accents`, `version`, `import`).

### 7. [LLM Context Reference](llm.md)
- High-density, token-optimized technical reference for AI assistants and LLMs.

### 8. [Design Revamp Roadmap](roadmap-design-revamp.md)
- Complete UI/UX revamp plan, Android one-handed mobile ergonomic priorities, zero-reflow animations, and adaptive desktop layout.

### 9. [Evolution & Performance Roadmap (Q4 2026)](roadmap-2026-q4.md)
- Multi-session feature roadmap: SQLite WAL I/O tuning, frontend code splitting, external search links, studio bento detail layout, saga hub, and search deduplication.

---

## 🛠️ Developer Architecture Guides

For developers and contributors looking into the internals:
- [**Architecture & Patterns**](patterns.md) — Backend and frontend conventions, routing inventory, and components catalog.
- [**Database Model & SQLite WAL**](dev/database-model.md) — Database schema, reader/writer separation, and transaction invariants.
- [**Matching Pipeline & AI Verification**](dev/matching-pipeline.md) — 5-step resolution engine and Gemini AI verification.
- [**AniList Sync & Multi-Parts**](dev/anilist-sync.md) — Prequel trees, season mapping, and GraphQL push protocol.
- [**Arr Stack Integration**](dev/arr-integration.md) — Radarr/Sonarr proxy architecture, queue management, and quality profiles.
- [**Background Jobs & Task Queue**](background-jobs.md) — 24h metadata refresh ticker, asynchronous SQLite task queue, retry policies, and graceful shutdown.
- [**Native Go MCP Server (stdio)**](dev/mcp-server.md) — Model Context Protocol (MCP) server for Claude Desktop, Cursor, and Antigravity.
