# TVDB Cross-Reference & ID Conflict Detection

## Context

TVDB's extended API endpoints return a `remoteIds` array that maps the title to its equivalents on other databases: sourceId=2 is IMDB, sourceId=5 is TMDB. We currently use this only to extract the IMDB ID as a fallback filler. Two things are missing:

1. **TMDB ID back-fill** — if we found a title via TVDB but have no TMDB ID yet, TVDB's `remoteIds` may give it to us.
2. **Conflict detection** — if TMDB says IMDB=tt123 but TVDB's `remoteIds` says IMDB=tt456, the two sources disagree → the match is likely wrong. Same logic applies to the TMDB ID itself: if Plex gave us TMDB ID 123, but the TVDB title's `remoteIds` says the TMDB counterpart is 456, those IDs don't belong to the same title.

A confirmed match where two independent authoritative sources agree on cross-IDs is much more trustworthy than one that doesn't. Conversely, a conflict is a reliable signal to surface for human review.

No conflict detection exists today. `MatchStatusPendingReview` is only set by Gemini confidence scoring.

---

## Behavior After This Change

- **ID back-fill**: after fetching TVDB, if `result.TMDBID` is still 0 and TVDB's `remoteIds` contains a TMDB ID, it is stored.
- **IMDB conflict**: if both TMDB and TVDB independently returned an IMDB ID and they differ → log the conflict, downgrade `MatchStatus` to `pending_review`.
- **TMDB ID conflict**: if we already have a TMDB ID (from Plex or TMDB search) and TVDB's `remoteIds` maps to a different TMDB ID → log the conflict, downgrade to `pending_review`.
- **No conflict**: IDs agree or only one source provided the ID → no change to status, back-fill proceeds silently.

---

## Files to Change

### `internal/service/matching/tvdb_details.go`

Add two extractor functions after `extractMovieIMDB` (line 177):

```
extractSeriesTMDB(d *tvdbSeriesDetail) int64
extractMovieTMDB(d *tvdbMovieDetail) int64
```

Both iterate `RemoteIDs`, match `sourceId == 5`, and parse the string ID to `int64` via `strconv.ParseInt`. Returns 0 on failure.

### `internal/service/matching/pipeline.go`

**`tvdbFetchResult` struct** — add `tmdbID int64` field.

**`fetchTVDBData` movie branch** — after `out.imdbID = extractMovieIMDB(details)`, add `out.tmdbID = extractMovieTMDB(details)`.

**`fetchTVDBData` series branch** — same, using `extractSeriesTMDB`.

**Fusion block in `enrichFromIDs`** (after the existing TVDB ID back-fill at line 493), add three new blocks in order:

1. **TMDB ID back-fill from TVDB**: `if result.TMDBID == 0 && tvdbRes.tmdbID != 0 → result.TMDBID = tvdbRes.tmdbID`

2. **IMDB conflict check**: if `tmdbRes.imdbID != "" && tvdbRes.imdbID != ""` and they differ → log + downgrade to `pending_review`.

3. **TMDB ID conflict check**: if `result.TMDBID != 0 && tvdbRes.tmdbID != 0` and they differ → log + downgrade to `pending_review`. (Only runs when we had a TMDB ID before the back-fill, i.e. `result.TMDBID` was already set by TMDB/Plex.)

---

## Verification

1. `make test` — all existing tests pass.
2. `make build` — clean compile.
3. Manual: trigger a Refresh on a title that has both TMDB and TVDB IDs — confirm no false-positive conflict is logged in `make logs`.
4. To verify conflict detection fires: temporarily swap a TVDB ID to a mismatched title in the DB, run Refresh, confirm `match_status` becomes `pending_review` and the conflict appears in logs.
