# Arr (Radarr & Sonarr) Technical Specification

[← Back to Index](../INDEX.md)

---

## Architecture & Source Files
- Service & Proxy: `internal/service/arr.go` (`ArrService`)
- Background Worker: `internal/service/background_arr.go` (`handleArrPush`, `saveArrID`)
- HTTP Handler: `internal/handler/arr.go` (`ArrHandler`)
- Router Endpoints: `internal/router/router.go` (`/api/arr/*`, `/api/admin/arr`)
- Frontend Components: `frontend/src/components/ArrBadge.tsx`, `frontend/src/components/ArrPushSheet.tsx`, `frontend/src/components/TypeBadge.tsx`
- Frontend Pages: `frontend/src/pages/AdminArr.tsx`

## Configuration Sources
Settings are loaded with fallback priority:
1. Environment variables: `RADARR_URL`, `RADARR_API_KEY`, `SONARR_URL`, `SONARR_API_KEY`.
2. Database settings: `radarr_url`, `radarr_api_key`, `sonarr_url`, `sonarr_api_key`, `radarr_root_folder`, `radarr_quality_profile`, `sonarr_root_folder`, `sonarr_quality_profile`.

## Push Workflow & ID Lookup
When a title is pushed directly from the title detail drawer (`ArrPushSheet` via `POST /api/arr/push/:id`):
1. **Radarr**: Lookup term `term=tmdb:<tmdb_id>` against `/api/v3/movie/lookup`.
2. **Sonarr**: Lookup term `term=tvdb:<tvdb_id>` against `/api/v3/series/lookup`.
3. If entry already exists in Arr:
   - Does not overwrite values in Arr (Arr values prevail); saves the Arr ID directly.
4. If entry is new:
   - Posts add payload via `POST /api/v3/movie` or `POST /api/v3/series` with `addOptions{searchForMovie: true, searchForMissingEpisodes: true}`.
5. Persists the returned ID:
   - Sets `titles.radarr_id = ?` or `titles.sonarr_id = ?`.

## Frontend Status Representation
- **Cards (PosterCard / TitleCard)**: `TypeBadge.tsx` displays colored top accent borders (`#ffc230` yellow for Radarr, `#00c0ff` cyan for Sonarr).
- **Badges**: `ArrBadge.tsx` renders current state:
  - `In Queue`: Title actively downloading.
  - `Downloaded`: Available on disk.
  - `Monitored`: Tracked for future releases.
  - `Ignored`: User marked title to bypass Arr prompts (`arr_ignored = 1`).

## Deletion & Import List Exclusion (Dropped Series)
When a TV/anime series is marked as `dropped` in Trackarr and the user opts to delete from Sonarr:
1. `EditSheet` sends `PATCH /api/titles/:id` with `status: "dropped"` and `delete_from_sonarr: true`.
2. Backend clears `titles.sonarr_id = NULL`, records `titles.sonarr_deleted_at = CURRENT_TIMESTAMP`, sets `titles.arr_ignored = 1`, and enqueues task `sonarr_delete`.
3. Background worker executes `DELETE /api/v3/series/{id}?deleteFiles=true&addImportListExclusion=true`:
   - `deleteFiles=true`: Purges series media files from disk.
   - `addImportListExclusion=true`: Registers the series with Sonarr's Import List Exclusions, preventing any collection or automated import list from re-adding it.

## Re-adding Previously Deleted Series to Sonarr
When a series has `sonarr_deleted_at != null` and `sonarr_id == null`:
1. **TitleDetail**: Instead of the standard "Send to Sonarr" ("Envoyer à Sonarr") button, the row displays a "Deleted" ("Supprimée") badge alongside a styled button "Re-add to Sonarr" ("Réajouter à Sonarr").
2. **Intent Validation**: Clicking "Re-add to Sonarr" ("Réajouter à Sonarr") displays a `ConfirmationDrawer` modal prompting the user to confirm their intent to re-add the previously deleted and excluded series.
3. **Configuration & Push**: Upon confirmation, `ArrPushSheet` opens with root folder, quality profile, and monitoring options, displaying a notice that the series was previously deleted.
4. **Execution & Cleanup**: When submitted, `ArrService.PushTitle` pushes the series to Sonarr, clears `titles.sonarr_deleted_at = NULL`, resets `titles.arr_ignored = 0`, updates `titles.sonarr_id`, and purges any matching exclusion in Sonarr via `DELETE /api/v3/importlistexclusion/{id}`.

## Rematch & External ID Modification (Unlink & Arr Deletion)
When rematching or editing external IDs on a title previously linked to Sonarr or Radarr:
1. **Re-derivation of Dependent External IDs**:
   - On TMDB rematch without explicit `tvdb_id`, Trackarr clears existing `tvdb_id` and `imdb_id` so that fresh values can be discovered from TMDB external IDs, avoiding retention of stale third-party IDs.
2. **Unlinking Arr Association**:
   - Any rematch or external ID modification unlinks the title (`ClearSonarrID = true`, `ClearRadarrID = true`, `ClearSonarrDeletedAt = true`, `arr_ignored = 0`).
3. **User Choice & Arr Deletion (`ArrUnlinkDrawer`)**:
   - If the title had `sonarr_id` or `radarr_id` set, the UI displays `ArrUnlinkDrawer` offering:
     - **Delete from {App} & Rematch**: Removes the entry from Sonarr (`DELETE /api/v3/series/{id}`) or Radarr (`DELETE /api/v3/movie/{id}`) via `DeleteTitleFromArr`, clears Trackarr's links, and performs the rematch.
     - **Unlink only & Rematch**: Retains the existing entry in Sonarr/Radarr, clears Trackarr's links, and rematches.
     - **Cancel**: Aborts without modifying the title.

## Live Status & Stale Link Detection (`ArrPushSheet`)
When opening `ArrPushSheet`:
1. Trackarr fetches live state from Sonarr/Radarr via `GET /api/arr/title/:id`.
2. If Trackarr has `sonarr_id` or `radarr_id` recorded on the title but the live check returns `exists: false` (the entry was deleted directly in Sonarr/Radarr):
   - `ArrPushSheet` warns the user with an alert banner (`staleEntryNotice`).
   - The action button automatically reflects addition ("Send to {App}") rather than update ("Update in {App}").
   - `ArrPushSheet` invokes `onUnlinked()` so parent components (e.g. `TitleDetail`) immediately reset local `sonarr_id` / `radarr_id` state without requiring a full manual refresh.



