# Search & Genre Filter — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Search library-first with an optional TMDB toggle, and add a searchable genre filter with AND/OR logic to FilterDrawer.

**Architecture:** New `title_genres(title_id, genre)` join table (migration 016) replacing the JSON `genres` column. New `GET /api/genres` endpoint for the genre checklist. `TitleFilter` in the repository gains `Genres []string` and `GenreOp string` fields. On the frontend, `Search.tsx` calls the existing library FTS endpoint by default; the TMDB toggle triggers an additional client-side call. `FilterDrawer.tsx` gains a genre section with a searchable checklist and AND/OR toggle.

**Tech Stack:** Go 1.24, SQLite (migration 016), chi, testify/assert, Preact 10 / TypeScript / CSS Modules

---

## File Map

| Action | File |
|---|---|
| Create | `internal/database/migrations/016_title_genres.up.sql` |
| Create | `internal/database/migrations/016_title_genres.down.sql` |
| Create | `internal/repository/genre_repository.go` |
| Create | `internal/repository/genre_repository_test.go` |
| Modify | `internal/repository/title_search.go` — add Genres + GenreOp to TitleFilter |
| Create | `internal/handler/genre.go` |
| Create | `internal/handler/genre_test.go` |
| Modify | `cmd/serve.go` — register GET /api/genres |
| Modify | `frontend/src/types.ts` — Genre types |
| Modify | `frontend/src/store.ts` — add Genres + GenreOp to TitleFilter |
| Modify | `frontend/src/components/FilterDrawer.tsx` — add genre section |
| Modify | `frontend/src/components/FilterDrawer.module.css` |
| Modify | `frontend/src/pages/Search.tsx` — library-first + TMDB toggle |
| Modify | `frontend/src/pages/Search.module.css` |
| Modify | `docs/patterns.md` |

---

### Task 1: Migration 016 — title_genres

**Files:**
- Create: `internal/database/migrations/016_title_genres.up.sql`
- Create: `internal/database/migrations/016_title_genres.down.sql`

- [ ] **Step 1: Create up migration**

```sql
CREATE TABLE title_genres (
  title_id INTEGER NOT NULL REFERENCES titles(id) ON DELETE CASCADE,
  genre    TEXT NOT NULL
);

CREATE INDEX idx_title_genres_genre    ON title_genres(genre);
CREATE INDEX idx_title_genres_title_id ON title_genres(title_id);

-- Populate from existing JSON genres column
INSERT INTO title_genres (title_id, genre)
SELECT id, value
FROM titles, json_each(titles.genres)
WHERE titles.genres IS NOT NULL AND titles.genres != '[]' AND titles.genres != '';

-- Drop the JSON column (requires SQLite >= 3.35)
ALTER TABLE titles DROP COLUMN genres;
```

- [ ] **Step 2: Create down migration**

```sql
-- Restore genres JSON column
ALTER TABLE titles ADD COLUMN genres TEXT;

UPDATE titles
SET genres = (
  SELECT json_group_array(genre)
  FROM title_genres
  WHERE title_id = titles.id
);

DROP TABLE IF EXISTS title_genres;
```

- [ ] **Step 3: Run migration**

```bash
make migrate
```

Expected: exits 0. Verify data with: `make shell` → `sqlite3 /data/plextracker.db "SELECT COUNT(*) FROM title_genres;"` — should match the total genre entries previously in JSON.

- [ ] **Step 4: Commit**

```bash
git add internal/database/migrations/016_title_genres.up.sql internal/database/migrations/016_title_genres.down.sql
git commit -m "$(cat <<'EOF'
chore(db): normalise les genres en table title_genres (migration 016)

Remplace la colonne JSON genres par une join table indexée.
Données migrées depuis json_each sur les lignes existantes.
EOF
)"
```

---

### Task 2: Genre Repository

**Files:**
- Create: `internal/repository/genre_repository.go`
- Create: `internal/repository/genre_repository_test.go`

- [ ] **Step 1: Write the failing test**

```go
package repository_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/repository"
)

func TestGenreRepository_ListWithCounts(t *testing.T) {
	db := database.OpenTestDB(t)
	titleRepo := repository.NewTitleRepository(db)
	genreRepo := repository.NewGenreRepository(db)

	// Insert two titles with genres via title_genres directly
	t1 := createTestTitle(t, db, "movie", 120)
	t2 := createTestTitle(t, db, "series", 45)
	db.Exec(`INSERT INTO title_genres (title_id, genre) VALUES (?, 'Drama'), (?, 'Thriller')`, t1.ID, t1.ID)
	db.Exec(`INSERT INTO title_genres (title_id, genre) VALUES (?, 'Drama')`, t2.ID)

	genres, err := genreRepo.ListWithCounts(context.Background())
	assert.NoError(t, err)
	assert.Len(t, genres, 2)
	assert.Equal(t, "Drama", genres[0].Genre)
	assert.Equal(t, 2, genres[0].Count)
	assert.Equal(t, "Thriller", genres[1].Genre)
	assert.Equal(t, 1, genres[1].Count)
}
```

- [ ] **Step 2: Run test — expect FAIL**

```bash
make test
```

- [ ] **Step 3: Implement `genre_repository.go`**

```go
package repository

import (
	"context"
	"fmt"

	"github.com/nicolasvasse/plextracker/internal/database"
)

type GenreCount struct {
	Genre string `json:"genre"`
	Count int    `json:"count"`
}

type GenreRepository struct {
	db database.DBTX
}

func NewGenreRepository(db database.DBTX) *GenreRepository {
	return &GenreRepository{db: db}
}

// ListWithCounts returns all genres in the library ordered by count descending.
func (r *GenreRepository) ListWithCounts(ctx context.Context) ([]GenreCount, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT genre, COUNT(*) AS count
		FROM title_genres
		GROUP BY genre
		ORDER BY count DESC, genre ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("genre: list with counts: %w", err)
	}
	defer rows.Close()

	var results []GenreCount
	for rows.Next() {
		var g GenreCount
		if err := rows.Scan(&g.Genre, &g.Count); err != nil {
			return nil, fmt.Errorf("genre: scan: %w", err)
		}
		results = append(results, g)
	}
	return results, rows.Err()
}
```

- [ ] **Step 4: Run test — expect PASS**

```bash
make test
```

- [ ] **Step 5: Commit**

```bash
git add internal/repository/genre_repository.go internal/repository/genre_repository_test.go
git commit -m "$(cat <<'EOF'
feat(repository): ajoute GenreRepository.ListWithCounts
EOF
)"
```

---

### Task 3: Genre API Endpoint

**Files:**
- Create: `internal/handler/genre.go`
- Create: `internal/handler/genre_test.go`
- Modify: `cmd/serve.go`

- [ ] **Step 1: Write the failing test**

```go
package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenreHandler_List(t *testing.T) {
	db := database.OpenTestDB(t)
	db.Exec(`INSERT INTO title_genres (title_id, genre) VALUES (1, 'Drama'), (1, 'Action'), (2, 'Drama')`)
	h := handler.NewGenreHandler(repository.NewGenreRepository(db))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/genres", nil)
	err := h.List(w, r)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)

	var genres []map[string]any
	json.NewDecoder(w.Body).Decode(&genres)
	assert.Len(t, genres, 2)
	assert.Equal(t, "Drama", genres[0]["genre"])
	assert.InDelta(t, 2, genres[0]["count"], 0)
}
```

- [ ] **Step 2: Run test — expect FAIL**

```bash
make test
```

- [ ] **Step 3: Implement `genre.go`**

```go
package handler

import (
	"fmt"
	"net/http"

	"github.com/nicolasvasse/plextracker/internal/handler/httputil"
	"github.com/nicolasvasse/plextracker/internal/repository"
)

type GenreHandler struct {
	genres *repository.GenreRepository
}

func NewGenreHandler(genres *repository.GenreRepository) *GenreHandler {
	return &GenreHandler{genres: genres}
}

func (h *GenreHandler) List(w http.ResponseWriter, r *http.Request) error {
	genres, err := h.genres.ListWithCounts(r.Context())
	if err != nil {
		return fmt.Errorf("genre: list: %w", err)
	}
	return httputil.WriteJSON(w, genres)
}
```

- [ ] **Step 4: Register route in `cmd/serve.go`**

```go
genreHandler := handler.NewGenreHandler(repository.NewGenreRepository(db))
r.With(authMiddleware).Get("/api/genres", httputil.WrapHandler(genreHandler.List))
```

- [ ] **Step 5: Run tests — expect PASS**

```bash
make test
```

- [ ] **Step 6: Commit**

```bash
git add internal/handler/genre.go internal/handler/genre_test.go cmd/serve.go
git commit -m "$(cat <<'EOF'
feat(handler): ajoute GET /api/genres pour alimenter le filtre de genres
EOF
)"
```

---

### Task 4: Genre Filtering in TitleRepository

**Files:**
- Modify: `internal/repository/title_search.go`

- [ ] **Step 1: Locate `TitleFilter` struct in `title_search.go`**

Find the struct (around line 298) that has `Sort`, `Order`, `Status`, `Type`, etc. fields.

- [ ] **Step 2: Add genre fields to `TitleFilter`**

```go
type TitleFilter struct {
	// ... existing fields ...
	Genres   []string // filter by these genres
	GenreOp  string   // "AND" | "OR", defaults to "OR"
}
```

- [ ] **Step 3: Write the failing test**

In `internal/repository/title_search_test.go` (create if missing):
```go
func TestTitleSearch_GenreFilterOR(t *testing.T) {
	db := database.OpenTestDB(t)
	repo := repository.NewTitleRepository(db)

	t1 := createTestTitle(t, db, "movie", 120)
	t2 := createTestTitle(t, db, "series", 45)
	t3 := createTestTitle(t, db, "movie", 90)
	db.Exec(`INSERT INTO title_genres VALUES (?, 'Drama'), (?, 'Action'), (?, 'Thriller')`, t1.ID, t2.ID, t3.ID)

	result, err := repo.List(context.Background(), repository.TitleFilter{
		Genres:  []string{"Drama", "Action"},
		GenreOp: "OR",
	})
	assert.NoError(t, err)
	assert.Len(t, result.Items, 2) // t1 and t2
}

func TestTitleSearch_GenreFilterAND(t *testing.T) {
	db := database.OpenTestDB(t)
	repo := repository.NewTitleRepository(db)

	t1 := createTestTitle(t, db, "movie", 120)
	t2 := createTestTitle(t, db, "series", 45)
	db.Exec(`INSERT INTO title_genres VALUES (?, 'Drama'), (?, 'Action')`, t1.ID, t1.ID)
	db.Exec(`INSERT INTO title_genres VALUES (?, 'Drama')`, t2.ID)

	result, err := repo.List(context.Background(), repository.TitleFilter{
		Genres:  []string{"Drama", "Action"},
		GenreOp: "AND",
	})
	assert.NoError(t, err)
	assert.Len(t, result.Items, 1) // only t1 has both Drama AND Action
	assert.Equal(t, t1.ID, result.Items[0].ID)
}
```

- [ ] **Step 4: Run tests — expect FAIL**

```bash
make test
```

- [ ] **Step 5: Implement genre filtering in the `List` query builder**

In the `buildQuery` function (or equivalent) that assembles the WHERE clause in `title_search.go`, add after other conditions:

```go
if len(filter.Genres) > 0 {
	op := filter.GenreOp
	if op != "AND" {
		op = "OR"
	}
	if op == "OR" {
		placeholders := make([]string, len(filter.Genres))
		for i, g := range filter.Genres {
			placeholders[i] = "?"
			args = append(args, g)
		}
		query += ` AND EXISTS (
			SELECT 1 FROM title_genres tg
			WHERE tg.title_id = t.id
			  AND tg.genre IN (` + strings.Join(placeholders, ",") + `)
		)`
	} else { // AND
		for _, g := range filter.Genres {
			query += ` AND EXISTS (
				SELECT 1 FROM title_genres tg
				WHERE tg.title_id = t.id AND tg.genre = ?
			)`
			args = append(args, g)
		}
	}
}
```

- [ ] **Step 6: Run tests — expect PASS**

```bash
make test
```

- [ ] **Step 7: Commit**

```bash
git add internal/repository/title_search.go internal/repository/title_search_test.go
git commit -m "$(cat <<'EOF'
feat(repository): ajoute le filtrage par genres (AND/OR) dans TitleFilter
EOF
)"
```

---

### Task 5: Also update matching pipeline — store genres in title_genres

**Files:**
- Modify: `internal/service/matching/pipeline.go` (or wherever genres are persisted after enrichment)

- [ ] **Step 1: Find where genres are saved after enrichment**

After matching, the pipeline sets `title.Genres` (previously the JSON field). Now genres must be saved to `title_genres`. Locate the call that persists genre data.

- [ ] **Step 2: Replace genre JSON persistence with title_genres upsert**

Instead of setting `title.Genres`, after the title is saved call:
```go
// Replace all genres for this title
if err := genreRepo.ReplaceForTitle(ctx, titleID, genres); err != nil {
	slog.WarnContext(ctx, "pipeline: save genres", "title_id", titleID, "err", err)
}
```

- [ ] **Step 3: Add `ReplaceForTitle` to `GenreRepository`**

```go
// ReplaceForTitle deletes all existing genres for a title and inserts the new ones.
func (r *GenreRepository) ReplaceForTitle(ctx context.Context, titleID int64, genres []string) error {
	return database.WithTx(r.db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM title_genres WHERE title_id = ?`, titleID); err != nil {
			return fmt.Errorf("genre: replace: delete: %w", err)
		}
		for _, g := range genres {
			if _, err := tx.ExecContext(ctx, `INSERT INTO title_genres (title_id, genre) VALUES (?, ?)`, titleID, g); err != nil {
				return fmt.Errorf("genre: replace: insert %q: %w", g, err)
			}
		}
		return nil
	})
}
```

- [ ] **Step 4: Build**

```bash
make build
```

- [ ] **Step 5: Commit**

```bash
git add internal/service/matching/pipeline.go internal/repository/genre_repository.go
git commit -m "$(cat <<'EOF'
feat(matching): persiste les genres dans title_genres après enrichissement
EOF
)"
```

---

### Task 6: Frontend — Library-First Search + TMDB Toggle

**Files:**
- Modify: `frontend/src/pages/Search.tsx`
- Modify: `frontend/src/pages/Search.module.css`
- Modify: `frontend/src/store.ts`

- [ ] **Step 1: Add `searchOnTMDB` toggle state to `Search.tsx`**

The `Search.tsx` page currently calls `GET /api/tmdb/search`. Change it to call the library search endpoint by default.

```tsx
const [searchOnTMDB, setSearchOnTMDB] = useState(false)
const [tmdbResults, setTmdbResults] = useState<TMDBSearchResult[]>([])
```

- [ ] **Step 2: Library search uses the existing `/api/titles` FTS endpoint**

When `query` changes, search the library:
```tsx
// Library results — uses existing FTS search in /api/titles with a query param
const { data: libraryResults } = useApi<PaginatedResult>(
  query ? `/api/titles?q=${encodeURIComponent(query)}&limit=50` : null
)
```

> Note: Check the existing `Search.tsx` for how it calls the TMDB endpoint today. Mirror the loading/error pattern.

- [ ] **Step 3: TMDB results load when toggle is on**

```tsx
useEffect(() => {
  if (!searchOnTMDB || !query) {
    setTmdbResults([])
    return
  }
  apiFetch<TMDBSearchResult[]>(`/api/tmdb/search?query=${encodeURIComponent(query)}`)
    .then(setTmdbResults)
    .catch(() => setTmdbResults([]))
}, [searchOnTMDB, query])
```

- [ ] **Step 4: Pass `searchOnTMDB` and `setSearchOnTMDB` to `FilterDrawer`**

```tsx
<FilterDrawer
  // ... existing props ...
  searchOnTMDB={searchOnTMDB}
  onSearchOnTMDBChange={setSearchOnTMDB}
/>
```

- [ ] **Step 5: Render library results first, then TMDB results with section header**

```tsx
{/* Library results */}
{libraryResults?.items.map(t => <TitleCard key={t.id} title={t} />)}

{/* TMDB results — only when toggle is on */}
{searchOnTMDB && tmdbResults.length > 0 && (
  <>
    <div className={s.sectionDivider}>TMDB Results</div>
    {tmdbResults
      .filter(r => !libraryResults?.items.some(l => l.tmdb_id === r.id))
      .map(r => <TMDBResultCard key={r.id} result={r} />)}
  </>
)}
```

- [ ] **Step 6: Add `.sectionDivider` CSS**

```css
.sectionDivider {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 16px 6px;
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: var(--text-muted);
}

.sectionDivider::after {
  content: '';
  flex: 1;
  height: 1px;
  background: var(--border);
}
```

- [ ] **Step 7: Build frontend**

```bash
make test-front
```

- [ ] **Step 8: Commit**

```bash
git add frontend/src/pages/Search.tsx frontend/src/pages/Search.module.css
git commit -m "$(cat <<'EOF'
feat(frontend): rend la recherche library-first avec toggle TMDB optionnel
EOF
)"
```

---

### Task 7: Frontend — Genre Filter in FilterDrawer

**Files:**
- Modify: `frontend/src/components/FilterDrawer.tsx`
- Modify: `frontend/src/components/FilterDrawer.module.css`
- Modify: `frontend/src/store.ts`
- Modify: `frontend/src/types.ts`

- [ ] **Step 1: Add types in `types.ts`**

```ts
export interface GenreCount {
  genre: string
  count: number
}
```

- [ ] **Step 2: Add genre fields to `TitleFilter` in `store.ts`**

Find the `TitleFilter` type/interface in `store.ts`:
```ts
genres: string[]     // default: []
genreOp: 'AND' | 'OR' // default: 'OR'
```

Add to localStorage-persisted filter state and to the `resetFilter` function.

- [ ] **Step 3: Add `searchOnTMDB` + genre props to `FilterDrawer`**

```tsx
interface FilterDrawerProps {
  // ... existing props ...
  searchOnTMDB?: boolean
  onSearchOnTMDBChange?: (v: boolean) => void
}
```

- [ ] **Step 4: Fetch genres inside `FilterDrawer` with `useApi`**

```tsx
const { data: genreList } = useApi<GenreCount[]>('/api/genres')
const [genreSearch, setGenreSearch] = useState('')

const filteredGenres = (genreList ?? []).filter(g =>
  g.genre.toLowerCase().includes(genreSearch.toLowerCase())
)

// Selected genres float to top
const sortedGenres = [
  ...filteredGenres.filter(g => selectedGenres.includes(g.genre)),
  ...filteredGenres.filter(g => !selectedGenres.includes(g.genre)),
]
```

Where `selectedGenres` comes from `filter.genres` in the store.

- [ ] **Step 5: Render genre section in FilterDrawer**

```tsx
{/* "Search on TMDB" toggle — only on Search page */}
{onSearchOnTMDBChange && (
  <div className={s.filterRow}>
    <span className={s.filterLabel}>Search on TMDB</span>
    <button
      className={`${s.togglePill} ${searchOnTMDB ? s.toggleOn : ''}`}
      onClick={() => onSearchOnTMDBChange(!searchOnTMDB)}
      aria-pressed={searchOnTMDB}
    />
  </div>
)}

{/* Genre section */}
<div className={s.sectionHeader}>
  <span className={s.sectionLabel}>Genre</span>
  {selectedGenres.length >= 2 && (
    <div className={s.andOrToggle}>
      <button
        className={`${s.andOrBtn} ${genreOp === 'AND' ? s.active : ''}`}
        onClick={() => setGenreOp('AND')}
      >AND</button>
      <button
        className={`${s.andOrBtn} ${genreOp === 'OR' ? s.active : ''}`}
        onClick={() => setGenreOp('OR')}
      >OR</button>
    </div>
  )}
</div>

<input
  className={s.genreSearch}
  placeholder="Filter genres…"
  value={genreSearch}
  onInput={e => setGenreSearch((e.target as HTMLInputElement).value)}
/>

<div className={s.genreList}>
  {sortedGenres.map(g => (
    <button
      key={g.genre}
      className={`${s.genreRow} ${selectedGenres.includes(g.genre) ? s.genreSelected : ''}`}
      onClick={() => toggleGenre(g.genre)}
    >
      <span className={`${s.checkbox} ${selectedGenres.includes(g.genre) ? s.checked : ''}`}>
        {selectedGenres.includes(g.genre) && '✓'}
      </span>
      <span className={s.genreName}>{g.genre}</span>
      <span className={s.genreCount}>{g.count}</span>
    </button>
  ))}
</div>
```

Where `toggleGenre`, `setGenreOp`, `genreOp` are wired to the store:
```tsx
const { filter, setFilter } = useTitleStore()
const selectedGenres = filter.genres ?? []
const genreOp = filter.genreOp ?? 'OR'

function toggleGenre(genre: string) {
  const next = selectedGenres.includes(genre)
    ? selectedGenres.filter(g => g !== genre)
    : [...selectedGenres, genre]
  setFilter({ ...filter, genres: next })
}

function setGenreOp(op: 'AND' | 'OR') {
  setFilter({ ...filter, genreOp: op })
}
```

- [ ] **Step 6: Add CSS for genre section in `FilterDrawer.module.css`**

```css
.sectionHeader { display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px; }
.andOrToggle { display: flex; gap: 2px; background: var(--surface-raised); border-radius: 6px; padding: 2px; }
.andOrBtn { font-size: 10px; font-weight: 700; padding: 3px 8px; border-radius: 4px; border: none; background: none; color: var(--text-muted); cursor: pointer; }
.andOrBtn.active { background: var(--surface-alt); color: var(--text-primary); }
.genreSearch { width: 100%; background: var(--surface-raised); border: none; border-radius: 8px; padding: 7px 10px; font-size: 12px; color: var(--text-primary); margin-bottom: 8px; }
.genreList { max-height: 200px; overflow-y: auto; }
.genreRow { display: flex; align-items: center; gap: 10px; width: 100%; padding: 7px 0; background: none; border: none; border-bottom: 1px solid var(--border); cursor: pointer; }
.genreName { flex: 1; font-size: 13px; color: var(--text-secondary); text-align: left; }
.genreCount { font-size: 11px; color: var(--text-muted); }
.togglePill { width: 36px; height: 20px; border-radius: 10px; background: var(--surface-raised); border: none; position: relative; cursor: pointer; transition: background 0.2s; }
.toggleOn { background: var(--accent-lavender); }
.togglePill::after { content: ''; position: absolute; top: 3px; left: 3px; width: 14px; height: 14px; border-radius: 7px; background: var(--text-muted); transition: left 0.2s; }
.toggleOn::after { left: 19px; background: white; }
```

- [ ] **Step 7: Build frontend**

```bash
make test-front
```

- [ ] **Step 8: Commit**

```bash
git add frontend/src/components/FilterDrawer.tsx frontend/src/components/FilterDrawer.module.css frontend/src/store.ts frontend/src/types.ts
git commit -m "$(cat <<'EOF'
feat(frontend): ajoute le filtre genre (checklist recherchable + AND/OR) dans FilterDrawer
EOF
)"
```

---

### Task 8: Update patterns.md

- [ ] **Step 1: Add new route, repository, and filter fields**

Under Routes: `GET /api/genres — List — Yes`
Under Repositories: `GenreRepository (ListWithCounts, ReplaceForTitle)`
Under FilterDrawer note: accepts `searchOnTMDB` prop (Search page only); genres filter with AND/OR.

- [ ] **Step 2: Commit**

```bash
git add docs/patterns.md
git commit -m "$(cat <<'EOF'
docs(patterns): documente GenreRepository, /api/genres et TitleFilter genres
EOF
)"
```
