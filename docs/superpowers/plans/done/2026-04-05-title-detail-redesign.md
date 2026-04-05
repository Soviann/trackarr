# Title Detail Page Redesign — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Redesign the title detail page to show rich TMDB metadata (synopsis, genres, runtime, cast, ratings) and consolidate all actions into a collapsible drawer.

**Architecture:** Backend adds 5 nullable columns to `titles` (overview, genres, runtime, tmdb_rating, credits), extends TMDB structs to decode new fields, and populates them during enrichment + daily refresh. Frontend replaces the empty detail page with stacked content cards and replaces ActionBar with an ActionDrawer following the FilterDrawer pattern.

**Tech Stack:** Go 1.24 / SQLite / Preact 10 / CSS Modules / TypeScript

**Spec:** `docs/superpowers/specs/2026-04-05-title-detail-redesign.md`

---

## Task 1: DB Migration — Add metadata columns

**Files:**
- Create: `internal/database/migrations/006_title_metadata.up.sql`
- Create: `internal/database/migrations/006_title_metadata.down.sql`

- [ ] **Step 1: Write up migration**

```sql
-- 006_title_metadata.up.sql
ALTER TABLE titles ADD COLUMN overview TEXT;
ALTER TABLE titles ADD COLUMN genres TEXT;
ALTER TABLE titles ADD COLUMN runtime INTEGER;
ALTER TABLE titles ADD COLUMN tmdb_rating REAL;
ALTER TABLE titles ADD COLUMN credits TEXT;
```

- [ ] **Step 2: Write down migration**

```sql
-- 006_title_metadata.down.sql
-- SQLite doesn't support DROP COLUMN before 3.35.0;
-- these are best-effort for dev environments.
ALTER TABLE titles DROP COLUMN overview;
ALTER TABLE titles DROP COLUMN genres;
ALTER TABLE titles DROP COLUMN runtime;
ALTER TABLE titles DROP COLUMN tmdb_rating;
ALTER TABLE titles DROP COLUMN credits;
```

- [ ] **Step 3: Run tests to verify migration applies cleanly**

Run: `make test`
Expected: All existing tests pass — new nullable columns don't affect existing scans (they aren't SELECTed yet).

- [ ] **Step 4: Commit**

```
feat(db): ajoute les colonnes de métadonnées TMDB aux titres
```

---

## Task 2: Model — Add fields to Title struct

**Files:**
- Modify: `internal/model/title.go:39-56`

- [ ] **Step 1: Add 5 fields to Title struct**

After `MatchSource` (line 54), before `CreatedAt` (line 55), add:

```go
Overview   *string  `json:"overview"`
Genres     *string  `json:"genres"`
Runtime    *int     `json:"runtime"`
TMDBRating *float64 `json:"tmdb_rating"`
Credits    *string  `json:"credits"`
```

Design note: `Genres` and `Credits` are stored as JSON strings (`*string`). The frontend will parse them with `JSON.parse()`. This avoids custom SQL scanner types and keeps the model simple.

- [ ] **Step 2: Run tests**

Run: `make test`
Expected: PASS — struct changes are additive, no scan breakage yet.

- [ ] **Step 3: Commit**

```
feat(model): ajoute les champs de métadonnées TMDB au modèle Title
```

---

## Task 3: Repository — Wire new fields into all queries

**Files:**
- Modify: `internal/repository/title.go:51-65` (TitleUpdate), `:82-88` (createInTx), `:107-111` (GetByID), `:180` (List baseCols), `:266-267` (List Scan), `:354` (ListAll), `:362-363` (ListAll Scan), `:435-490` (Update)
- Modify: `internal/repository/title_search.go:68` (baseCols), `:113-114` (Scan), `:319` (SELECT), `:340-341` (Scan)
- Modify: `internal/repository/title_test.go` (add tests)

- [ ] **Step 1: Write tests for metadata round-trip**

Add to `internal/repository/title_test.go`:

```go
func TestTitleRepository_MetadataRoundTrip(t *testing.T) {
	db := setupTestDB(t)
	repo := NewTitleRepository(db)

	genres := `["Science Fiction","Drama"]`
	credits := `[{"name":"Jack Arnold","role":"Director"},{"name":"Michel Ray","role":"Bud"}]`
	overview := "A mysterious brain from space..."
	runtime := 69
	tmdbRating := 5.2

	title := &model.Title{
		Type:       model.TitleTypeMovie,
		Year:       1958,
		Status:     model.TitleStatusCompleted,
		MatchStatus: model.MatchStatusConfirmed,
		Overview:   &overview,
		Genres:     &genres,
		Runtime:    &runtime,
		TMDBRating: &tmdbRating,
		Credits:    &credits,
	}

	id, err := repo.Create(title, []model.TitleName{{Name: "The Space Children", Language: "en", IsPrimary: true}})
	require.NoError(t, err)

	got, err := repo.GetByID(id)
	require.NoError(t, err)
	assert.Equal(t, &overview, got.Overview)
	assert.Equal(t, &genres, got.Genres)
	assert.Equal(t, &runtime, got.Runtime)
	assert.Equal(t, &tmdbRating, got.TMDBRating)
	assert.Equal(t, &credits, got.Credits)
}

func TestTitleRepository_UpdateMetadata(t *testing.T) {
	db := setupTestDB(t)
	repo := NewTitleRepository(db)

	title := &model.Title{
		Type:       model.TitleTypeMovie,
		Year:       1958,
		Status:     model.TitleStatusCompleted,
		MatchStatus: model.MatchStatusConfirmed,
	}

	id, err := repo.Create(title, []model.TitleName{{Name: "Test", Language: "en", IsPrimary: true}})
	require.NoError(t, err)

	overview := "Updated overview"
	genres := `["Action"]`
	runtime := 120
	tmdbRating := 7.5
	credits := `[{"name":"Director","role":"Director"}]`

	err = repo.Update(id, TitleUpdate{
		Overview:   &overview,
		Genres:     &genres,
		Runtime:    &runtime,
		TMDBRating: &tmdbRating,
		Credits:    &credits,
	})
	require.NoError(t, err)

	got, err := repo.GetByID(id)
	require.NoError(t, err)
	assert.Equal(t, &overview, got.Overview)
	assert.Equal(t, &genres, got.Genres)
	assert.Equal(t, &runtime, got.Runtime)
	assert.Equal(t, &tmdbRating, got.TMDBRating)
	assert.Equal(t, &credits, got.Credits)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `make test`
Expected: FAIL — columns not in SELECT/Scan yet.

- [ ] **Step 3: Add fields to TitleUpdate struct**

In `title.go:51-65`, add after `Type`:

```go
Overview   *string
Genres     *string
Runtime    *int
TMDBRating *float64
Credits    *string
```

- [ ] **Step 4: Update createInTx INSERT**

In `title.go:82-88`, add 5 columns and values:

```go
res, err := db.Exec(`
    INSERT INTO titles (type, year, cover_url, imdb_id, anilist_id, tmdb_id, tvdb_id, plex_rating_key, my_rating, status, series_status, match_status, original_title, match_source, overview, genres, runtime, tmdb_rating, credits)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
    title.Type, title.Year, title.CoverURL, title.IMDBID, title.AniListID, title.TMDBID, title.TVDBID,
    title.PlexRatingKey, title.MyRating, title.Status, title.SeriesStatus, title.MatchStatus,
    title.OriginalTitle, title.MatchSource,
    title.Overview, title.Genres, title.Runtime, title.TMDBRating, title.Credits,
)
```

- [ ] **Step 5: Update GetByID SELECT + Scan**

In `title.go:109-111`:

```go
err := r.db.QueryRow(`SELECT id, type, year, cover_url, imdb_id, anilist_id, tmdb_id, tvdb_id, plex_rating_key, my_rating, status, series_status, match_status, original_title, match_source, overview, genres, runtime, tmdb_rating, credits, created_at, updated_at FROM titles WHERE id = ?`, id).
    Scan(&title.ID, &title.Type, &title.Year, &title.CoverURL, &title.IMDBID, &title.AniListID, &title.TMDBID, &title.TVDBID,
        &title.PlexRatingKey, &title.MyRating, &title.Status, &title.SeriesStatus, &title.MatchStatus, &title.OriginalTitle, &title.MatchSource,
        &title.Overview, &title.Genres, &title.Runtime, &title.TMDBRating, &title.Credits,
        &title.CreatedAt, &title.UpdatedAt)
```

- [ ] **Step 6: Update List baseCols + Scan**

In `title.go:180`:

```go
baseCols := `t.id, t.type, t.year, t.cover_url, t.imdb_id, t.anilist_id, t.tmdb_id, t.tvdb_id, t.plex_rating_key, t.my_rating, t.status, t.series_status, t.match_status, t.original_title, t.match_source, t.overview, t.genres, t.runtime, t.tmdb_rating, t.credits, t.created_at, t.updated_at`
```

In `title.go:266-267`:

```go
if err := rows.Scan(&t.ID, &t.Type, &t.Year, &t.CoverURL, &t.IMDBID, &t.AniListID, &t.TMDBID, &t.TVDBID,
    &t.PlexRatingKey, &t.MyRating, &t.Status, &t.SeriesStatus, &t.MatchStatus, &t.OriginalTitle, &t.MatchSource,
    &t.Overview, &t.Genres, &t.Runtime, &t.TMDBRating, &t.Credits,
    &t.CreatedAt, &t.UpdatedAt); err != nil {
```

- [ ] **Step 7: Update ListAll SELECT + Scan**

In `title.go:354`:

```go
rows, err := r.db.Query(`SELECT id, type, year, cover_url, imdb_id, anilist_id, tmdb_id, tvdb_id, plex_rating_key, my_rating, status, series_status, match_status, original_title, match_source, overview, genres, runtime, tmdb_rating, credits, created_at, updated_at FROM titles ORDER BY updated_at DESC`)
```

In `title.go:362-363`:

```go
if err := rows.Scan(&t.ID, &t.Type, &t.Year, &t.CoverURL, &t.IMDBID, &t.AniListID, &t.TMDBID, &t.TVDBID,
    &t.PlexRatingKey, &t.MyRating, &t.Status, &t.SeriesStatus, &t.MatchStatus, &t.OriginalTitle, &t.MatchSource,
    &t.Overview, &t.Genres, &t.Runtime, &t.TMDBRating, &t.Credits,
    &t.CreatedAt, &t.UpdatedAt); err != nil {
```

- [ ] **Step 8: Update title_search.go — baseCols (line 68)**

```go
baseCols := `t.id, t.type, t.year, t.cover_url, t.imdb_id, t.anilist_id, t.tmdb_id, t.tvdb_id, t.plex_rating_key, t.my_rating, t.status, t.series_status, t.match_status, t.original_title, t.match_source, t.overview, t.genres, t.runtime, t.tmdb_rating, t.credits, t.created_at, t.updated_at`
```

Update Scan at line 113-114 (adds matched_name/matched_lang after the base cols):

```go
if err := rows.Scan(&t.ID, &t.Type, &t.Year, &t.CoverURL, &t.IMDBID, &t.AniListID, &t.TMDBID, &t.TVDBID,
    &t.PlexRatingKey, &t.MyRating, &t.Status, &t.SeriesStatus, &t.MatchStatus, &t.OriginalTitle, &t.MatchSource,
    &t.Overview, &t.Genres, &t.Runtime, &t.TMDBRating, &t.Credits,
    &t.CreatedAt, &t.UpdatedAt,
    &matchedName, &matchedLang); err != nil {
```

- [ ] **Step 9: Update title_search.go — fuzzy search SELECT (line 319) + Scan (line 340-341)**

```go
query := `SELECT t.id, t.type, t.year, t.cover_url, t.imdb_id, t.anilist_id, t.tmdb_id, t.tvdb_id, t.plex_rating_key, t.my_rating, t.status, t.series_status, t.match_status, t.original_title, t.match_source, t.overview, t.genres, t.runtime, t.tmdb_rating, t.credits, t.created_at, t.updated_at FROM titles t WHERE t.id IN (` + placeholders + `)`
```

```go
if err := tRows.Scan(&t.ID, &t.Type, &t.Year, &t.CoverURL, &t.IMDBID, &t.AniListID, &t.TMDBID, &t.TVDBID,
    &t.PlexRatingKey, &t.MyRating, &t.Status, &t.SeriesStatus, &t.MatchStatus, &t.OriginalTitle, &t.MatchSource,
    &t.Overview, &t.Genres, &t.Runtime, &t.TMDBRating, &t.Credits,
    &t.CreatedAt, &t.UpdatedAt); err != nil {
```

- [ ] **Step 10: Add Update handlers for 5 new fields**

In `title.go` Update method, after the `Type` handler (line ~490):

```go
if update.Overview != nil {
    sets = append(sets, `overview = ?`)
    args = append(args, *update.Overview)
}
if update.Genres != nil {
    sets = append(sets, `genres = ?`)
    args = append(args, *update.Genres)
}
if update.Runtime != nil {
    sets = append(sets, `runtime = ?`)
    args = append(args, *update.Runtime)
}
if update.TMDBRating != nil {
    sets = append(sets, `tmdb_rating = ?`)
    args = append(args, *update.TMDBRating)
}
if update.Credits != nil {
    sets = append(sets, `credits = ?`)
    args = append(args, *update.Credits)
}
```

- [ ] **Step 11: Run tests**

Run: `make test`
Expected: PASS — all existing + new metadata tests pass.

- [ ] **Step 12: Commit**

```
feat(repository): propage les champs de métadonnées TMDB dans toutes les requêtes
```

---

## Task 4: TMDB Structs — Extend to decode new fields

**Files:**
- Modify: `internal/service/matching/tmdb_details.go:8-34` (structs), `:60-76` (fetch methods)

- [ ] **Step 1: Add new types for TMDB credits**

At the top of `tmdb_details.go`, add:

```go
type TMDBGenre struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type TMDBCredits struct {
	Cast []TMDBCastMember `json:"cast"`
	Crew []TMDBCrewMember `json:"crew"`
}

type TMDBCastMember struct {
	Name      string `json:"name"`
	Character string `json:"character"`
	Order     int    `json:"order"`
}

type TMDBCrewMember struct {
	Name       string `json:"name"`
	Job        string `json:"job"`
	Department string `json:"department"`
}
```

- [ ] **Step 2: Extend TMDBMovieDetails**

```go
type TMDBMovieDetails struct {
	ID          int64        `json:"id"`
	Title       string       `json:"title"`
	Overview    string       `json:"overview"`
	ReleaseDate string       `json:"release_date"`
	PosterPath  *string      `json:"poster_path"`
	IMDBID      string       `json:"imdb_id"`
	Genres      []TMDBGenre  `json:"genres"`
	Runtime     *int         `json:"runtime"`
	VoteAverage float64      `json:"vote_average"`
	Credits     *TMDBCredits `json:"credits"`
	ExternalIDs *struct {
		IMDBID string `json:"imdb_id"`
		TVDBID int64  `json:"tvdb_id"`
	} `json:"external_ids"`
}
```

- [ ] **Step 3: Extend TMDBTVDetails**

```go
type TMDBTVDetails struct {
	ID             int64        `json:"id"`
	Name           string       `json:"name"`
	Overview       string       `json:"overview"`
	Status         string       `json:"status"`
	FirstAirDate   string       `json:"first_air_date"`
	PosterPath     *string      `json:"poster_path"`
	Genres         []TMDBGenre  `json:"genres"`
	EpisodeRunTime []int        `json:"episode_run_time"`
	VoteAverage    float64      `json:"vote_average"`
	Credits        *TMDBCredits `json:"credits"`
	Seasons        []struct {
		SeasonNumber int `json:"season_number"`
		EpisodeCount int `json:"episode_count"`
	} `json:"seasons"`
	ExternalIDs *struct {
		IMDBID string `json:"imdb_id"`
		TVDBID int64  `json:"tvdb_id"`
	} `json:"external_ids"`
}
```

- [ ] **Step 4: Append credits to API calls**

In `GetMovieDetails` (line 62), change:

```go
params := url.Values{"append_to_response": {"external_ids,credits"}}
```

In `GetTVDetails` (line 71), change:

```go
params := url.Values{"append_to_response": {"external_ids,credits"}}
```

- [ ] **Step 5: Add helper to extract metadata from details**

Add to `tmdb_details.go`:

```go
// ExtractMetadata builds JSON strings for genres and credits from TMDB details.
func ExtractMovieMetadata(d *TMDBMovieDetails) (genres, credits string, runtime *int, rating *float64) {
	genres = marshalGenres(d.Genres)
	credits = marshalCredits(d.Credits)
	if d.Runtime != nil && *d.Runtime > 0 {
		runtime = d.Runtime
	}
	if d.VoteAverage > 0 {
		rating = &d.VoteAverage
	}
	return
}

func ExtractTVMetadata(d *TMDBTVDetails) (genres, credits string, runtime *int, rating *float64) {
	genres = marshalGenres(d.Genres)
	credits = marshalCredits(d.Credits)
	if len(d.EpisodeRunTime) > 0 && d.EpisodeRunTime[0] > 0 {
		runtime = &d.EpisodeRunTime[0]
	}
	if d.VoteAverage > 0 {
		rating = &d.VoteAverage
	}
	return
}

func marshalGenres(genres []TMDBGenre) string {
	names := make([]string, 0, len(genres))
	for _, g := range genres {
		names = append(names, g.Name)
	}
	b, _ := json.Marshal(names)
	return string(b)
}

func marshalCredits(c *TMDBCredits) string {
	if c == nil {
		return "[]"
	}
	type entry struct {
		Name string `json:"name"`
		Role string `json:"role"`
	}
	var entries []entry
	// Director(s) first
	for _, crew := range c.Crew {
		if crew.Job == "Director" {
			entries = append(entries, entry{Name: crew.Name, Role: "Director"})
		}
	}
	// Top 5 cast by order
	limit := 5
	if len(c.Cast) < limit {
		limit = len(c.Cast)
	}
	for _, cast := range c.Cast[:limit] {
		entries = append(entries, entry{Name: cast.Name, Role: cast.Character})
	}
	b, _ := json.Marshal(entries)
	return string(b)
}
```

Add `"encoding/json"` to the imports.

- [ ] **Step 6: Run tests**

Run: `make test`
Expected: PASS

- [ ] **Step 7: Commit**

```
feat(tmdb): étend les structures de détails pour synopsis, genres, durée, note et casting
```

---

## Task 5: Enrichment Pipeline — Populate new MatchResult fields

**Files:**
- Modify: `internal/service/matching/pipeline.go:51-61` (MatchResult), `:265-339` (enrichFromIDs), `:341-365` (downloadCover)

- [ ] **Step 1: Add metadata fields to MatchResult**

In `pipeline.go:51-61`, add:

```go
type MatchResult struct {
	IMDBID      string
	TMDBID      int64
	TVDBID      int64
	AniListID   int64
	MatchStatus model.MatchStatus
	MatchSource string
	Names       []model.TitleName
	CoverFile   string
	TitleType   model.TitleType
	// TMDB metadata
	Overview   string
	Genres     string  // JSON array
	Runtime    *int
	TMDBRating *float64
	Credits    string  // JSON array
}
```

- [ ] **Step 2: Refactor downloadCover to return details and avoid double API call**

Replace `downloadCover` with `fetchTMDBDetailsAndCover` that returns the fetched details:

```go
func (p *Pipeline) fetchTMDBDetailsAndCover(result *MatchResult) {
	if result.TitleType == model.TitleTypeMovie {
		details, err := p.tmdb.GetMovieDetails(result.TMDBID)
		if err != nil {
			log.Printf("fetch movie details failed: %v", err)
			return
		}
		// Extract metadata
		result.Overview = details.Overview
		genres, credits, runtime, rating := matching.ExtractMovieMetadata(details)
		result.Genres = genres
		result.Credits = credits
		result.Runtime = runtime
		result.TMDBRating = rating
		// Download cover
		if details.PosterPath != nil && *details.PosterPath != "" {
			p.downloadPoster(*details.PosterPath, result)
		}
	} else {
		details, err := p.tmdb.GetTVDetails(result.TMDBID)
		if err != nil {
			log.Printf("fetch tv details failed: %v", err)
			return
		}
		result.Overview = details.Overview
		genres, credits, runtime, rating := matching.ExtractTVMetadata(details)
		result.Genres = genres
		result.Credits = credits
		result.Runtime = runtime
		result.TMDBRating = rating
		if details.PosterPath != nil && *details.PosterPath != "" {
			p.downloadPoster(*details.PosterPath, result)
		}
	}
}

func (p *Pipeline) downloadPoster(posterPath string, result *MatchResult) {
	coversDir := fmt.Sprintf("%s/covers", p.dataDir)
	filename, err := p.tmdb.DownloadCover(posterPath, coversDir)
	if err != nil {
		log.Printf("download cover failed: %v", err)
		return
	}
	result.CoverFile = filename
}
```

- [ ] **Step 3: Update enrichFromIDs to call the new method**

In `enrichFromIDs` (line 332-335), replace:

```go
// Download cover: try TMDB first, fallback to AniList
if p.tmdb != nil && result.TMDBID != 0 {
    p.downloadCover(result)
}
```

with:

```go
// Fetch TMDB details + metadata + cover
if p.tmdb != nil && result.TMDBID != 0 {
    p.fetchTMDBDetailsAndCover(result)
}
```

Remove the old `downloadCover` function.

- [ ] **Step 4: Run tests**

Run: `make test`
Expected: PASS

- [ ] **Step 5: Commit**

```
feat(pipeline): extrait les métadonnées TMDB lors de l'enrichissement
```

---

## Task 6: Task Queue + Background Refresh — Pass metadata to repository

**Files:**
- Modify: `internal/service/taskqueue.go:180-203` (handleEnrichment)
- Modify: `internal/service/background.go:141-162` (refreshMovieFromTMDB), `:164-235` (refreshSeriesFromTMDB)

- [ ] **Step 1: Update handleEnrichment in taskqueue.go**

After the existing `update` fields (line ~203), add:

```go
if result.Overview != "" {
    update.Overview = &result.Overview
}
if result.Genres != "" {
    update.Genres = &result.Genres
}
if result.Runtime != nil {
    update.Runtime = result.Runtime
}
if result.TMDBRating != nil {
    update.TMDBRating = result.TMDBRating
}
if result.Credits != "" {
    update.Credits = &result.Credits
}
```

- [ ] **Step 2: Update refreshMovieFromTMDB in background.go**

After the cover update block (line ~156), add metadata extraction:

```go
// Update metadata from TMDB details
genres, credits, runtime, rating := matching.ExtractMovieMetadata(details)
overview := details.Overview
metaUpdate := repository.TitleUpdate{
    Overview: &overview,
    Genres:   &genres,
    Credits:  &credits,
}
if runtime != nil {
    metaUpdate.Runtime = runtime
}
if rating != nil {
    metaUpdate.TMDBRating = rating
}
_ = s.titles.Update(title.ID, metaUpdate)
```

Add import for `matching` package if not already imported.

- [ ] **Step 3: Update refreshSeriesFromTMDB in background.go**

After the cover update block (line ~204), add similar metadata extraction:

```go
// Update metadata from TMDB details
genres, credits, runtime, rating := matching.ExtractTVMetadata(details)
overview := details.Overview
metaUpdate := repository.TitleUpdate{
    Overview: &overview,
    Genres:   &genres,
    Credits:  &credits,
}
if runtime != nil {
    metaUpdate.Runtime = runtime
}
if rating != nil {
    metaUpdate.TMDBRating = rating
}
_ = s.titles.Update(title.ID, metaUpdate)
```

- [ ] **Step 4: Run tests**

Run: `make test`
Expected: PASS

- [ ] **Step 5: Commit**

```
feat(background): propage les métadonnées TMDB lors du rafraîchissement quotidien
```

---

## Task 7: AniList — Add averageScore to details query + store in Title

**Files:**
- Modify: `internal/service/matching/anilist_search.go:53-64` (query), `:104-143` (GetAnimeDetails)
- Modify: `internal/model/title.go` (add AniListRating)
- Modify: `internal/database/migrations/006_title_metadata.up.sql` (add anilist_rating column)
- Modify: All repository SELECT/Scan locations (same as Task 3 pattern)

- [ ] **Step 1: Add `anilist_rating` to migration 006**

Add to `006_title_metadata.up.sql`:

```sql
ALTER TABLE titles ADD COLUMN anilist_rating INTEGER;
```

And to `006_title_metadata.down.sql`:

```sql
ALTER TABLE titles DROP COLUMN anilist_rating;
```

- [ ] **Step 2: Add AniListRating to Title model**

In `internal/model/title.go`, add after `Credits`:

```go
AniListRating *int `json:"anilist_rating"`
```

- [ ] **Step 3: Add to TitleUpdate, all SELECT/Scan, and Update method**

Same pattern as Task 3 — add `anilist_rating` column to every SELECT, Scan, Insert, and Update location in `title.go` and `title_search.go`. Add `AniListRating *int` to `TitleUpdate`.

- [ ] **Step 4: Add averageScore to GraphQL query**

Update `getAnimeDetailsQuery` (line 53-64):

```go
const getAnimeDetailsQuery = `
query ($id: Int) {
  Media(id: $id, type: ANIME) {
    id
    idMal
    title { romaji english }
    episodes
    format
    seasonYear
    averageScore
    coverImage { extraLarge large }
  }
}
`
```

- [ ] **Step 5: Add AverageScore to response struct and AniListDetails**

In `GetAnimeDetails` function (line 104), add to the response struct:

```go
AverageScore *int `json:"averageScore"`
```

Find `AniListDetails` struct and add:

```go
AverageScore *int
```

In the return statement, add:

```go
AverageScore: resp.Media.AverageScore,
```

- [ ] **Step 6: Store AniList score in pipeline enrichment**

In `enrichFromIDs` (pipeline.go), after fetching AniList details for cover (line ~368-375), also capture the score:

```go
if details.AverageScore != nil {
    result.AniListRating = details.AverageScore
}
```

Add `AniListRating *int` to `MatchResult` struct.

In `handleEnrichment` (taskqueue.go), pass it through:

```go
if result.AniListRating != nil {
    update.AniListRating = result.AniListRating
}
```

- [ ] **Step 7: Run tests**

Run: `make test`
Expected: PASS

- [ ] **Step 8: Commit**

```
feat(anilist): récupère et stocke le score moyen AniList
```

---

## Task 8: Frontend Types — Add new fields to Title interface

**Files:**
- Modify: `frontend/src/types.ts:6-26` (Title interface)

- [ ] **Step 1: Add metadata fields to Title interface**

After `match_source` (line 20):

```ts
overview: string | null
genres: string | null     // JSON string: '["Action","Drama"]'
runtime: number | null
tmdb_rating: number | null
credits: string | null    // JSON string: '[{"name":"...","role":"..."}]'
anilist_rating: number | null
created_at: string
updated_at: string
```

- [ ] **Step 2: Run tests**

Run: `make test-front`
Expected: PASS — type additions are backward-compatible.

- [ ] **Step 3: Commit**

```
feat(types): ajoute les champs de métadonnées TMDB au type Title
```

---

## Task 9: ActionDrawer Component — New collapsible drawer

**Files:**
- Create: `frontend/src/components/ActionDrawer.tsx`
- Create: `frontend/src/components/ActionDrawer.module.css`

- [ ] **Step 1: Create ActionDrawer.module.css**

Follow the FilterDrawer.module.css pattern:

```css
.container {
  position: fixed;
  bottom: 69px;
  left: 0;
  right: 0;
  z-index: 99;
}

.handle {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 6px 0 4px;
  cursor: pointer;
  border-top: 1px solid var(--color-border-subtle);
  background: var(--color-bg-primary);
  -webkit-tap-highlight-color: transparent;
}

.handleBar {
  width: 28px;
  height: 3px;
  border-radius: 2px;
  background: #333;
}

.handleText {
  font-size: 8px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: var(--color-text-dimmed);
}

.chevron {
  font-size: 8px;
  color: var(--color-text-dimmed);
  transition: transform 0.2s;
}

.chevronOpen {
  transform: rotate(180deg);
}

.drawer {
  overflow: hidden;
  background: var(--color-bg-primary);
  transition: max-height 0.25s ease, opacity 0.2s ease;
}

.drawerCollapsed {
  max-height: 0;
  opacity: 0;
}

.drawerExpanded {
  max-height: 200px;
  opacity: 1;
}

.sectionLabel {
  font-size: 9px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: #B0B0B0;
  padding: 6px var(--space-lg) 2px;
}

.sectionLabelFirst {
  padding-top: 4px;
}

.actionRow {
  display: flex;
  gap: 8px;
  padding: 4px var(--space-lg);
}

.actionBtn {
  flex: 1;
  text-align: center;
  padding: 10px 4px;
  border-radius: var(--radius-md);
  font-size: 12px;
  font-weight: 600;
  border: none;
  cursor: pointer;
  font-family: inherit;
  -webkit-tap-highlight-color: transparent;
}

.rate {
  composes: actionBtn;
  background: var(--wash-amber);
  color: var(--color-accent-amber);
}

.imdb {
  composes: actionBtn;
  background: #F5C5181F;
  color: var(--color-accent-imdb);
}

.anilist {
  composes: actionBtn;
  background: #02A9FF1F;
  color: var(--color-accent-anilist);
}

.markNext {
  composes: actionBtn;
  background: var(--wash-coral);
  color: var(--color-accent-coral);
}

.manage {
  composes: actionBtn;
  background: var(--color-bg-surface);
  color: var(--color-text-muted);
}

.bottomPad {
  height: 4px;
}
```

- [ ] **Step 2: Create ActionDrawer.tsx**

```tsx
import { useState } from 'preact/hooks'
import clsx from 'clsx'
import type { Title, Episode } from '../types'
import s from './ActionDrawer.module.css'

interface ActionDrawerProps {
  title: Title
  nextEpisode: Episode | null
  nextSeasonNumber?: number
  onMarkNext?: () => void
  onRate: () => void
  onEdit: () => void
  onRematch: () => void
  onAniList?: () => void
}

export function ActionDrawer({
  title, nextEpisode, nextSeasonNumber,
  onMarkNext, onRate, onEdit, onRematch, onAniList,
}: ActionDrawerProps) {
  const [open, setOpen] = useState(false)

  const hasImdb = !!title.imdb_id
  const hasAnilist = title.type === 'anime'
  const hasSeries = title.type !== 'movie'

  return (
    <div className={s.container}>
      <div className={s.handle} onClick={() => setOpen(!open)}>
        <div className={s.handleBar} />
        <span className={s.handleText}>Actions</span>
        <span className={clsx(s.chevron, open && s.chevronOpen)}>&#9650;</span>
      </div>

      <div className={clsx(s.drawer, open ? s.drawerExpanded : s.drawerCollapsed)}>
        <div className={clsx(s.sectionLabel, s.sectionLabelFirst)}>Quick actions</div>
        <div className={s.actionRow}>
          {hasSeries && nextEpisode && (
            <button onClick={onMarkNext} className={s.markNext}>
              ✓ S{String(nextSeasonNumber ?? 1).padStart(2, '0')}E{String(nextEpisode.episode).padStart(2, '0')}
            </button>
          )}
          <button onClick={onRate} className={s.rate}>
            ★ Rate
          </button>
          {hasImdb && (
            <a
              href={`https://www.imdb.com/title/${title.imdb_id}/`}
              target="_blank"
              rel="noopener noreferrer"
              className={s.imdb}
            >
              IMDb
            </a>
          )}
          {hasAnilist && (
            <button onClick={onAniList} className={s.anilist}>
              AniList
            </button>
          )}
        </div>

        <div className={s.sectionLabel}>Manage</div>
        <div className={s.actionRow}>
          <button onClick={onEdit} className={s.manage}>
            ✎ Edit
          </button>
          <button onClick={onRematch} className={s.manage}>
            🔍 Fix match
          </button>
        </div>

        <div className={s.bottomPad} />
      </div>
    </div>
  )
}
```

- [ ] **Step 3: Run tests**

Run: `make test-front`
Expected: PASS

- [ ] **Step 4: Commit**

```
feat(frontend): ajoute le composant ActionDrawer pour la page détail
```

---

## Task 10: TitleDetail Page Rewrite

**Files:**
- Modify: `frontend/src/pages/TitleDetail.tsx` (rewrite)
- Modify: `frontend/src/pages/TitleDetail.module.css` (rewrite)

- [ ] **Step 1: Rewrite TitleDetail.module.css**

Complete rewrite. Replace the entire file content:

```css
.page {
  padding-bottom: 140px; /* space for drawer + navbar */
}

.loading {
  padding: 40px var(--space-lg);
  text-align: center;
  color: var(--color-text-secondary);
}

/* Hero — pure visual, no text */
.hero {
  position: relative;
  height: 260px;
  display: flex;
  align-items: flex-end;
}

.heroFade {
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  height: 80px;
  background: linear-gradient(transparent, var(--color-bg-primary));
}

.backBtn {
  position: absolute;
  top: 14px;
  left: 14px;
  width: 32px;
  height: 32px;
  background: rgba(0, 0, 0, 0.5);
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  border: none;
  padding: 0;
}

/* Identity zone */
.identity {
  display: flex;
  gap: 14px;
  padding: 0 var(--space-lg);
  margin-top: -32px;
  position: relative;
  z-index: 1;
}

.miniPoster {
  width: 80px;
  height: 120px;
  border-radius: var(--radius-md);
  flex-shrink: 0;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.7);
  object-fit: cover;
  border: 1px solid #2a2a2a;
}

.miniPosterPlaceholder {
  composes: miniPoster;
  display: flex;
  align-items: center;
  justify-content: center;
}

.identityInfo {
  flex: 1;
  padding-top: 36px;
}

.identityTitle {
  font-size: var(--font-size-xl);
  font-weight: 700;
  color: #fff;
  line-height: 1.2;
}

.identityMeta {
  font-size: var(--font-size-xs);
  color: #aaa;
  margin-top: 3px;
}

.genrePills {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
  margin-top: var(--space-sm);
}

.genrePill {
  font-size: var(--font-size-xs);
  padding: 3px 10px;
  border-radius: var(--radius-full);
  background: var(--color-bg-surface);
  color: #aaa;
  border: 1px solid #2a2a2a;
}

/* Content cards */
.card {
  margin: 10px var(--space-lg);
  background: var(--color-bg-card);
  border-radius: var(--radius-lg);
  padding: var(--space-md) 14px;
  border: 1px solid var(--color-border-subtle);
}

.cardLabel {
  font-size: 10px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: var(--color-text-muted);
  margin-bottom: var(--space-sm);
}

/* Ratings card */
.ratingsRow {
  display: flex;
  align-items: center;
}

.myRating {
  font-size: 28px;
  font-weight: 700;
  color: var(--color-accent-amber);
  line-height: 1;
}

.myRatingSuffix {
  font-size: 16px;
  color: var(--color-text-muted);
}

.noRating {
  font-size: 14px;
  color: var(--color-text-dimmed);
}

.extRatings {
  display: flex;
  gap: 14px;
  margin-left: auto;
}

.extItem {
  text-align: center;
}

.extScore {
  font-size: var(--font-size-md);
  font-weight: 600;
}

.extSource {
  font-size: 9px;
  color: var(--color-text-muted);
  margin-top: 1px;
}

.tmdbColor { color: var(--color-accent-teal); }
.anilistColor { color: var(--color-accent-anilist); }

/* Synopsis card */
.synopsisText {
  font-size: var(--font-size-sm);
  color: #bbb;
  line-height: 1.55;
}

.synopsisClamped {
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.synopsisToggle {
  color: var(--color-accent-amber);
  font-size: 12px;
  margin-top: var(--space-xs);
  cursor: pointer;
  background: none;
  border: none;
  padding: 0;
  font-family: inherit;
}

/* Cast list */
.castList {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.castEntry {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 12px;
}

.castPerson { color: #ccc; }
.castRole { color: var(--color-text-secondary); font-size: var(--font-size-xs); }

/* Details rows */
.detailRow {
  display: flex;
  justify-content: space-between;
  padding: 6px 0;
  font-size: 12px;
  border-bottom: 1px solid var(--color-border-subtle);
}

.detailRow:last-child { border-bottom: none; }
.detailKey { color: var(--color-text-secondary); }
.detailVal { color: #aaa; text-align: right; }

/* Series-specific (unchanged pattern) */
.progressWrap {
  padding: var(--space-md) var(--space-lg) 10px;
}

.progressTrack {
  height: 3px;
  background: #2a2a2a;
  border-radius: 2px;
  overflow: hidden;
}

.progressBar {
  height: 100%;
  background: var(--color-accent-amber);
  border-radius: 2px;
}

.progressLabel {
  font-size: 10px;
  color: var(--color-text-secondary);
  margin-top: var(--space-xs);
}

.seasonTabs {
  padding: 0 var(--space-lg) 10px;
  display: flex;
  gap: var(--space-sm);
  overflow-x: auto;
}

.episodeList {
  padding: 0 var(--space-lg);
  display: flex;
  flex-direction: column;
  gap: 6px;
}
```

- [ ] **Step 2: Rewrite TitleDetail.tsx**

Replace the full component. Key changes:
- Remove ActionBar import, add ActionDrawer
- Remove hero text overlay buttons (edit, rematch, anilist)
- Add identity zone with mini poster
- Add content cards (ratings, synopsis, cast, details)
- Add synopsis expand/collapse state
- Parse `genres` and `credits` JSON strings
- Keep all existing bottom sheets and series-specific sections

```tsx
import { useState } from 'preact/hooks'
import type { Title } from '../types'
import { useApi } from '../hooks/useApi'
import { getName, getTypeLabel, getStatusLabel } from '../utils'
import { apiFetch } from '../api'
import { SeasonTab } from '../components/SeasonTab'
import { EpisodeRow } from '../components/EpisodeRow'
import { ActionDrawer } from '../components/ActionDrawer'
import { RatingPrompt } from '../components/RatingPrompt'
import { EditSheet } from '../components/EditSheet'
import { RematchSheet } from '../components/RematchSheet'
import { AniListSheet } from '../components/AniListSheet'
import { ErrorBanner } from '../components/ErrorBanner'
import { CoverPlaceholder, coverBackground } from '../components/CoverPlaceholder'
import s from './TitleDetail.module.css'

function getNextUnwatched(title: Title) {
  for (const season of [...(title.seasons ?? [])].sort((a, b) => a.season_number - b.season_number)) {
    for (const ep of [...(season.episodes ?? [])].sort((a, b) => a.episode - b.episode)) {
      if (!ep.watched) return { season, episode: ep }
    }
  }
  return null
}

function formatSeriesStatus(st: string | null) {
  if (!st) return ''
  return st.charAt(0).toUpperCase() + st.slice(1).replace('_', ' ')
}

function formatRuntime(minutes: number): string {
  const h = Math.floor(minutes / 60)
  const m = minutes % 60
  return h > 0 ? `${h}h ${m.toString().padStart(2, '0')}m` : `${m}m`
}

function parseJSON<T>(json: string | null): T | null {
  if (!json) return null
  try { return JSON.parse(json) } catch { return null }
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric' })
}

export function TitleDetail({ id }: { id?: string; path?: string }) {
  const { data: title, loading, error, mutate } = useApi<Title>(id ? `/titles/${id}` : null)
  const [activeSeason, setActiveSeason] = useState<number | null>(null)
  const [showRating, setShowRating] = useState(false)
  const [showEdit, setShowEdit] = useState(false)
  const [showAniList, setShowAniList] = useState(false)
  const [showRematch, setShowRematch] = useState(false)
  const [synopsisExpanded, setSynopsisExpanded] = useState(false)

  if (loading || !title) {
    return (
      <div className={s.loading}>
        {error ? <ErrorBanner message={error} onRetry={mutate} /> : loading ? 'Loading...' : 'Title not found'}
      </div>
    )
  }

  const name = getName(title)
  const typeLabel = getTypeLabel(title.type)
  const sortedSeasons = [...(title.seasons ?? [])].sort((a, b) => a.season_number - b.season_number)
  const current = sortedSeasons.find((ss) => ss.season_number === activeSeason)
    ?? sortedSeasons.find((ss) => (ss.episodes ?? []).some((e) => !e.watched))
    ?? sortedSeasons[sortedSeasons.length - 1]

  const currentEps = current?.episodes ?? []
  const watched = currentEps.filter((e) => e.watched).length
  const total = current?.total_episodes ?? currentEps.length
  const pct = total > 0 ? (watched / total) * 100 : 0
  const next = getNextUnwatched(title)

  const genres = parseJSON<string[]>(title.genres)
  const credits = parseJSON<{ name: string; role: string }[]>(title.credits)

  const handleMarkNext = async () => {
    if (!next) return
    await apiFetch(`/titles/${title.id}/episodes/${next.episode.id}`, { method: 'PATCH' })
    mutate()
  }

  const handleSaveRating = async (rating: number) => {
    await apiFetch(`/titles/${title.id}`, {
      method: 'PATCH',
      body: JSON.stringify({ my_rating: rating }),
    })
    setShowRating(false)
    mutate()
  }

  const handleSaveEdit = async (updates: { type?: string; status?: string }) => {
    if (Object.keys(updates).length > 0) {
      await apiFetch(`/titles/${title.id}`, {
        method: 'PATCH',
        body: JSON.stringify(updates),
      })
      mutate()
    }
    setShowEdit(false)
  }

  const handleConfirmAniList = async () => {
    await apiFetch(`/titles/${title.id}`, {
      method: 'PATCH',
      body: JSON.stringify({ match_status: 'confirmed' }),
    })
    setShowAniList(false)
    mutate()
  }

  // Build meta line
  const metaParts = [typeLabel, String(title.year)]
  if (title.runtime) metaParts.push(formatRuntime(title.runtime))
  if (title.series_status) metaParts.push(formatSeriesStatus(title.series_status))

  return (
    <div className={s.page}>
      {/* Hero — pure visual */}
      <div
        className={s.hero}
        style={{ background: coverBackground(title.cover_url, title.type) }}
      >
        {!title.cover_url && <CoverPlaceholder type={title.type} iconSize="48px" />}
        <div className={s.heroFade} />
        <button onClick={() => history.back()} aria-label="Back" className={s.backBtn}>
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="#fff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <line x1="19" y1="12" x2="5" y2="12" />
            <polyline points="12 19 5 12 12 5" />
          </svg>
        </button>
      </div>

      {/* Identity zone */}
      <div className={s.identity}>
        {title.cover_url ? (
          <img src={`/api/covers/${title.cover_url}`} alt="" className={s.miniPoster} />
        ) : (
          <div className={s.miniPosterPlaceholder} style={{ background: coverBackground(null, title.type) }}>
            <CoverPlaceholder type={title.type} iconSize="24px" />
          </div>
        )}
        <div className={s.identityInfo}>
          <div className={s.identityTitle}>{name}</div>
          <div className={s.identityMeta}>{metaParts.join(' · ')}</div>
          {genres && genres.length > 0 && (
            <div className={s.genrePills}>
              {genres.map((g) => <span key={g} className={s.genrePill}>{g}</span>)}
            </div>
          )}
        </div>
      </div>

      {/* Ratings card */}
      <div className={s.card} style={{ marginTop: '12px' }}>
        <div className={s.ratingsRow}>
          <div>
            <div className={s.cardLabel}>My rating</div>
            {title.my_rating != null ? (
              <div className={s.myRating}>{title.my_rating}<span className={s.myRatingSuffix}>/10</span></div>
            ) : (
              <div className={s.noRating}>Not rated</div>
            )}
          </div>
          <div className={s.extRatings}>
            {title.tmdb_rating != null && (
              <div className={s.extItem}>
                <div className={`${s.extScore} ${s.tmdbColor}`}>{title.tmdb_rating.toFixed(1)}</div>
                <div className={s.extSource}>TMDB</div>
              </div>
            )}
            {title.anilist_rating != null && (
              <div className={s.extItem}>
                <div className={`${s.extScore} ${s.anilistColor}`}>{title.anilist_rating}%</div>
                <div className={s.extSource}>AniList</div>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Synopsis card */}
      {title.overview && (
        <div className={s.card}>
          <div className={s.cardLabel}>Synopsis</div>
          <div className={`${s.synopsisText} ${!synopsisExpanded ? s.synopsisClamped : ''}`}>
            {title.overview}
          </div>
          <button className={s.synopsisToggle} onClick={() => setSynopsisExpanded(!synopsisExpanded)}>
            {synopsisExpanded ? 'Show less' : 'Show more'}
          </button>
        </div>
      )}

      {/* Cast & Crew card */}
      {credits && credits.length > 0 && (
        <div className={s.card}>
          <div className={s.cardLabel}>Cast & Crew</div>
          <div className={s.castList}>
            {credits.map((c, i) => (
              <div key={i} className={s.castEntry}>
                <span className={s.castPerson}>{c.name}</span>
                <span className={s.castRole}>{c.role}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Details card */}
      <div className={s.card}>
        <div className={s.cardLabel}>Details</div>
        <div className={s.detailRow}>
          <span className={s.detailKey}>Status</span>
          <span className={s.detailVal}>{getStatusLabel(title.status)}</span>
        </div>
        <div className={s.detailRow}>
          <span className={s.detailKey}>Added</span>
          <span className={s.detailVal}>{formatDate(title.created_at as any)}</span>
        </div>
        {title.match_source && (
          <div className={s.detailRow}>
            <span className={s.detailKey}>Match</span>
            <span className={s.detailVal}>{title.match_source}</span>
          </div>
        )}
        {title.original_title && title.original_title !== name && (
          <div className={s.detailRow}>
            <span className={s.detailKey}>Original title</span>
            <span className={s.detailVal}>{title.original_title}</span>
          </div>
        )}
      </div>

      {/* Progress bar (series/anime) */}
      {current && title.type !== 'movie' && (
        <div className={s.progressWrap}>
          <div className={s.progressTrack}>
            <div className={s.progressBar} style={{ width: `${pct}%` }} />
          </div>
          <div className={s.progressLabel}>
            S{current.season_number} · {watched} of {total} episodes watched
          </div>
        </div>
      )}

      {/* Season tabs */}
      {sortedSeasons.length > 1 && (
        <div className={s.seasonTabs}>
          {sortedSeasons.map((ss) => (
            <SeasonTab
              key={ss.id}
              season={ss}
              active={ss.id === current?.id}
              onClick={() => setActiveSeason(ss.season_number)}
            />
          ))}
        </div>
      )}

      {/* Episode list */}
      {current && (
        <div className={s.episodeList}>
          {[...(current.episodes ?? [])]
            .sort((a, b) => a.episode - b.episode)
            .map((ep) => (
              <EpisodeRow key={ep.id} titleId={title.id} episode={ep} onToggle={mutate} />
            ))}
        </div>
      )}

      {/* Action drawer */}
      <ActionDrawer
        title={title}
        nextEpisode={next?.episode ?? null}
        nextSeasonNumber={next?.season.season_number}
        onMarkNext={handleMarkNext}
        onRate={() => setShowRating(true)}
        onEdit={() => setShowEdit(true)}
        onRematch={() => setShowRematch(true)}
        onAniList={() => setShowAniList(true)}
      />

      {/* Bottom sheets */}
      <RatingPrompt
        open={showRating}
        onClose={() => setShowRating(false)}
        titleName={name}
        initialRating={title.my_rating}
        hasImdb={!!title.imdb_id}
        hasAnilist={title.type === 'anime'}
        onSave={handleSaveRating}
        onSaveAndImdb={(rating) => {
          handleSaveRating(rating)
          if (title.imdb_id) window.open(`https://www.imdb.com/title/${title.imdb_id}/`, '_blank', 'noopener,noreferrer')
        }}
      />

      <EditSheet
        open={showEdit}
        onClose={() => setShowEdit(false)}
        title={title}
        onSave={handleSaveEdit}
      />

      {title.type === 'anime' && (
        <AniListSheet
          open={showAniList}
          onClose={() => setShowAniList(false)}
          title={title}
          onConfirm={handleConfirmAniList}
        />
      )}

      <RematchSheet
        open={showRematch}
        onClose={() => setShowRematch(false)}
        title={title}
        onDone={mutate}
      />
    </div>
  )
}
```

Note: The `created_at` field needs to be added to the Title TS type — it's already returned by the API but not in the TS interface. Add `created_at: string` to the Title interface in Task 8.

- [ ] **Step 3: Run tests**

Run: `make test-front`
Expected: PASS

- [ ] **Step 4: Visual verification via Chrome DevTools MCP**

1. Login with DEBUG_LOGIN credentials from `.env.local`
2. Navigate to a movie detail page
3. Verify: clean hero, identity zone with poster, ratings card, synopsis card, cast card, details card, action drawer
4. Open/close the action drawer
5. Check a series/anime detail page for progress bar + episodes
6. Check console for errors

- [ ] **Step 5: Commit**

```
feat(frontend): refonte complète de la page détail titre avec métadonnées TMDB
```

---

## Task 11: Cleanup — Delete ActionBar

**Files:**
- Delete: `frontend/src/components/ActionBar.tsx`
- Delete: `frontend/src/components/ActionBar.module.css`

- [ ] **Step 1: Verify no remaining imports**

Run grep for ActionBar imports. Should find none after Task 10.

- [ ] **Step 2: Delete the files**

```bash
rm frontend/src/components/ActionBar.tsx frontend/src/components/ActionBar.module.css
```

- [ ] **Step 3: Run tests + lint**

Run: `make test-front && make lint`
Expected: PASS

- [ ] **Step 4: Commit**

```
chore(frontend): supprime l'ancien composant ActionBar remplacé par ActionDrawer
```

---

## Task 12: Update patterns.md and docs

**Files:**
- Modify: `docs/patterns.md`

- [ ] **Step 1: Update component table**

Replace `ActionBar` entry with `ActionDrawer`. Add description of new content cards pattern.

- [ ] **Step 2: Move spec to done**

```bash
mv docs/superpowers/specs/2026-04-05-title-detail-redesign.md docs/superpowers/specs/done/
```

- [ ] **Step 3: Commit**

```
docs: met à jour patterns.md après la refonte de la page détail
```
