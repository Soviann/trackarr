# Library Page Enhancements — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add three features to the Library page: a "Coming up" collapsible strip (upcoming episode air dates), a "Continue Watching" collapsible strip (in-progress titles), and a bulk selection mode for status changes and deletions.

**Architecture:** Two new lazy-loaded API endpoints (`/api/titles/continue-watching`, `/api/titles/upcoming`). Three new title-level endpoints (`DELETE /api/titles/{id}`, `POST /api/titles/batch-delete`, `POST /api/titles/batch-status`). A new `next_air_date`/`next_air_episode` column pair on `titles` populated during TMDB background refresh. On the frontend: a reusable `CollapsibleSection` component wrapping a horizontal `PosterStrip`, plus a bulk-selection mode wired into `Library.tsx`.

**Tech Stack:** Go 1.24, chi, SQLite (migration 017), testify/assert, Preact 10 / TypeScript / CSS Modules

---

## File Map

| Action | File |
|---|---|
| Create | `internal/database/migrations/017_next_air_date.up.sql` |
| Create | `internal/database/migrations/017_next_air_date.down.sql` |
| Modify | `internal/model/title.go` — add `NextAirDate`, `NextAirEpisode` |
| Modify | `internal/repository/title.go` — add fields to queries |
| Modify | `internal/repository/title_search.go` — add fields to SELECT/scan |
| Modify | `internal/service/background.go` — populate next_air_date during refresh |
| Create | `internal/handler/library.go` — continue-watching + upcoming handlers |
| Create | `internal/handler/library_test.go` |
| Modify | `internal/handler/title.go` — add delete + batch handlers |
| Modify | `internal/handler/title_test.go` — delete + batch tests |
| Modify | `cmd/serve.go` — register new routes |
| Modify | `frontend/src/types.ts` — add fields |
| Create | `frontend/src/components/CollapsibleSection.tsx` + `.module.css` |
| Create | `frontend/src/components/PosterStrip.tsx` + `.module.css` |
| Modify | `frontend/src/pages/Library.tsx` — wire strips + bulk mode |
| Modify | `frontend/src/pages/Library.module.css` |
| Modify | `docs/patterns.md` |

---

### Task 1: Migration 017

**Files:**
- Create: `internal/database/migrations/017_next_air_date.up.sql`
- Create: `internal/database/migrations/017_next_air_date.down.sql`

- [ ] **Step 1: Create up migration**

```sql
ALTER TABLE titles ADD COLUMN next_air_date TEXT;
ALTER TABLE titles ADD COLUMN next_air_episode TEXT;
```

- [ ] **Step 2: Create down migration**

```sql
ALTER TABLE titles DROP COLUMN next_air_episode;
ALTER TABLE titles DROP COLUMN next_air_date;
```

- [ ] **Step 3: Run migration**

```bash
make migrate
```

Expected: exits 0.

- [ ] **Step 4: Commit**

```bash
git add internal/database/migrations/017_next_air_date.up.sql internal/database/migrations/017_next_air_date.down.sql
git commit -m "$(cat <<'EOF'
chore(db): ajoute next_air_date et next_air_episode sur titles (migration 017)
EOF
)"
```

---

### Task 2: Model & Repository Updates

**Files:**
- Modify: `internal/model/title.go`
- Modify: `internal/repository/title.go`
- Modify: `internal/repository/title_search.go`

- [ ] **Step 1: Add fields to `internal/model/title.go`**

```go
NextAirDate    *string `json:"next_air_date,omitempty"`
NextAirEpisode *string `json:"next_air_episode,omitempty"`
```

- [ ] **Step 2: Add to repository struct in `internal/repository/title.go`**

```go
NextAirDate    *string
NextAirEpisode *string
```

- [ ] **Step 3: Add to INSERT, UPDATE, and SELECT/Scan in `title.go`**

In INSERT: add `next_air_date, next_air_episode` columns and `title.NextAirDate, title.NextAirEpisode` values.
In UPDATE: add `next_air_date = ?, next_air_episode = ?` and corresponding values.
In SELECT (GetByID): add `next_air_date, next_air_episode` to column list and `&title.NextAirDate, &title.NextAirEpisode` to Scan.

- [ ] **Step 4: Add to `title_search.go` baseCols and scan**

Add `t.next_air_date, t.next_air_episode` to both `baseCols` strings (lines 68 and 352) and the corresponding `&t.NextAirDate, &t.NextAirEpisode` to both Scan calls.

- [ ] **Step 5: Build**

```bash
make build
```

- [ ] **Step 6: Commit**

```bash
git add internal/model/title.go internal/repository/title.go internal/repository/title_search.go
git commit -m "$(cat <<'EOF'
feat(model): ajoute next_air_date et next_air_episode au modèle titre
EOF
)"
```

---

### Task 3: Background Refresh — Populate next_air_date

**Files:**
- Modify: `internal/service/background.go`

- [ ] **Step 1: Locate where TMDB series details are fetched in `background.go`**

Find `refreshSeriesFromTMDB` (or equivalent). The TMDB series detail response already contains `next_episode_to_air` with `air_date` and `season_number`/`episode_number`.

- [ ] **Step 2: Check if the TMDB details struct exposes `next_episode_to_air`**

Look at `internal/service/matching/tmdb.go` for the series detail response struct. If `NextEpisodeToAir` is not already there, add:

```go
type TMDBSeriesDetails struct {
	// ... existing fields ...
	NextEpisodeToAir *struct {
		AirDate       string `json:"air_date"`
		SeasonNumber  int    `json:"season_number"`
		EpisodeNumber int    `json:"episode_number"`
	} `json:"next_episode_to_air"`
}
```

- [ ] **Step 3: Populate `next_air_date` and `next_air_episode` during refresh**

In `refreshSeriesFromTMDB`, after setting other metadata:
```go
if details.NextEpisodeToAir != nil && details.NextEpisodeToAir.AirDate != "" {
	airDate := details.NextEpisodeToAir.AirDate
	airEp := fmt.Sprintf("S%d E%d",
		details.NextEpisodeToAir.SeasonNumber,
		details.NextEpisodeToAir.EpisodeNumber,
	)
	title.NextAirDate = &airDate
	title.NextAirEpisode = &airEp
} else {
	title.NextAirDate = nil
	title.NextAirEpisode = nil
}
```

- [ ] **Step 4: Build**

```bash
make build
```

- [ ] **Step 5: Commit**

```bash
git add internal/service/background.go internal/service/matching/tmdb.go
git commit -m "$(cat <<'EOF'
feat(background): persiste next_air_date depuis next_episode_to_air TMDB lors du refresh
EOF
)"
```

---

### Task 4: Continue Watching & Upcoming API Endpoints

**Files:**
- Create: `internal/handler/library.go`
- Create: `internal/handler/library_test.go`

- [ ] **Step 1: Write the failing tests**

`internal/handler/library_test.go`:
```go
package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLibraryHandler_ContinueWatching(t *testing.T) {
	db := database.OpenTestDB(t)
	h := handler.NewLibraryHandler(repository.NewTitleRepository(db))

	// Setup: one Watching title with unwatched episodes, one Completed title
	// ...

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/titles/continue-watching", nil)
	err := h.ContinueWatching(w, r)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)

	var result []map[string]any
	json.NewDecoder(w.Body).Decode(&result)
	assert.Len(t, result, 1) // only the Watching title
}

func TestLibraryHandler_Upcoming(t *testing.T) {
	db := database.OpenTestDB(t)
	h := handler.NewLibraryHandler(repository.NewTitleRepository(db))

	// Setup: one Watching title with next_air_date in the future, one without
	// ...

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/titles/upcoming", nil)
	err := h.Upcoming(w, r)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)

	var result []map[string]any
	json.NewDecoder(w.Body).Decode(&result)
	assert.Len(t, result, 1)
}
```

- [ ] **Step 2: Run tests — expect FAIL (handler not defined)**

```bash
make test
```

- [ ] **Step 3: Implement `library.go`**

```go
package handler

import (
	"net/http"
	"time"

	"github.com/nicolasvasse/plextracker/internal/handler/httputil"
	"github.com/nicolasvasse/plextracker/internal/repository"
)

type LibraryHandler struct {
	titles *repository.TitleRepository
}

func NewLibraryHandler(titles *repository.TitleRepository) *LibraryHandler {
	return &LibraryHandler{titles: titles}
}

// ContinueWatching returns Watching titles with ≥1 unwatched episode, ordered by last_watched_at DESC.
func (h *LibraryHandler) ContinueWatching(w http.ResponseWriter, r *http.Request) error {
	titles, err := h.titles.ListContinueWatching(r.Context())
	if err != nil {
		return fmt.Errorf("library: continue watching: %w", err)
	}
	return httputil.WriteJSON(w, titles)
}

// Upcoming returns Watching/PlanToWatch titles with next_air_date >= today, ordered by next_air_date ASC.
func (h *LibraryHandler) Upcoming(w http.ResponseWriter, r *http.Request) error {
	today := time.Now().Format("2006-01-02")
	titles, err := h.titles.ListUpcoming(r.Context(), today)
	if err != nil {
		return fmt.Errorf("library: upcoming: %w", err)
	}
	return httputil.WriteJSON(w, titles)
}
```

- [ ] **Step 4: Add `ListContinueWatching` and `ListUpcoming` to `internal/repository/title.go`**

```go
// ListContinueWatching returns Watching titles that have at least one unwatched episode,
// ordered by last_watched_at DESC.
func (r *TitleRepository) ListContinueWatching(ctx context.Context) ([]Title, error) {
	query := `
		SELECT t.id, t.type, t.is_anime, t.cover_url, t.status, t.runtime,
		       t.last_watched_at, tn.name,
		       (SELECT COUNT(*) FROM episodes e
		        JOIN seasons s ON e.season_id = s.id
		        WHERE s.title_id = t.id AND e.watched = 0) AS unwatched_count,
		       (SELECT COUNT(*) FROM episodes e
		        JOIN seasons s ON e.season_id = s.id
		        WHERE s.title_id = t.id) AS total_episodes,
		       (SELECT COUNT(*) FROM episodes e
		        JOIN seasons s ON e.season_id = s.id
		        WHERE s.title_id = t.id AND e.watched = 1) AS watched_episodes,
		       t.next_air_episode
		FROM titles t
		JOIN title_names tn ON tn.title_id = t.id AND tn.language = 'fr'
		WHERE t.status = 'watching'
		  AND (SELECT COUNT(*) FROM episodes e
		       JOIN seasons s ON e.season_id = s.id
		       WHERE s.title_id = t.id AND e.watched = 0) > 0
		ORDER BY t.last_watched_at DESC`
	// ... scan rows into []Title
}

// ListUpcoming returns Watching and PlanToWatch titles with next_air_date >= today,
// ordered by next_air_date ASC.
func (r *TitleRepository) ListUpcoming(ctx context.Context, today string) ([]Title, error) {
	query := `
		SELECT t.id, t.type, t.is_anime, t.cover_url, t.status,
		       t.next_air_date, t.next_air_episode, tn.name
		FROM titles t
		JOIN title_names tn ON tn.title_id = t.id AND tn.language = 'fr'
		WHERE t.status IN ('watching', 'plan_to_watch')
		  AND t.next_air_date IS NOT NULL
		  AND t.next_air_date >= ?
		ORDER BY t.next_air_date ASC`
	// ... scan rows
}
```

> Note: The `title_names` join for preferred language should follow the same pattern as existing list queries. Check `title_search.go` for the exact pattern (it may use a subquery or a different language fallback). Mirror that pattern.

- [ ] **Step 5: Run tests — expect PASS**

```bash
make test
```

- [ ] **Step 6: Commit**

```bash
git add internal/handler/library.go internal/handler/library_test.go internal/repository/title.go
git commit -m "$(cat <<'EOF'
feat(handler): ajoute les endpoints /continue-watching et /upcoming
EOF
)"
```

---

### Task 5: Delete & Batch Endpoints

**Files:**
- Modify: `internal/handler/title.go`
- Modify: `internal/handler/title_test.go`

- [ ] **Step 1: Write failing tests**

In `internal/handler/title_test.go`:
```go
func TestTitleHandler_Delete(t *testing.T) {
	db := database.OpenTestDB(t)
	h := setupTitleHandler(t, db)
	title := createTestTitle(t, db, "movie", 120)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, "/api/titles/"+strconv.Itoa(int(title.ID)), nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.Itoa(int(title.ID)))
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	err := h.Delete(w, r)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, w.Code)

	// Verify deleted
	_, err = titleRepo(db).GetByID(title.ID)
	assert.Error(t, err) // should be sql.ErrNoRows or similar
}

func TestTitleHandler_BatchDelete(t *testing.T) {
	db := database.OpenTestDB(t)
	h := setupTitleHandler(t, db)
	t1 := createTestTitle(t, db, "movie", 90)
	t2 := createTestTitle(t, db, "series", 45)

	body := fmt.Sprintf(`{"ids":[%d,%d]}`, t1.ID, t2.ID)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/titles/batch-delete", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")

	err := h.BatchDelete(w, r)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestTitleHandler_BatchStatus(t *testing.T) {
	db := database.OpenTestDB(t)
	h := setupTitleHandler(t, db)
	t1 := createTestTitle(t, db, "movie", 90)
	t2 := createTestTitle(t, db, "series", 45)

	body := fmt.Sprintf(`{"ids":[%d,%d],"status":"completed"}`, t1.ID, t2.ID)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/titles/batch-status", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")

	err := h.BatchStatus(w, r)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, w.Code)

	updated1, _ := titleRepo(db).GetByID(t1.ID)
	assert.Equal(t, "completed", string(updated1.Status))
}
```

- [ ] **Step 2: Run tests — expect FAIL**

```bash
make test
```

- [ ] **Step 3: Add `Delete`, `BatchDelete`, `BatchStatus` to `internal/handler/title.go`**

```go
func (h *TitleHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	id, err := httputil.ParseIDParam(r, "id")
	if err != nil {
		return err
	}
	if err := h.titles.Delete(r.Context(), id); err != nil {
		return fmt.Errorf("title: delete: %w", err)
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *TitleHandler) BatchDelete(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		IDs []int64 `json:"ids"`
	}
	if err := httputil.ReadJSON(r, &body); err != nil {
		return httputil.APIError(http.StatusBadRequest, "invalid body")
	}
	if len(body.IDs) == 0 {
		return httputil.APIError(http.StatusBadRequest, "ids is required")
	}
	if err := h.titles.BatchDelete(r.Context(), body.IDs); err != nil {
		return fmt.Errorf("title: batch delete: %w", err)
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *TitleHandler) BatchStatus(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		IDs    []int64 `json:"ids"`
		Status string  `json:"status"`
	}
	if err := httputil.ReadJSON(r, &body); err != nil {
		return httputil.APIError(http.StatusBadRequest, "invalid body")
	}
	if len(body.IDs) == 0 || body.Status == "" {
		return httputil.APIError(http.StatusBadRequest, "ids and status are required")
	}
	validStatuses := map[string]bool{"watching": true, "completed": true, "dropped": true, "plan_to_watch": true}
	if !validStatuses[body.Status] {
		return httputil.APIError(http.StatusBadRequest, "invalid status")
	}
	if err := h.titles.BatchUpdateStatus(r.Context(), body.IDs, body.Status); err != nil {
		return fmt.Errorf("title: batch status: %w", err)
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
```

- [ ] **Step 4: Add `Delete`, `BatchDelete`, `BatchUpdateStatus` to `internal/repository/title.go`**

```go
func (r *TitleRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM titles WHERE id = ?`, id)
	return err
}

func (r *TitleRepository) BatchDelete(ctx context.Context, ids []int64) error {
	// Build placeholders: DELETE FROM titles WHERE id IN (?,?,?)
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	query := `DELETE FROM titles WHERE id IN (` + strings.Join(placeholders, ",") + `)`
	_, err := r.db.ExecContext(ctx, query, args...)
	return err
}

func (r *TitleRepository) BatchUpdateStatus(ctx context.Context, ids []int64, status string) error {
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids)+1)
	args[0] = status
	for i, id := range ids {
		placeholders[i] = "?"
		args[i+1] = id
	}
	query := `UPDATE titles SET status = ? WHERE id IN (` + strings.Join(placeholders, ",") + `)`
	_, err := r.db.ExecContext(ctx, query, args...)
	return err
}
```

> Note: Cascade deletes (seasons, episodes, watch_events, title_names) should be handled by SQLite foreign key constraints (`ON DELETE CASCADE`). Check the initial migration `001_init.up.sql` to confirm cascades are set. If not, add explicit deletes in the `Delete` function before deleting the title.

- [ ] **Step 5: Run tests — expect PASS**

```bash
make test
```

- [ ] **Step 6: Commit**

```bash
git add internal/handler/title.go internal/handler/title_test.go internal/repository/title.go
git commit -m "$(cat <<'EOF'
feat(handler): ajoute DELETE /titles/{id}, batch-delete et batch-status
EOF
)"
```

---

### Task 6: Route Registration

**Files:**
- Modify: `cmd/serve.go`

- [ ] **Step 1: Register all new routes in the authenticated group**

```go
libraryHandler := handler.NewLibraryHandler(titleRepo)

// Authenticated routes — add these:
r.With(authMiddleware).Get("/api/titles/continue-watching", httputil.WrapHandler(libraryHandler.ContinueWatching))
r.With(authMiddleware).Get("/api/titles/upcoming", httputil.WrapHandler(libraryHandler.Upcoming))
r.With(authMiddleware).Delete("/api/titles/{id}", httputil.WrapHandler(titleHandler.Delete))
r.With(authMiddleware).Post("/api/titles/batch-delete", httputil.WrapHandler(titleHandler.BatchDelete))
r.With(authMiddleware).Post("/api/titles/batch-status", httputil.WrapHandler(titleHandler.BatchStatus))
```

> Important: `/api/titles/continue-watching` and `/api/titles/upcoming` must be registered **before** `/api/titles/{id}` in chi, otherwise chi will match `continue-watching` as an `{id}` param. Verify the order.

- [ ] **Step 2: Build**

```bash
make build
```

- [ ] **Step 3: Commit**

```bash
git add cmd/serve.go
git commit -m "$(cat <<'EOF'
feat(router): enregistre les routes library, delete et batch dans chi
EOF
)"
```

---

### Task 7: Frontend — CollapsibleSection & PosterStrip Components

**Files:**
- Create: `frontend/src/components/CollapsibleSection.tsx`
- Create: `frontend/src/components/CollapsibleSection.module.css`
- Create: `frontend/src/components/PosterStrip.tsx`
- Create: `frontend/src/components/PosterStrip.module.css`
- Modify: `frontend/src/types.ts`

- [ ] **Step 1: Add types for the new endpoints in `types.ts`**

```ts
export interface ContinueWatchingTitle {
  id: number
  type: string
  cover_url: string | null
  name: string
  next_air_episode: string | null
  watched_episodes: number
  total_episodes: number
  last_watched_at: string | null
}

export interface UpcomingTitle {
  id: number
  type: string
  cover_url: string | null
  name: string
  next_air_date: string
  next_air_episode: string | null
  status: string
}
```

- [ ] **Step 2: Create `CollapsibleSection.tsx`**

```tsx
import { useState, useEffect, useRef } from 'preact/hooks'
import type { ComponentChildren } from 'preact'
import s from './CollapsibleSection.module.css'

interface Props {
  title: string
  count?: number
  children: ComponentChildren
  onExpand?: () => void
}

export function CollapsibleSection({ title, count, children, onExpand }: Props) {
  const [open, setOpen] = useState(false)
  const didLoad = useRef(false)

  function toggle() {
    const next = !open
    setOpen(next)
    if (next && !didLoad.current) {
      didLoad.current = true
      onExpand?.()
    }
  }

  return (
    <div className={s.section}>
      <button className={s.header} onClick={toggle} aria-expanded={open}>
        <span className={s.title}>{title}</span>
        {count !== undefined && <span className={s.count}>{count}</span>}
        <span className={`${s.arrow} ${open ? s.open : ''}`}>›</span>
      </button>
      {open && <div className={s.body}>{children}</div>}
    </div>
  )
}
```

- [ ] **Step 3: Create `CollapsibleSection.module.css`**

```css
.section {
  border-bottom: 1px solid var(--border);
}

.header {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 10px 16px;
  background: var(--surface-alt);
  border: none;
  cursor: pointer;
  text-align: left;
}

.title {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-secondary);
  flex: 1;
}

.count {
  font-size: 11px;
  font-weight: 600;
  padding: 1px 7px;
  border-radius: 10px;
  background: var(--surface-raised);
  color: var(--text-muted);
}

.arrow {
  font-size: 14px;
  color: var(--text-muted);
  transition: transform 0.2s;
}

.arrow.open {
  transform: rotate(90deg);
}

.body {
  /* content area */
}
```

- [ ] **Step 4: Create `PosterStrip.tsx`**

```tsx
import { Link } from 'preact-router/match'
import s from './PosterStrip.module.css'
import { CoverPlaceholder } from './CoverPlaceholder'

interface PosterStripItem {
  id: number
  cover_url: string | null
  name: string
  sublabel: string        // e.g. "S2 · E5" or air date badge text
  sublabelVariant?: 'default' | 'amber' | 'teal' | 'muted'
  progressRatio?: number  // 0–1, shown as amber bar if provided
}

interface Props {
  items: PosterStripItem[]
}

export function PosterStrip({ items }: Props) {
  return (
    <div className={s.strip}>
      {items.map(item => (
        <Link key={item.id} href={`/titles/${item.id}`} className={s.card}>
          <div className={s.poster}>
            {item.cover_url
              ? <img src={item.cover_url} alt="" role="presentation" />
              : <CoverPlaceholder />}
            {item.progressRatio !== undefined && (
              <div className={s.progressBar}>
                <div className={s.progressFill} style={{ width: `${item.progressRatio * 100}%` }} />
              </div>
            )}
            {item.progressRatio === undefined && (
              <span className={`${s.badge} ${s[`badge_${item.sublabelVariant ?? 'default'}`]}`}>
                {item.sublabel}
              </span>
            )}
          </div>
          <div className={s.info}>
            <span className={s.name}>{item.name}</span>
            {item.progressRatio !== undefined && (
              <span className={s.ep}>{item.sublabel}</span>
            )}
          </div>
        </Link>
      ))}
    </div>
  )
}
```

- [ ] **Step 5: Create `PosterStrip.module.css`**

```css
.strip {
  display: flex;
  gap: 10px;
  padding: 10px 16px 14px;
  overflow-x: auto;
  scrollbar-width: none;
}

.strip::-webkit-scrollbar { display: none; }

.card {
  flex: 0 0 100px;
  text-decoration: none;
  color: inherit;
}

.poster {
  width: 100px;
  height: 140px;
  border-radius: 8px;
  overflow: hidden;
  background: var(--surface-raised);
  position: relative;
}

.poster img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.progressBar {
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  height: 3px;
  background: var(--surface-raised);
}

.progressFill {
  height: 100%;
  background: var(--accent-amber);
  border-radius: 0 2px 2px 0;
}

.badge {
  position: absolute;
  top: 5px;
  right: 5px;
  font-size: 9px;
  font-weight: 700;
  padding: 2px 5px;
  border-radius: 4px;
}

.badge_amber { background: var(--accent-amber); color: var(--bg); }
.badge_teal  { background: var(--accent-teal);  color: var(--bg); }
.badge_muted { background: var(--surface-raised); color: var(--text-muted); }
.badge_default { background: var(--surface-raised); color: var(--text-secondary); }

.info {
  padding: 5px 2px 0;
}

.name {
  display: block;
  font-size: 10px;
  font-weight: 600;
  color: var(--text-secondary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.ep {
  display: block;
  font-size: 9px;
  color: var(--text-muted);
  margin-top: 1px;
}
```

- [ ] **Step 6: Build frontend**

```bash
make test-front
```

- [ ] **Step 7: Commit**

```bash
git add frontend/src/components/CollapsibleSection.tsx frontend/src/components/CollapsibleSection.module.css frontend/src/components/PosterStrip.tsx frontend/src/components/PosterStrip.module.css frontend/src/types.ts
git commit -m "$(cat <<'EOF'
feat(frontend): ajoute CollapsibleSection et PosterStrip — composants réutilisables Library
EOF
)"
```

---

### Task 8: Frontend — Wire Strips into Library.tsx

**Files:**
- Modify: `frontend/src/pages/Library.tsx`
- Modify: `frontend/src/pages/Library.module.css`

- [ ] **Step 1: Add lazy-loaded hooks for the two strips in `Library.tsx`**

```tsx
import { useState, useCallback } from 'preact/hooks'
import { apiFetch } from '../api'
import type { ContinueWatchingTitle, UpcomingTitle } from '../types'
import { CollapsibleSection } from '../components/CollapsibleSection'
import { PosterStrip } from '../components/PosterStrip'

// Inside the Library component:
const [continueWatching, setContinueWatching] = useState<ContinueWatchingTitle[] | null>(null)
const [upcoming, setUpcoming] = useState<UpcomingTitle[] | null>(null)

const loadContinueWatching = useCallback(async () => {
  if (continueWatching !== null) return
  const data = await apiFetch<ContinueWatchingTitle[]>('/api/titles/continue-watching')
  setContinueWatching(data)
}, [continueWatching])

const loadUpcoming = useCallback(async () => {
  if (upcoming !== null) return
  const data = await apiFetch<UpcomingTitle[]>('/api/titles/upcoming')
  setUpcoming(data)
}, [upcoming])
```

- [ ] **Step 2: Add airdate badge helper**

```tsx
function airDateBadge(dateStr: string): { label: string; variant: 'amber' | 'teal' | 'muted' } {
  const today = new Date()
  today.setHours(0, 0, 0, 0)
  const air = new Date(dateStr)
  air.setHours(0, 0, 0, 0)
  const diffDays = Math.round((air.getTime() - today.getTime()) / 86_400_000)
  if (diffDays === 0) return { label: 'Today', variant: 'amber' }
  if (diffDays <= 6) return { label: air.toLocaleDateString('fr-FR', { weekday: 'short' }), variant: 'teal' }
  return { label: `in ${diffDays}d`, variant: 'muted' }
}
```

- [ ] **Step 3: Render the two strips at the top of the Library return**

```tsx
// In the Library JSX, before the main grid/list:
<CollapsibleSection
  title="Coming up"
  count={upcoming?.length}
  onExpand={loadUpcoming}
>
  {upcoming && (
    <PosterStrip items={upcoming.map(t => {
      const { label, variant } = airDateBadge(t.next_air_date)
      return {
        id: t.id,
        cover_url: t.cover_url,
        name: t.name,
        sublabel: label,
        sublabelVariant: variant,
      }
    })} />
  )}
</CollapsibleSection>

<CollapsibleSection
  title="Continue Watching"
  count={continueWatching?.length}
  onExpand={loadContinueWatching}
>
  {continueWatching && (
    <PosterStrip items={continueWatching.map(t => ({
      id: t.id,
      cover_url: t.cover_url,
      name: t.name,
      sublabel: t.next_air_episode ?? '',
      progressRatio: t.total_episodes > 0
        ? t.watched_episodes / t.total_episodes
        : 0,
    }))} />
  )}
</CollapsibleSection>
```

- [ ] **Step 4: Build frontend**

```bash
make test-front
```

- [ ] **Step 5: Commit**

```bash
git add frontend/src/pages/Library.tsx frontend/src/pages/Library.module.css
git commit -m "$(cat <<'EOF'
feat(frontend): intègre les strips Coming Up et Continue Watching dans la Library
EOF
)"
```

---

### Task 9: Frontend — Bulk Selection Mode

**Files:**
- Modify: `frontend/src/pages/Library.tsx`
- Modify: `frontend/src/pages/Library.module.css`

- [ ] **Step 1: Add selection state to `Library.tsx`**

```tsx
const [selecting, setSelecting] = useState(false)
const [selected, setSelected] = useState<Set<number>>(new Set())

function toggleSelect(id: number) {
  setSelected(prev => {
    const next = new Set(prev)
    if (next.has(id)) next.delete(id)
    else next.add(id)
    return next
  })
}

function selectAll() {
  setSelected(new Set(titles.map(t => t.id)))
}

function exitSelect() {
  setSelecting(false)
  setSelected(new Set())
}
```

- [ ] **Step 2: Add "Select" button to the top bar**

In the Library top bar JSX, alongside existing filter/sort controls:
```tsx
<button
  className={`${s.selectBtn} ${selecting ? s.selectBtnActive : ''}`}
  onClick={() => selecting ? exitSelect() : setSelecting(true)}
>
  {selecting ? 'Cancel' : 'Select'}
</button>
```

- [ ] **Step 3: Add "Select all / count" row below top bar when in selection mode**

```tsx
{selecting && (
  <div className={s.selectAllRow}>
    <button className={s.selectAllBtn} onClick={selectAll}>
      Select all
    </button>
    <span className={s.selectCount}>{selected.size} of {titles.length}</span>
  </div>
)}
```

- [ ] **Step 4: Pass `selecting` and `onSelect` props to `PosterCard` and `TitleCard`**

In `PosterCard.tsx` and `TitleCard.tsx`, add props:
```tsx
interface Props {
  // ... existing props ...
  selecting?: boolean
  selected?: boolean
  onSelect?: (id: number) => void
}
```

When `selecting` is true, clicking the card calls `onSelect(id)` instead of navigating. Render a checkbox overlay in the top-left corner:
```tsx
{props.selecting && (
  <div className={`${s.checkbox} ${props.selected ? s.checked : ''}`}>
    {props.selected && '✓'}
  </div>
)}
```

- [ ] **Step 5: Add action bar**

```tsx
{selecting && selected.size > 0 && (
  <div className={s.actionBar}>
    <span className={s.actionBarLabel}>{selected.size} selected</span>
    <button className={s.actionBtnStatus} onClick={() => setStatusSheetOpen(true)}>
      Status
    </button>
    <button className={s.actionBtnDelete} onClick={() => setDeleteConfirmOpen(true)}>
      Delete
    </button>
  </div>
)}
```

- [ ] **Step 6: Add status picker sheet**

```tsx
const [statusSheetOpen, setStatusSheetOpen] = useState(false)

async function applyBulkStatus(status: string) {
  await apiFetch('/api/titles/batch-status', {
    method: 'POST',
    body: JSON.stringify({ ids: [...selected], status }),
  })
  setStatusSheetOpen(false)
  exitSelect()
  // Refresh library titles
  store.refresh()
}
```

Status sheet renders in a `BottomSheet` (already exists in components) with 4 status options.

- [ ] **Step 7: Add delete confirmation**

```tsx
const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false)

async function confirmBulkDelete() {
  await apiFetch('/api/titles/batch-delete', {
    method: 'POST',
    body: JSON.stringify({ ids: [...selected] }),
  })
  setDeleteConfirmOpen(false)
  exitSelect()
  store.refresh()
}
```

Delete confirmation renders in `ConfirmationDrawer` (already exists) with message `"Delete ${selected.size} title${selected.size > 1 ? 's' : ''}? This cannot be undone."`.

- [ ] **Step 8: Add CSS for new elements in `Library.module.css`**

```css
.selectBtn { font-size: 12px; font-weight: 600; padding: 5px 10px; border-radius: 6px; background: var(--surface-raised); color: var(--text-muted); border: none; cursor: pointer; }
.selectBtnActive { background: color-mix(in srgb, var(--accent-amber) 20%, transparent); color: var(--accent-amber); }
.selectAllRow { display: flex; align-items: center; justify-content: space-between; padding: 8px 16px; border-bottom: 1px solid var(--border); }
.selectAllBtn { font-size: 12px; font-weight: 600; color: var(--accent-amber); background: none; border: none; cursor: pointer; }
.selectCount { font-size: 12px; color: var(--text-muted); }
.actionBar { position: fixed; bottom: 56px; left: 0; right: 0; background: var(--surface-alt); border-top: 1px solid var(--border); padding: 10px 16px; display: flex; align-items: center; gap: 10px; z-index: 10; }
.actionBarLabel { flex: 1; font-size: 12px; color: var(--text-muted); }
.actionBtnStatus { font-size: 12px; font-weight: 600; padding: 7px 12px; border-radius: 7px; background: color-mix(in srgb, var(--accent-teal) 20%, transparent); color: var(--accent-teal); border: none; cursor: pointer; }
.actionBtnDelete { font-size: 12px; font-weight: 600; padding: 7px 12px; border-radius: 7px; background: color-mix(in srgb, var(--accent-coral) 20%, transparent); color: var(--accent-coral); border: none; cursor: pointer; }
.checkbox { position: absolute; top: 6px; left: 6px; width: 20px; height: 20px; border-radius: 10px; border: 2px solid rgba(255,255,255,0.4); background: rgba(0,0,0,0.4); display: flex; align-items: center; justify-content: center; font-size: 11px; color: white; }
.checked { background: var(--accent-amber); border-color: var(--accent-amber); }
```

- [ ] **Step 9: Build frontend**

```bash
make test-front
```

- [ ] **Step 10: Commit**

```bash
git add frontend/src/pages/Library.tsx frontend/src/pages/Library.module.css frontend/src/components/PosterCard.tsx frontend/src/components/PosterCard.module.css frontend/src/components/TitleCard.tsx frontend/src/components/TitleCard.module.css
git commit -m "$(cat <<'EOF'
feat(frontend): ajoute le mode de sélection multiple dans la Library (statut + suppression)
EOF
)"
```

---

### Task 10: Update patterns.md

- [ ] **Step 1: Add new routes, components, and repository methods to `docs/patterns.md`**

Under Routes, add:
```
GET  /api/titles/continue-watching  ContinueWatching  Yes
GET  /api/titles/upcoming           Upcoming           Yes
DELETE /api/titles/{id}             Delete             Yes
POST /api/titles/batch-delete       BatchDelete        Yes
POST /api/titles/batch-status       BatchStatus        Yes
```

Under Components, add `CollapsibleSection` and `PosterStrip`.

- [ ] **Step 2: Commit**

```bash
git add docs/patterns.md
git commit -m "$(cat <<'EOF'
docs(patterns): documente les nouvelles routes library, composants et méthodes repo
EOF
)"
```
