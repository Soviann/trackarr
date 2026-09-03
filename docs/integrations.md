# Integrations & Webhooks

[← Back to Index](INDEX.md)

This guide covers real-time webhooks, media server scrobbling (Jellyfin / Plex), AniList bidirectional sync, and the Radarr / Sonarr (*arr) stack integration.

---

## 1. Jellyfin & Plex Webhook Scrobbling

Trackarr logs media consumption automatically as you watch. When a movie or episode reaches the end, your media server sends a webhook event.

### Jellyfin Webhook Configuration:
1. In Jellyfin, install the **Webhook** plugin (*Dashboard ➔ Plugins ➔ Catalog ➔ Webhook*), then restart Jellyfin.
2. Navigate to *Dashboard ➔ Plugins ➔ Webhook ➔ Add Generic Destination*.
3. Configure the destination:
   - **Webhook URL**: `https://<your-trackarr-url>/api/webhook/jellyfin/<secret>` (where `<secret>` matches `JELLYFIN_WEBHOOK_SECRET` or your configured token).
   - **Notification Type**: Check **Playback Stop** only.
   - **Item Type**: Check **Movies** and **Episodes**.
   - **Send All Properties**: Leave unchecked.
   - **Request Header**: `Content-Type: application/json`.
   - **JSON Template**:
     ```handlebars
     {
       "notification_type": "{{NotificationType}}",
       "item_type": "{{ItemType}}",
       "name": "{{{Name}}}",
       "year": "{{Year}}",
       "played_to_completion": "{{PlayedToCompletion}}",
       "provider_imdb": "{{Provider_imdb}}",
       "provider_tmdb": "{{Provider_tmdb}}",
       "provider_tvdb": "{{Provider_tvdb}}",
       "item_id": "{{ItemId}}",
       "series_name": "{{{SeriesName}}}",
       "series_id": "{{SeriesId}}",
       "season": "{{SeasonNumber}}",
       "episode": "{{EpisodeNumber}}"
     }
     ```

### Plex Webhook Configuration:
1. In Plex Web, go to **Settings ➔ Webhooks ➔ Add Webhook**.
2. Set URL to `https://<your-trackarr-url>/api/webhook/plex/<secret>` (where `<secret>` matches `PLEX_WEBHOOK_SECRET` or your configured token).
3. Scrobble events (`media.scrobble`) will be parsed automatically.

---

## 2. AniList Synchronization

Trackarr maintains continuous, bidirectional synchronization with AniList:
- **Per-Season Mapping**: Since AniList treats anime seasons as distinct media entries, Trackarr tracks each season under its dedicated AniList ID while presenting the series unified.
- **Progress Sync**: Marking an episode watched in Trackarr immediately advances episode progress and status on AniList (*CURRENT*, *COMPLETED*).
- **Ratings Synchronization**: Scores (1–10) are pushed automatically to AniList when an anime season is marked **Completed** or **Dropped** (as required by AniList's API rules).
- **Multi-Part Seasons**: For anime seasons split into multiple AniList entries (e.g. *Part 1*, *Part 2*), tap the ✎ icon on the Season AniList strip to attach multiple AniList entries; Trackarr automatically balances watched episode counts across parts.

---

## 3. Radarr & Sonarr (*arr Stack)

Manage your media library and track availability seamlessly:

### Configuration (`/admin/arr` & `/admin/system-settings`):
- **Radarr**: Configure URL (`http://radarr:7878`), API Key, default Root Folder, and Quality Profile.
- **Sonarr**: Configure URL (`http://sonarr:8989`), API Key, Standard Profile, Anime Profile, and Root Folder.

### UI Indicators & Push:
- **Yellow Top Border**: Movie is managed and monitored in **Radarr**.
- **Cyan Top Border**: Series is managed and monitored in **Sonarr**.
- **Arr Icon Badges**: 18px indicator pills on search and list cards.
- **Direct Arr Push**: Send titles directly to Radarr or Sonarr from the title detail sheet (**Actions ➔ More ➔ Send to Radarr/Sonarr**).
- **Prowlarr Releases**: View indexer releases on `/releases` with multi-indexer filtering, year filtering, and add media directly to Trackarr with 1-click **+ Add**.
