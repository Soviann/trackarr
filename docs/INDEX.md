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
- Complete UI walkthrough: Library statuses (*Watching*, *Caught up*, *Completed*, *Plan*, *Dropped*).
- Multi-View Calendar (Month, Week, List) and RFC 5545 iCal subscription feed (`/coming-up`).
- Filterable Stats insights, decomposed watch time, Year/Media filters, Top Cast & interactive filmography drawer (`/stats`).
- Trackarr Wrapped annual retrospective story player and immutable archives gallery (`/wrapped`).
- Unified Franchise & History Hub Bar with slide-up bottom sheets, Next Episode Hero, and Binge Estimator.
- Single-line closed filter handle, scrolling dismissible chips, and compact filter drawer with selects.
- Gestures & shortcuts: Universal Undo snackbar, circular `+1` quick mark, multi-select mode, swipe actions, pull-to-refresh.
- Match Review, Rematching, and Season Audit.
- Comprehensive Q&A and troubleshooting based on real-world edge cases.

### 3. [API Setup & Credentials Guide](api-setup.md)
- Step-by-step instructions to get API keys for **TMDB**, **Google Gemini AI**, **AniList**, **TheTVDB**, and the ***arr Stack**.
- Exact form fields, OAuth callback URLs, and permissions.

### 4. [Integrations & Webhooks](integrations.md)
- Real-time scrobbling via **Jellyfin** & **Plex** webhooks (template and deduplication).
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
- Weekly documentation, i18n & hardcoded text audit workflow (`weekly-docs-audit.yml`).
- Online SQLite WAL backup and restoration procedures.
- Emergency CLI maintenance commands (`reset-password`, `backfill-accents`, `version`, `import`).

### 7. [LLM Context Reference](llm.md)
- High-density, token-optimized technical reference for AI assistants and LLMs.

---

## 🛠️ Developer Architecture Guides

For developers and contributors looking into the internals:
- [**Architecture & Patterns**](patterns.md) — Backend and frontend conventions, routing inventory, and components catalog.
- [**Database Model & SQLite WAL**](dev/database-model.md) — Database schema, reader/writer separation, and transaction invariants.
- [**Matching Pipeline & AI Verification**](dev/matching-pipeline.md) — 5-step resolution engine and Gemini AI verification.
- [**AniList Sync & Multi-Parts**](dev/anilist-sync.md) — Prequel trees, season mapping, and GraphQL push protocol.
- [**Arr Stack Integration**](dev/arr-integration.md) — Radarr/Sonarr proxy architecture, queue management, and quality profiles.
- [**Background Jobs & Task Queue**](background-jobs.md) — 24h metadata refresh ticker, asynchronous SQLite task queue, retry policies, and graceful shutdown.
