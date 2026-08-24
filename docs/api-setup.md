# API Setup & Credentials Guide

[← Back to Index](INDEX.md)

---

This guide walks you through obtaining and configuring every external API key and integration supported by **Trackarr**.

All API keys can be set either via environment variables (`.env` / `.env.local` / Docker environment) or configured directly inside Trackarr via the **Admin Dashboard** (`/admin/system-settings` and `/admin/anilist`).

---

## 📑 Table of Contents
1. [The Movie Database (TMDB) API](#1-the-movie-database-tmdb-api) *(Recommended for Movies & TV)*
2. [Google Gemini AI API](#2-google-gemini-ai-api) *(Recommended for Fuzzy Matching & Verification)*
3. [AniList API (OAuth Client)](#3-anilist-api-oauth-client) *(Required for Anime Sync)*
4. [TheTVDB API](#4-thetvdb-api) *(Optional fallback for TV metadata)*
5. [Radarr / Sonarr / Prowlarr (*arr Stack)](#5-radarr--sonarr--prowlarr-arr-stack) *(Optional media management)*
6. [Web Push (VAPID Keys)](#6-web-push-vapid-keys) *(Optional for Push Notifications)*

---

## 1. The Movie Database (TMDB) API

TMDB provides primary metadata, synopsis, cast & crew, release dates, and high-resolution posters for movies and series.

### Steps to get your API Key:
1. Create a free account on [The Movie Database (themoviedb.org)](https://www.themoviedb.org/signup).
2. Log in and navigate to **Settings** ➔ [**API**](https://www.themoviedb.org/settings/api).
3. Click **Create** or **Request an API Key** (choose **Developer**).
4. Fill in the short application form:
   - **Application Name**: `Trackarr` (or your personal name)
   - **Application URL**: `https://<your-trackarr-url>` (or `http://<server-ip>:8080`)
   - **Application Summary**: `Personal media tracking application`
5. Submit the form. Your **API Key (v3 auth)** will be generated immediately.

```env
# In .env or Docker environment
TMDB_API_KEY=your_32_character_tmdb_api_key_here
```

---

## 2. Google Gemini AI API

Trackarr uses Google Gemini AI for smart fuzzy matching, auto-validating ambiguous titles, and resolving multi-season anime prequels without manual review. Free tier keys are sufficient for personal usage.

### Steps to get your API Key:
1. Go to [Google AI Studio (aistudio.google.com)](https://aistudio.google.com/app/apikey).
2. Sign in with your Google account.
3. Click **Create API Key**.
4. Select an existing Google Cloud project or click **Create API key in new project**.
5. Copy the generated API Key.

```env
# In .env or Docker environment
GEMINI_API_KEY=AIzaSy...your_gemini_key_here
```

> **Tip**: You can provide multiple comma-separated keys (`GEMINI_API_KEY=key1,key2`) for automatic load balancing and fallback on rate limits.

---

## 3. AniList API (OAuth Client)

AniList provides anime metadata, episode lists, seasons relations, community scores, and allows bidirectional background progress synchronization.

### Steps to create your OAuth Client:
1. Log in to [AniList (anilist.co)](https://anilist.co/login).
2. Go to **Settings** ➔ [**Developer**](https://anilist.co/settings/developer).
3. Click **Create New Client**.
4. Fill in the form:
   - **Name**: `Trackarr`
   - **Redirect URL**: `https://<your-trackarr-url>/api/auth/anilist/callback` (or `http://<server-ip>:8080/api/auth/anilist/callback`)
5. Click **Save**.
6. Copy the **Client ID** (a numeric ID) and **Client Secret**.

```env
# In .env or Docker environment
ANILIST_CLIENT_ID=12345
ANILIST_CLIENT_SECRET=your_anilist_client_secret_here
```

### Linking your Account:
Once configured, go to **Admin Dashboard ➔ AniList Integration ➔ Connect Account** in Trackarr. Authorize Trackarr on AniList; tokens are securely stored in your SQLite database.

---

## 4. TheTVDB API

TheTVDB is an optional secondary provider for TV series metadata, episode ordering, and fallback poster matching.

### Steps to get your API Key:
1. Create an account on [TheTVDB.com](https://thetvdb.com/dashboard/account/apikeys).
2. Navigate to **API Keys** in your dashboard.
3. Click **Create Project API Key**.
4. Enter your Project Name (`Trackarr`).
5. Copy the **API Key**.

```env
# In .env or Docker environment
TVDB_API_KEY=your_tvdb_api_key_here
```

---

## 5. Radarr / Sonarr / Prowlarr (*arr Stack)

Trackarr seamlessly interfaces with Radarr and Sonarr to check download availability, show colored status borders on posters, and queue titles directly from incoming Prowlarr releases.

### Radarr (Movies):
1. Open **Radarr** ➔ **Settings** ➔ **General** ➔ **Security**.
2. Copy the **API Key**.
3. In Trackarr **Admin ➔ Arr Stack**:
   - **Radarr URL**: `http://radarr:7878` (if on the same Docker network) or `http://<radarr-host-ip>:7878` (or reverse proxy URL).
   - **API Key**: Paste the Radarr API Key.
   - **Quality Profile ID** & **Root Folder Path**: Select from dropdowns.

### Sonarr (TV Series & Anime):
1. Open **Sonarr** ➔ **Settings** ➔ **General** ➔ **Security**.
2. Copy the **API Key**.
3. In Trackarr **Admin ➔ Arr Stack**:
   - **Sonarr URL**: `http://sonarr:8989` (Docker network) or `http://<sonarr-host-ip>:8989` (or reverse proxy URL).
   - **API Key**: Paste the Sonarr API Key.
   - **Standard Profile** & **Anime Profile**: Select preferred quality profiles.

### Prowlarr (Releases Feed):
1. Open **Prowlarr** ➔ **Settings** ➔ **General** ➔ **Security**.
2. Copy the **API Key**.
3. In Trackarr **Admin ➔ Arr Stack**:
   - **Prowlarr URL**: `http://prowlarr:9696` (Docker network) or `http://<prowlarr-host-ip>:9696` (or reverse proxy URL).
   - **API Key**: Paste the Prowlarr API Key to enable the **Releases** feed.

---

## 6. Web Push (VAPID Keys)

For browser push notifications (e.g. notifications when a new episode airs or when background tasks complete):

Generate a VAPID key pair:
```bash
# Using npx web-push
npx web-push generate-vapid-keys
```

Configure in `.env`:
```env
VAPID_PUBLIC_KEY=your_generated_public_key
VAPID_PRIVATE_KEY=your_generated_private_key
VAPID_SUBJECT=mailto:your-email@example.com
```
