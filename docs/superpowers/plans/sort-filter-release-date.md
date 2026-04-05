# Sort & Filter by Release Date — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `release_date` column to titles, populate it via TMDB enrichment, expose it as the default sort and as a filter (decade dropdown + date range + include-null toggle) in the frontend.

**Architecture:** New DB column `release_date TEXT` (YYYY-MM-DD format). Enrichment pipeline already fetches this from TMDB — just persist it. Backend: new filter fields on `TitleFilter`, new sort option. Frontend: new `SortField`, new filter section in `FilterDrawer` with decade dropdown, date range inputs, and toggle.

**Tech Stack:** Go / SQLite / chi (backend), Preact / TypeScript / Zustand (frontend)

---

### Task 1: Migration — Add `release_date` column

**Files:**
- Create: `internal/database/migrations/007_release_date.up.sql`
- Create: `internal/database/migrations/007_release_date.down.sql`

- [ ] **Step 1: Create up migration**

```sql
ALTER TABLE titles ADD COLUMN release_date TEXT;
```

- [ ] **Step 2: Create down migration**

```sql
ALTER TABLE titles DROP COLUMN release_date;
```

- [ ] **Step 3: Run migration**

Run: `make migrate`

- [ ] **Step 4: Commit**

```
feat(db): ajoute la colonne release_date sur titles
```

---

### Task 2: Model & Repository — Wire `release_date` through Go code

**Files:**
- Modify: `internal/model/title.go:60` — add field after `AniListRating`
- Modify: `internal/repository/title.go` — TitleFilter, TitleUpdate, Create, GetByID, List, ListAll, Update

- [ ] **Step 1: Add `ReleaseDate` to model**

In `internal/model/title.go`, add after `AniListRating *int` (line 60):

```go
	ReleaseDate   *string       `json:"release_date"`
```

- [ ] **Step 2: Add filter fields to `TitleFilter`**

In `internal/repository/title.go`, add to the `TitleFilter` struct (after line 32):

```go
	Decade         *int    // e.g. 2020 → year BETWEEN 2020 AND 2029
	ReleaseFrom    *string // YYYY-MM-DD, filters on release_date >=
	ReleaseTo      *string // YYYY-MM-DD, filters on release_date <=
	IncludeNoRelease bool  // when false + date filter active, exclude NULL release_date
```

- [ ] **Step 3: Add `ReleaseDate` to `TitleUpdate`**

In `internal/repository/title.go`, add after `AniListRating *int` (line 70):

```go
	ReleaseDate   *string
```

- [ ] **Step 4: Add `release_date` to `Create`**

In `createInTx` (line 88–96), update the INSERT:

```go
	res, err := db.Exec(`
		INSERT INTO titles (type, year, cover_url, imdb_id, anilist_id, tmdb_id, tvdb_id, plex_rating_key, my_rating, status, series_status, match_status, original_title, match_source, overview, genres, runtime, tmdb_rating, credits, anilist_rating, release_date)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		title.Type, title.Year, title.CoverURL, title.IMDBID, title.AniListID, title.TMDBID, title.TVDBID,
		title.PlexRatingKey, title.MyRating, title.Status, title.SeriesStatus, title.MatchStatus,
		title.OriginalTitle, title.MatchSource,
		title.Overview, title.Genres, title.Runtime, title.TMDBRating, title.Credits, title.AniListRating,
		title.ReleaseDate,
	)
```

- [ ] **Step 5: Add `release_date` to `GetByID`**

In `GetByID` (line 114–120), update SELECT and Scan:

```go
	err := r.db.QueryRow(`SELECT id, type, year, cover_url, imdb_id, anilist_id, tmdb_id, tvdb_id, plex_rating_key, my_rating, status, series_status, match_status, original_title, match_source, overview, genres, runtime, tmdb_rating, credits, anilist_rating, release_date, created_at, updated_at FROM titles WHERE id = ?`, id).
		Scan(&title.ID, &title.Type, &title.Year, &title.CoverURL, &title.IMDBID, &title.AniListID, &title.TMDBID, &title.TVDBID,
			&title.PlexRatingKey, &title.MyRating, &title.Status, &title.SeriesStatus, &title.MatchStatus, &title.OriginalTitle, &title.MatchSource,
			&title.Overview, &title.Genres, &title.Runtime, &title.TMDBRating, &title.Credits, &title.AniListRating,
			&title.ReleaseDate, &title.CreatedAt, &title.UpdatedAt)
```

- [ ] **Step 6: Add `release_date` to `List`**

In `List` (line 189), update `baseCols`:

```go
	baseCols := `t.id, t.type, t.year, t.cover_url, t.imdb_id, t.anilist_id, t.tmdb_id, t.tvdb_id, t.plex_rating_key, t.my_rating, t.status, t.series_status, t.match_status, t.original_title, t.match_source, t.overview, t.genres, t.runtime, t.tmdb_rating, t.credits, t.anilist_rating, t.release_date, t.created_at, t.updated_at`
```

Add filter conditions after the `SeriesStatus` block (after line 222):

```go
	if filter.Decade != nil {
		conditions = append(conditions, `t.year BETWEEN ? AND ?`)
		args = append(args, *filter.Decade, *filter.Decade+9)
	}
	if filter.ReleaseFrom != nil {
		conditions = append(conditions, `t.release_date >= ?`)
		args = append(args, *filter.ReleaseFrom)
		if !filter.IncludeNoRelease {
			conditions = append(conditions, `t.release_date IS NOT NULL`)
		}
	}
	if filter.ReleaseTo != nil {
		conditions = append(conditions, `t.release_date <= ?`)
		args = append(args, *filter.ReleaseTo)
		if !filter.IncludeNoRelease && filter.ReleaseFrom == nil {
			conditions = append(conditions, `t.release_date IS NOT NULL`)
		}
	}
```

Add `"release_date"` to the NULLS LAST switch (line 255):

```go
	case "my_rating", "year", "original_title", "release_date":
```

Update the Scan in the rows loop (lines 275–278):

```go
	if err := rows.Scan(&t.ID, &t.Type, &t.Year, &t.CoverURL, &t.IMDBID, &t.AniListID, &t.TMDBID, &t.TVDBID,
		&t.PlexRatingKey, &t.MyRating, &t.Status, &t.SeriesStatus, &t.MatchStatus, &t.OriginalTitle, &t.MatchSource,
		&t.Overview, &t.Genres, &t.Runtime, &t.TMDBRating, &t.Credits, &t.AniListRating,
		&t.ReleaseDate, &t.CreatedAt, &t.UpdatedAt); err != nil {
```

- [ ] **Step 7: Add `release_date` to `ListAll`**

In `ListAll` (lines 365–376), update SELECT and Scan:

```go
	rows, err := r.db.Query(`SELECT id, type, year, cover_url, imdb_id, anilist_id, tmdb_id, tvdb_id, plex_rating_key, my_rating, status, series_status, match_status, original_title, match_source, overview, genres, runtime, tmdb_rating, credits, anilist_rating, release_date, created_at, updated_at FROM titles ORDER BY updated_at DESC`)
```

```go
	if err := rows.Scan(&t.ID, &t.Type, &t.Year, &t.CoverURL, &t.IMDBID, &t.AniListID, &t.TMDBID, &t.TVDBID,
		&t.PlexRatingKey, &t.MyRating, &t.Status, &t.SeriesStatus, &t.MatchStatus, &t.OriginalTitle, &t.MatchSource,
		&t.Overview, &t.Genres, &t.Runtime, &t.TMDBRating, &t.Credits, &t.AniListRating,
		&t.ReleaseDate, &t.CreatedAt, &t.UpdatedAt); err != nil {
```

- [ ] **Step 8: Add `release_date` to `Update`**

After the `AniListRating` block (line 527), add:

```go
	if update.ReleaseDate != nil {
		sets = append(sets, `release_date = ?`)
		args = append(args, *update.ReleaseDate)
	}
```

- [ ] **Step 9: Run tests**

Run: `make test`
Expected: all existing tests pass (new column is nullable, no existing data breaks)

- [ ] **Step 10: Commit**

```
feat(model): ajoute release_date au modèle, repository et filtres
```

---

### Task 3: Tests — Sort and filter by release_date

**Files:**
- Modify: `internal/repository/title_test.go`

- [ ] **Step 1: Write sort test**

Add after `TestTitleRepository_List_SortByRating_NullsLast`:

```go
func TestTitleRepository_List_SortByReleaseDate(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	rd1 := "2020-03-15"
	rd2 := "2024-11-01"
	_, _ = repo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2020, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed, ReleaseDate: &rd1}, []model.TitleName{{Name: "Old Movie", Language: "en", IsPrimary: true}})
	_, _ = repo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed, ReleaseDate: &rd2}, []model.TitleName{{Name: "New Movie", Language: "en", IsPrimary: true}})

	result, err := repo.List(repository.TitleFilter{
		Status: ptr(model.TitleStatusCompleted),
		Sort:   "release_date",
		Order:  "asc",
	})
	require.NoError(t, err)
	require.Len(t, result.Titles, 2)
	assert.Equal(t, "Old Movie", result.Titles[0].PrimaryName())
	assert.Equal(t, "New Movie", result.Titles[1].PrimaryName())
}
```

- [ ] **Step 2: Write NULLS LAST test**

```go
func TestTitleRepository_List_SortByReleaseDate_NullsLast(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	rd := "2024-06-01"
	_, _ = repo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed, ReleaseDate: &rd}, []model.TitleName{{Name: "With Date", Language: "en", IsPrimary: true}})
	_, _ = repo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2023, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "No Date", Language: "en", IsPrimary: true}})

	result, err := repo.List(repository.TitleFilter{
		Status: ptr(model.TitleStatusCompleted),
		Sort:   "release_date",
		Order:  "desc",
	})
	require.NoError(t, err)
	require.Len(t, result.Titles, 2)
	assert.Equal(t, "With Date", result.Titles[0].PrimaryName())
	assert.Equal(t, "No Date", result.Titles[1].PrimaryName())
}
```

- [ ] **Step 3: Write decade filter test**

```go
func TestTitleRepository_List_FilterByDecade(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	_, _ = repo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2015, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "2010s Movie", Language: "en", IsPrimary: true}})
	_, _ = repo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2022, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "2020s Movie", Language: "en", IsPrimary: true}})

	decade := 2020
	result, err := repo.List(repository.TitleFilter{
		Status: ptr(model.TitleStatusCompleted),
		Decade: &decade,
	})
	require.NoError(t, err)
	require.Len(t, result.Titles, 1)
	assert.Equal(t, "2020s Movie", result.Titles[0].PrimaryName())
}
```

- [ ] **Step 4: Write date range filter test**

```go
func TestTitleRepository_List_FilterByDateRange(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	rd1 := "2023-01-15"
	rd2 := "2024-06-20"
	rd3 := "2025-01-01"
	_, _ = repo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2023, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed, ReleaseDate: &rd1}, []model.TitleName{{Name: "Early", Language: "en", IsPrimary: true}})
	_, _ = repo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed, ReleaseDate: &rd2}, []model.TitleName{{Name: "Mid", Language: "en", IsPrimary: true}})
	_, _ = repo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2025, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed, ReleaseDate: &rd3}, []model.TitleName{{Name: "Late", Language: "en", IsPrimary: true}})

	from := "2024-01-01"
	to := "2024-12-31"
	result, err := repo.List(repository.TitleFilter{
		Status:      ptr(model.TitleStatusCompleted),
		ReleaseFrom: &from,
		ReleaseTo:   &to,
	})
	require.NoError(t, err)
	require.Len(t, result.Titles, 1)
	assert.Equal(t, "Mid", result.Titles[0].PrimaryName())
}
```

- [ ] **Step 5: Write include-no-release toggle test**

```go
func TestTitleRepository_List_FilterByDateRange_ExcludeNoRelease(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	rd := "2024-06-20"
	_, _ = repo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed, ReleaseDate: &rd}, []model.TitleName{{Name: "With Date", Language: "en", IsPrimary: true}})
	_, _ = repo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "No Date", Language: "en", IsPrimary: true}})

	from := "2024-01-01"
	// IncludeNoRelease = true (default) → include NULL release_date
	result, err := repo.List(repository.TitleFilter{
		Status:           ptr(model.TitleStatusCompleted),
		ReleaseFrom:      &from,
		IncludeNoRelease: true,
	})
	require.NoError(t, err)
	require.Len(t, result.Titles, 2)

	// IncludeNoRelease = false → exclude NULL release_date
	result2, err := repo.List(repository.TitleFilter{
		Status:           ptr(model.TitleStatusCompleted),
		ReleaseFrom:      &from,
		IncludeNoRelease: false,
	})
	require.NoError(t, err)
	require.Len(t, result2.Titles, 1)
	assert.Equal(t, "With Date", result2.Titles[0].PrimaryName())
}
```

- [ ] **Step 6: Run tests**

Run: `make test`
Expected: all new + existing tests pass

- [ ] **Step 7: Commit**

```
test(repository): ajoute les tests de tri et filtre par release_date
```

---

### Task 4: Handler — Parse new filter params

**Files:**
- Modify: `internal/handler/title.go:26-70`
- Modify: `internal/handler/title_test.go`

- [ ] **Step 1: Add `release_date` to `allowedSorts`**

In `internal/handler/title.go` (line 26–32):

```go
var allowedSorts = map[string]bool{
	"updated_at":     true,
	"original_title": true,
	"year":           true,
	"my_rating":      true,
	"created_at":     true,
	"release_date":   true,
}
```

- [ ] **Step 2: Parse new filter params**

After the `series_status` block (line 67–70), add:

```go
	if d := r.URL.Query().Get("decade"); d != "" {
		if decade := httputil.ParseQueryInt(r, "decade", 0); decade >= 1900 && decade <= 2100 {
			filter.Decade = &decade
		}
	}
	if rf := r.URL.Query().Get("release_from"); rf != "" {
		filter.ReleaseFrom = &rf
	}
	if rt := r.URL.Query().Get("release_to"); rt != "" {
		filter.ReleaseTo = &rt
	}
	filter.IncludeNoRelease = true // default: include titles without release date
	if r.URL.Query().Get("include_no_release") == "false" {
		filter.IncludeNoRelease = false
	}
```

- [ ] **Step 3: Add handler test for release_date sort**

In `internal/handler/title_test.go`, add after `TestTitleHandler_List_WithSort`:

```go
func TestTitleHandler_List_WithReleaseDateSort(t *testing.T) {
	h, _ := setupTitleHandler(t)

	req := httptest.NewRequest("GET", "/api/titles?status=completed&sort=release_date&order=desc", nil)
	rr := httptest.NewRecorder()
	require.NoError(t, h.List(rr, req))
	assert.Equal(t, 200, rr.Code)
}
```

- [ ] **Step 4: Run tests**

Run: `make test`
Expected: PASS

- [ ] **Step 5: Commit**

```
feat(handler): ajoute le tri et filtre par release_date dans l'API
```

---

### Task 5: Enrichment pipeline — Persist `release_date` from TMDB

**Files:**
- Modify: `internal/service/matching/pipeline.go:51-68` — add `ReleaseDate` to `MatchResult`
- Modify: `internal/service/matching/pipeline.go:348-380` — set it in `fetchTMDBDetailsAndCover`
- Modify: `internal/service/taskqueue.go:218-220` — map to `TitleUpdate`

- [ ] **Step 1: Add `ReleaseDate` to `MatchResult`**

In `internal/service/matching/pipeline.go`, add after `AniListRating *int` (line 67):

```go
	ReleaseDate   string
```

- [ ] **Step 2: Populate in `fetchTMDBDetailsAndCover`**

In the movie branch (after line 360, before poster download):

```go
		result.ReleaseDate = details.ReleaseDate
```

In the TV branch (after line 375, before poster download):

```go
		result.ReleaseDate = details.FirstAirDate
```

- [ ] **Step 3: Map in `handleEnrichment`**

In `internal/service/taskqueue.go`, after the `AniListRating` block (line 218–220):

```go
	if result.ReleaseDate != "" {
		update.ReleaseDate = &result.ReleaseDate
	}
```

- [ ] **Step 4: Run tests**

Run: `make test`
Expected: PASS

- [ ] **Step 5: Commit**

```
feat(enrichment): persiste la release_date depuis TMDB
```

---

### Task 6: Frontend — Sort by release date

**Files:**
- Modify: `frontend/src/types.ts:26` — add `release_date` to `Title`
- Modify: `frontend/src/store.ts:9,17` — add to `SortField`, change default
- Modify: `frontend/src/components/FilterDrawer.tsx:43-49` — replace Year chip

- [ ] **Step 1: Add `release_date` to `Title` type**

In `frontend/src/types.ts`, add after `anilist_rating`:

```ts
  release_date: string | null
```

- [ ] **Step 2: Update `SortField` and default sort**

In `frontend/src/store.ts`, line 9:

```ts
export type SortField = 'updated_at' | 'original_title' | 'release_date' | 'my_rating' | 'created_at'
```

Line 17:

```ts
const DEFAULT_SORT: SortState = { field: 'release_date', order: 'desc' }
```

- [ ] **Step 3: Replace Year sort chip with Release date**

In `frontend/src/components/FilterDrawer.tsx`, replace the `sortOptions` array (lines 43–49):

```ts
const sortOptions: { field: SortField; label: string; defaultOrder: SortOrder }[] = [
  { field: 'updated_at', label: 'Last updated', defaultOrder: 'desc' },
  { field: 'original_title', label: 'Title', defaultOrder: 'asc' },
  { field: 'release_date', label: 'Release date', defaultOrder: 'desc' },
  { field: 'my_rating', label: 'Rating', defaultOrder: 'desc' },
  { field: 'created_at', label: 'Date added', defaultOrder: 'desc' },
]
```

- [ ] **Step 4: Update collapsed tag logic**

In `FilterDrawer.tsx` line 109, update the default sort check:

```ts
  if (!isSearchActive && activeSort && sort.field !== 'release_date') {
```

- [ ] **Step 5: Run frontend tests**

Run: `make test-front`
Expected: PASS

- [ ] **Step 6: Commit**

```
feat(front): remplace le tri Year par Release date comme tri par défaut
```

---

### Task 7: Frontend — Release date filter section

**Files:**
- Modify: `frontend/src/store.ts` — add filter fields + query params
- Modify: `frontend/src/components/FilterDrawer.tsx` — add filter section
- Modify: `frontend/src/components/FilterDrawer.module.css` — styles for new controls
- Modify: `frontend/src/app.tsx` — wire new filter props

- [ ] **Step 1: Add filter fields to store**

In `frontend/src/store.ts`, extend the `filter` type (lines 43–49):

```ts
  filter: {
    status?: string
    type?: string
    search?: string
    match_status?: string
    series_status?: string
    decade?: string
    release_from?: string
    release_to?: string
    include_no_release?: string
  }
```

In `fetchTitles` params builder (after line 88), add:

```ts
      if (f.decade) params.set('decade', f.decade)
      if (f.release_from) params.set('release_from', f.release_from)
      if (f.release_to) params.set('release_to', f.release_to)
      if (f.include_no_release) params.set('include_no_release', f.include_no_release)
```

Same for `loadMore` params builder (after line 119).

- [ ] **Step 2: Add release date filter props to FilterDrawer**

In `frontend/src/components/FilterDrawer.tsx`, extend `FilterDrawerProps`:

```ts
interface FilterDrawerProps {
  status: StatusFilter
  type: TypeFilter
  seriesStatus: SeriesStatusFilter
  onStatusChange: (status: StatusFilter) => void
  onTypeChange: (type: TypeFilter) => void
  onSeriesStatusChange: (seriesStatus: SeriesStatusFilter) => void
  sort: SortState
  onSortChange: (sort: SortState) => void
  isSearchActive: boolean
  defaultOpen?: boolean
  decade: string | null
  releaseFrom: string
  releaseTo: string
  includeNoRelease: boolean
  onDecadeChange: (decade: string | null) => void
  onReleaseFromChange: (date: string) => void
  onReleaseToChange: (date: string) => void
  onIncludeNoReleaseChange: (include: boolean) => void
}
```

- [ ] **Step 3: Add decade options and filter section JSX**

Add decade options constant:

```ts
const decadeOptions = [
  { value: '', label: 'All' },
  { value: '2000', label: '2000s' },
  { value: '2010', label: '2010s' },
  { value: '2020', label: '2020s' },
]
```

Destructure new props in the component function. Add the filter section JSX after the series status block (before `<div className={s.bottomPad} />`):

```tsx
        <div className={s.filterLabel}>Release date</div>
        <div className={s.filterRow}>
          <select
            className={s.select}
            value={decade ?? ''}
            onChange={(e) => {
              const val = (e.target as HTMLSelectElement).value
              onDecadeChange(val || null)
              if (val) {
                onReleaseFromChange('')
                onReleaseToChange('')
              }
            }}
          >
            {decadeOptions.map((opt) => (
              <option key={opt.value} value={opt.value}>{opt.label}</option>
            ))}
          </select>
          <input
            type="date"
            className={s.dateInput}
            value={releaseFrom}
            placeholder="From"
            onChange={(e) => {
              onReleaseFromChange((e.target as HTMLInputElement).value)
              if ((e.target as HTMLInputElement).value) onDecadeChange(null)
            }}
          />
          <input
            type="date"
            className={s.dateInput}
            value={releaseTo}
            placeholder="To"
            onChange={(e) => {
              onReleaseToChange((e.target as HTMLInputElement).value)
              if ((e.target as HTMLInputElement).value) onDecadeChange(null)
            }}
          />
        </div>
        {(decade || releaseFrom || releaseTo) && (
          <div className={s.filterRow}>
            <label className={s.toggleLabel}>
              <input
                type="checkbox"
                checked={includeNoRelease}
                onChange={(e) => onIncludeNoReleaseChange((e.target as HTMLInputElement).checked)}
              />
              <span>Include without release date</span>
            </label>
          </div>
        )}
```

- [ ] **Step 4: Update active tags for collapsed state**

Add before the return, after the existing `activeTags` logic:

```ts
  if (decade) {
    const decadeLabel = decadeOptions.find((o) => o.value === decade)?.label ?? decade
    activeTags.push({ label: decadeLabel, color: colors.accentTeal })
  } else if (releaseFrom || releaseTo) {
    const tag = releaseFrom && releaseTo
      ? `${releaseFrom.slice(0, 7)} → ${releaseTo.slice(0, 7)}`
      : releaseFrom ? `≥ ${releaseFrom}` : `≤ ${releaseTo}`
    activeTags.push({ label: tag, color: colors.accentTeal })
  }
```

- [ ] **Step 5: Add CSS styles**

In `frontend/src/components/FilterDrawer.module.css`, add:

```css
.select {
  font-size: 9px;
  padding: 3px 6px;
  border-radius: 10px;
  border: none;
  background: var(--color-bg-surface);
  color: var(--color-text-muted);
  font-family: inherit;
  cursor: pointer;
  -webkit-appearance: none;
  appearance: none;
}

.dateInput {
  font-size: 9px;
  padding: 3px 6px;
  border-radius: 10px;
  border: none;
  background: var(--color-bg-surface);
  color: var(--color-text-muted);
  font-family: inherit;
  min-width: 0;
  flex: 1;
}

.dateInput::-webkit-calendar-picker-indicator {
  filter: invert(0.7);
  cursor: pointer;
}

.toggleLabel {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 9px;
  color: var(--color-text-muted);
  cursor: pointer;
}

.toggleLabel input[type="checkbox"] {
  width: 12px;
  height: 12px;
  accent-color: var(--color-accent-teal);
}
```

Update `drawerExpanded` max-height (line 62):

```css
.drawerExpanded {
  max-height: 400px;
  opacity: 1;
}
```

- [ ] **Step 6: Wire filter props in `app.tsx`**

In `frontend/src/app.tsx`, add handlers and pass props:

```ts
  const handleDecadeChange = useCallback((d: string | null) => {
    setFilter({ decade: d ?? undefined, release_from: undefined, release_to: undefined })
  }, [setFilter])

  const handleReleaseFromChange = useCallback((d: string) => {
    setFilter({ release_from: d || undefined, decade: undefined })
  }, [setFilter])

  const handleReleaseToChange = useCallback((d: string) => {
    setFilter({ release_to: d || undefined, decade: undefined })
  }, [setFilter])

  const handleIncludeNoReleaseChange = useCallback((include: boolean) => {
    setFilter({ include_no_release: include ? undefined : 'false' })
  }, [setFilter])
```

Add to `<FilterDrawer>` props:

```tsx
              <FilterDrawer
                status={statusFilter}
                type={typeFilter}
                seriesStatus={seriesStatusFilter}
                onStatusChange={handleStatusChange}
                onTypeChange={handleTypeChange}
                onSeriesStatusChange={handleSeriesStatusChange}
                sort={sort}
                onSortChange={setSort}
                isSearchActive={currentPath === '/search'}
                defaultOpen={currentPath === '/'}
                decade={filter.decade ?? null}
                releaseFrom={filter.release_from ?? ''}
                releaseTo={filter.release_to ?? ''}
                includeNoRelease={filter.include_no_release !== 'false'}
                onDecadeChange={handleDecadeChange}
                onReleaseFromChange={handleReleaseFromChange}
                onReleaseToChange={handleReleaseToChange}
                onIncludeNoReleaseChange={handleIncludeNoReleaseChange}
              />
```

- [ ] **Step 7: Run frontend build + tests**

Run: `make build && make test-front`
Expected: PASS

- [ ] **Step 8: Commit**

```
feat(front): ajoute le filtre par date de sortie dans le tiroir
```

---

### Task 8: Visual verification

- [ ] **Step 1: Start dev servers**

Run: `make up` and `make dev-frontend`

- [ ] **Step 2: Verify sort**

Navigate to home page. Confirm default sort is "Release date ↓". Titles with release dates appear first, sorted newest-first. Titles without release date appear at the end.

- [ ] **Step 3: Verify filter drawer**

Open filter drawer. Confirm:
- "Release date" chip replaces "Year" in sort row
- New "Release date" section with decade dropdown, From/To date inputs
- Selecting "2020s" filters to 2020–2029 titles
- Setting a From date deselects the decade dropdown
- "Include without release date" toggle appears when a filter is active
- Collapsed drawer shows active filter as a tag

- [ ] **Step 4: Verify other pages still work**

Navigate to search, admin, title detail — no console errors, no regressions.

- [ ] **Step 5: Commit fixes if needed**

---

### Task 9: Documentation updates

**Files:**
- Modify: `docs/patterns.md` — add release_date filter/sort documentation
- Modify: `CHANGELOG.md` — add entry

- [ ] **Step 1: Update patterns.md**

Add `release_date` to the list of sort fields and document the new filter params (`decade`, `release_from`, `release_to`, `include_no_release`).

- [ ] **Step 2: Update CHANGELOG.md**

Add under the next unreleased version:

```markdown
- Tri par date de sortie (nouveau tri par défaut)
- Filtre par date de sortie : décennie, intervalle de dates, option d'inclusion des titres sans date
```

- [ ] **Step 3: Move spec to done**

Move `docs/superpowers/specs/sort-release-date.md` → `docs/superpowers/specs/done/sort-release-date.md`

- [ ] **Step 4: Commit**

```
docs: met à jour patterns, changelog et spec pour release_date
```
