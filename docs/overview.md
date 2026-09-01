# Overview & Project Scope

[← Back to Index](INDEX.md)

---

## What is Trackarr?

**Trackarr** is a fast, lightweight, and self-hosted personal media tracker for movies, TV shows, and anime.

### What Trackarr Does
- **Automatic Scrobbling**: Automatically logs finished movies and episodes via Jellyfin and Plex webhooks.
- **Unified Library**: Track your media across standard statuses: *Watching*, *Caught up*, *Plan to Watch*, *Completed*, and *Dropped*.
- **Instant Episode Progress**: One-tap progress tracking with episode counters and upcoming air dates.
- **Ratings & AniList Sync**: Native 1–10 scoring with automatic background synchronization to AniList (including multi-part season chaining).
- ***arr Stack Integration**: Live checks with Radarr and Sonarr, colored status borders, and direct queue management.
- **Prowlarr Releases Feed**: 1-click addition from your indexer releases into Trackarr.
- **Smart Matching Engine**: Resolves external IDs across TMDB, TVDB, IMDb, and AniList with Gemini AI verification.
- **Trackarr Wrapped**: Interactive multi-slide annual retrospective with Gemini AI viewing persona, top favorites, release rankings, rewatch champion, and custom fun facts.

### What Trackarr Intentionally Does NOT Do
- **Not a Social Review Platform**: No lengthy review writing (provides 1-click links to TMDB, IMDb, and AniList instead).
- **Not a Heavy Native App**: Mobile-first Progressive Web App (PWA) optimized for Android WebAPK and desktop browsers.
- **Not a Multi-Tenant SaaS**: Engineered as a single-user private media vault with minimal footprint (~30 MB RAM).

---

## PWA Installation & Access

Trackarr is built as a **Progressive Web App (PWA)**:

### First Access & Setup
1. Open your Trackarr URL (e.g. `https://<your-trackarr-url>` or `http://<server-ip>:8080`) in your browser.
2. Log in using your local password or Google OAuth.
3. **Install the App**:
   - **Android / Chrome**: Tap the "Install Trackarr" banner, or open the Chrome menu (⋮) ➔ *Install app* / *Add to Home screen*.
   - **iOS / Safari**: Tap the Share button ➔ *Add to Home Screen*.
   - **Desktop**: Click the install icon in the address bar.

### App Shortcuts
Long-pressing the Trackarr app icon on Android reveals 3 instant app shortcuts:
- **Add Title** (`/add`)
- **Library** (`/`)
- **Search** (`/search`)

### Dynamic App Badge
Trackarr updates its app icon badge count with the number of titles waiting for review in **Match Review**.
