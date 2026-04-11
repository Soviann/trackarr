# Richer Stats & Activity Feed — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend the Stats page with watchtime, genre breakdown, and streak cards. Add a global activity feed section and a per-title watch history view on TitleDetail.

**Architecture:** Extend `GET /api/stats` response with `total_watch_minutes`, `genres`, and `streaks`. Two new endpoints: `GET /api/stats/activity` (paginated watch events) and `GET /api/titles/{id}/history` (per-title watch log). Frontend changes are confined to `Stats.tsx` and `TitleDetail.tsx`, plus a new `TitleHistory` component. Soft dependencies: watchtime card requires `total_watch_minutes` column (plan `2026-04-11-watchtime`); genre chart requires `title_genres` table (plan `2026-04-11-search-filter`). Both are handled gracefully with empty states.

**Tech Stack:** Go 1.24, chi, SQLite, testify/assert, Preact 10 / TypeScript / CSS Modules

---

## File Map

| Action | File |
|---|---|
| Modify | `internal/repository/stats.go` — add watchtime, genres, streaks queries |
| Modify | `internal/repository/stats_test.go` |
| Create | `internal/handler/activity.go` — GET /api/stats/activity |
| Create | `internal/handler/activity_test.go` |
| Create | `internal/handler/history.go` — GET /api/titles/{id}/history |
| Create | `internal/handler/history_test.go` |
| Modify | `cmd/serve.go` — register two new routes |
| Modify | `frontend/src/types.ts` — extend StatsResponse, add Activity/History types |
| Modify | `frontend/src/pages/Stats.tsx` — add new sections |
| Modify | `frontend/src/pages/Stats.module.css` |
| Modify | `frontend/src/pages/TitleDetail.tsx` — add Historique button |
| Create | `frontend/src/components/TitleHistory.tsx` + `.module.css` |
| Modify | `docs/patterns.md` |

---

### Task 1: Stats Repository — Watchtime, Genres, Streaks

**Files:**
- Modify: `internal/repository/stats.go`
- Modify: `internal/repository/stats_test.go` (or create)

- [ ] **Step 1: Write failing tests**

```go
func TestStatsRepo_TotalWatchMinutes(t *testing.T) {
	db := database.OpenTestDB(t)
	repo := repository.NewStatsRepository(db)

	// No titles → 0
	total, err := repo.TotalWatchMinutes(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 0, total)

	// Insert titles with total_watch_minutes
	db.Exec(`INSERT INTO titles (type, total_watch_minutes) VALUES ('movie', 120), ('series', 600)`)
	total, err = repo.TotalWatchMinutes(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 720, total)
}

func TestStatsRepo_CurrentStreak(t *testing.T) {
	db := database.OpenTestDB(t)
	repo := repository.NewStatsRepository(context.Background())

	// Seed watch events on consecutive days
	today := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	twoDaysAgo := time.Now().AddDate(0, 0, -2).Format("2006-01-02")

	db.Exec(`INSERT INTO watch_events (title_id, watched_at) VALUES (1, ?), (1, ?), (1, ?)`,
		today+" 21:00:00", yesterday+" 20:00:00", twoDaysAgo+" 19:00:00")

	streak, err := repo.CurrentStreak(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 3, streak)
}

func TestStatsRepo_BestStreak(t *testing.T) {
	db := database.OpenTestDB(t)
	repo := repository.NewStatsRepository(db)

	// 5 consecutive days, then a gap, then 2 days
	for i := 10; i >= 6; i-- {
		d := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		db.Exec(`INSERT INTO watch_events (title_id, watched_at) VALUES (1, ?)`, d+" 20:00:00")
	}
	for i := 2; i >= 1; i-- {
		d := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		db.Exec(`INSERT INTO watch_events (title_id, watched_at) VALUES (1, ?)`, d+" 20:00:00")
	}

	best, err := repo.BestStreak(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 5, best)
}
```

- [ ] **Step 2: Run tests — expect FAIL**

```bash
make test
```

- [ ] **Step 3: Implement `TotalWatchMinutes` in `stats.go`**

```go
func (r *StatsRepository) TotalWatchMinutes(ctx context.Context) (int, error) {
	var total int
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(total_watch_minutes), 0) FROM titles
	`).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("stats: total watch minutes: %w", err)
	}
	return total, nil
}
```

- [ ] **Step 4: Implement `TopGenres` in `stats.go`**

```go
type GenreStat struct {
	Genre string `json:"genre"`
	Count int    `json:"count"`
}

// TopGenres returns top N genres by title count. Returns empty slice if title_genres table doesn't exist.
func (r *StatsRepository) TopGenres(ctx context.Context, limit int) ([]GenreStat, error) {
	// Check table exists to handle soft dependency gracefully
	var tableExists int
	r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='title_genres'
	`).Scan(&tableExists)
	if tableExists == 0 {
		return []GenreStat{}, nil
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT genre, COUNT(*) AS count
		FROM title_genres
		GROUP BY genre
		ORDER BY count DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("stats: top genres: %w", err)
	}
	defer rows.Close()

	var results []GenreStat
	for rows.Next() {
		var g GenreStat
		if err := rows.Scan(&g.Genre, &g.Count); err != nil {
			return nil, fmt.Errorf("stats: genre scan: %w", err)
		}
		results = append(results, g)
	}
	return results, rows.Err()
}
```

- [ ] **Step 5: Implement `CurrentStreak` and `BestStreak` in `stats.go`**

```go
// CurrentStreak returns the number of consecutive calendar days (ending today or yesterday) with ≥1 watch event.
func (r *StatsRepository) CurrentStreak(ctx context.Context) (int, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT DATE(watched_at) AS day
		FROM watch_events
		ORDER BY day DESC
		LIMIT 400
	`)
	if err != nil {
		return 0, fmt.Errorf("stats: current streak: %w", err)
	}
	defer rows.Close()

	var days []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return 0, err
		}
		days = append(days, d)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	return computeCurrentStreak(days, time.Now()), nil
}

func computeCurrentStreak(days []string, now time.Time) int {
	if len(days) == 0 {
		return 0
	}
	today := now.Format("2006-01-02")
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")

	// Streak must end today or yesterday
	if days[0] != today && days[0] != yesterday {
		return 0
	}

	streak := 1
	for i := 1; i < len(days); i++ {
		prev, _ := time.Parse("2006-01-02", days[i-1])
		curr, _ := time.Parse("2006-01-02", days[i])
		if prev.AddDate(0, 0, -1).Format("2006-01-02") == curr.Format("2006-01-02") {
			streak++
		} else {
			break
		}
	}
	return streak
}

// BestStreak returns the longest ever consecutive watch streak.
func (r *StatsRepository) BestStreak(ctx context.Context) (int, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT DATE(watched_at) AS day
		FROM watch_events
		ORDER BY day ASC
	`)
	if err != nil {
		return 0, fmt.Errorf("stats: best streak: %w", err)
	}
	defer rows.Close()

	var days []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return 0, err
		}
		days = append(days, d)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	return computeBestStreak(days), nil
}

func computeBestStreak(days []string) int {
	if len(days) == 0 {
		return 0
	}
	best, current := 1, 1
	for i := 1; i < len(days); i++ {
		prev, _ := time.Parse("2006-01-02", days[i-1])
		curr, _ := time.Parse("2006-01-02", days[i])
		if prev.AddDate(0, 0, 1).Format("2006-01-02") == curr.Format("2006-01-02") {
			current++
			if current > best {
				best = current
			}
		} else {
			current = 1
		}
	}
	return best
}
```

- [ ] **Step 6: Extend the existing stats `Get` handler to include the new fields**

In `internal/handler/stats.go`, find the `Get` handler. Extend the response struct and populate the new fields:

```go
type statsResponse struct {
	// ... existing fields ...
	TotalWatchMinutes int           `json:"total_watch_minutes"`
	Genres            []GenreStat   `json:"genres"`
	Streaks           streakStat    `json:"streaks"`
}

type streakStat struct {
	Current int `json:"current"`
	Best    int `json:"best"`
}

// In the handler:
totalWatchMinutes, _ := h.stats.TotalWatchMinutes(r.Context())
genres, _ := h.stats.TopGenres(r.Context(), 10)
currentStreak, _ := h.stats.CurrentStreak(r.Context())
bestStreak, _ := h.stats.BestStreak(r.Context())
```

- [ ] **Step 7: Run tests — expect PASS**

```bash
make test
```

- [ ] **Step 8: Commit**

```bash
git add internal/repository/stats.go internal/repository/stats_test.go internal/handler/stats.go
git commit -m "$(cat <<'EOF'
feat(stats): ajoute watchtime total, top genres et séries de jours dans l'API stats
EOF
)"
```

---

### Task 2: Activity Feed Endpoint

**Files:**
- Create: `internal/handler/activity.go`
- Create: `internal/handler/activity_test.go`
- Modify: `cmd/serve.go`

- [ ] **Step 1: Write the failing test**

```go
func TestActivityHandler_List(t *testing.T) {
	db := database.OpenTestDB(t)

	// Setup: a movie title + watch event
	db.Exec(`INSERT INTO titles (id, type, runtime) VALUES (1, 'movie', 120)`)
	db.Exec(`INSERT INTO title_names (title_id, name, language) VALUES (1, 'Dune', 'fr')`)
	db.Exec(`INSERT INTO watch_events (title_id, watched_at) VALUES (1, '2026-04-10 21:00:00')`)

	h := handler.NewActivityHandler(database.NewActivityRepository(db))
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/stats/activity?limit=50&offset=0", nil)

	err := h.List(w, r)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)

	var result []map[string]any
	json.NewDecoder(w.Body).Decode(&result)
	assert.Len(t, result, 1)
	assert.Equal(t, "Dune", result[0]["title_name"])
}
```

- [ ] **Step 2: Run test — expect FAIL**

```bash
make test
```

- [ ] **Step 3: Add `ActivityRepository` to `internal/repository/`**

Create `internal/repository/activity_repository.go`:

```go
package repository

import (
	"context"
	"fmt"

	"github.com/nicolasvasse/plextracker/internal/database"
)

type ActivityEvent struct {
	TitleID      int64   `json:"title_id"`
	TitleName    string  `json:"title_name"`
	CoverURL     *string `json:"cover_url"`
	TitleType    string  `json:"title_type"`
	EpisodeID    *int64  `json:"episode_id,omitempty"`
	EpisodeName  *string `json:"episode_name,omitempty"`
	SeasonNumber *int    `json:"season_number,omitempty"`
	EpisodeNumber *int   `json:"episode_number,omitempty"`
	WatchedAt    string  `json:"watched_at"`
	IsCompletion bool    `json:"is_completion"`
}

type ActivityRepository struct {
	db database.DBTX
}

func NewActivityRepository(db database.DBTX) *ActivityRepository {
	return &ActivityRepository{db: db}
}

func (r *ActivityRepository) List(ctx context.Context, limit, offset int) ([]ActivityEvent, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			we.title_id,
			tn.name,
			t.cover_url,
			t.type,
			we.episode_id,
			e.name AS episode_name,
			s.number AS season_number,
			e.number AS episode_number,
			we.watched_at,
			CASE
				WHEN t.status = 'completed'
				  AND we.episode_id IS NOT NULL
				  AND NOT EXISTS (
					SELECT 1 FROM episodes e2
					JOIN seasons s2 ON e2.season_id = s2.id
					WHERE s2.title_id = t.id AND e2.watched = 0
				  )
				THEN 1 ELSE 0
			END AS is_completion
		FROM watch_events we
		JOIN titles t ON t.id = we.title_id
		JOIN title_names tn ON tn.title_id = t.id AND tn.language = 'fr'
		LEFT JOIN episodes e ON e.id = we.episode_id
		LEFT JOIN seasons s ON s.id = e.season_id
		ORDER BY we.watched_at DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("activity: list: %w", err)
	}
	defer rows.Close()

	var events []ActivityEvent
	for rows.Next() {
		var ev ActivityEvent
		var isCompletion int
		if err := rows.Scan(
			&ev.TitleID, &ev.TitleName, &ev.CoverURL, &ev.TitleType,
			&ev.EpisodeID, &ev.EpisodeName, &ev.SeasonNumber, &ev.EpisodeNumber,
			&ev.WatchedAt, &isCompletion,
		); err != nil {
			return nil, fmt.Errorf("activity: scan: %w", err)
		}
		ev.IsCompletion = isCompletion == 1
		events = append(events, ev)
	}
	return events, rows.Err()
}
```

- [ ] **Step 4: Implement `activity.go` handler**

```go
package handler

import (
	"fmt"
	"net/http"

	"github.com/nicolasvasse/plextracker/internal/handler/httputil"
	"github.com/nicolasvasse/plextracker/internal/repository"
)

type ActivityHandler struct {
	activity *repository.ActivityRepository
}

func NewActivityHandler(activity *repository.ActivityRepository) *ActivityHandler {
	return &ActivityHandler{activity: activity}
}

func (h *ActivityHandler) List(w http.ResponseWriter, r *http.Request) error {
	limit := httputil.ParseQueryInt(r, "limit", 50)
	offset := httputil.ParseQueryInt(r, "offset", 0)
	if limit > 100 {
		limit = 100
	}
	events, err := h.activity.List(r.Context(), limit, offset)
	if err != nil {
		return fmt.Errorf("activity: list: %w", err)
	}
	return httputil.WriteJSON(w, events)
}
```

- [ ] **Step 5: Register route in `cmd/serve.go`**

```go
activityHandler := handler.NewActivityHandler(repository.NewActivityRepository(db))
r.With(authMiddleware).Get("/api/stats/activity", httputil.WrapHandler(activityHandler.List))
```

- [ ] **Step 6: Run tests — expect PASS**

```bash
make test
```

- [ ] **Step 7: Commit**

```bash
git add internal/repository/activity_repository.go internal/handler/activity.go internal/handler/activity_test.go cmd/serve.go
git commit -m "$(cat <<'EOF'
feat(handler): ajoute GET /api/stats/activity — flux d'activité paginé
EOF
)"
```

---

### Task 3: Per-Title Watch History Endpoint

**Files:**
- Create: `internal/handler/history.go`
- Create: `internal/handler/history_test.go`
- Modify: `cmd/serve.go`

- [ ] **Step 1: Write the failing test**

```go
func TestHistoryHandler_Get(t *testing.T) {
	db := database.OpenTestDB(t)
	db.Exec(`INSERT INTO titles (id, type, runtime) VALUES (1, 'series', 45)`)
	// Two watch events for the same episode (rewatch)
	db.Exec(`INSERT INTO watch_events (title_id, episode_id, watched_at) VALUES (1, 10, '2026-04-01 21:00:00')`)
	db.Exec(`INSERT INTO watch_events (title_id, episode_id, watched_at) VALUES (1, 10, '2026-04-10 22:00:00')`)
	db.Exec(`INSERT INTO watch_events (title_id, episode_id, watched_at) VALUES (1, 11, '2026-04-11 20:00:00')`)

	h := handler.NewHistoryHandler(repository.NewHistoryRepository(db))
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/titles/1/history", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	err := h.Get(w, r)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, w.Code)

	var result []map[string]any
	json.NewDecoder(w.Body).Decode(&result)
	// Episode 10 watched twice, episode 11 once → 2 entries
	assert.Len(t, result, 2)
	// Most recent first
	assert.Equal(t, float64(11), result[0]["episode_id"])
}
```

- [ ] **Step 2: Run test — expect FAIL**

```bash
make test
```

- [ ] **Step 3: Add `HistoryRepository` to `internal/repository/history_repository.go`**

```go
package repository

import (
	"context"
	"fmt"

	"github.com/nicolasvasse/plextracker/internal/database"
)

type EpisodeHistory struct {
	EpisodeID     *int64  `json:"episode_id,omitempty"`
	EpisodeName   *string `json:"episode_name,omitempty"`
	SeasonNumber  *int    `json:"season_number,omitempty"`
	EpisodeNumber *int    `json:"episode_number,omitempty"`
	WatchCount    int     `json:"watch_count"`
	LastWatchedAt string  `json:"last_watched_at"`
	AllWatches    []string `json:"watches"` // all watched_at timestamps for this episode
}

type HistoryRepository struct {
	db database.DBTX
}

func NewHistoryRepository(db database.DBTX) *HistoryRepository {
	return &HistoryRepository{db: db}
}

func (r *HistoryRepository) GetByTitleID(ctx context.Context, titleID int64) ([]EpisodeHistory, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			we.episode_id,
			e.name,
			s.number AS season_number,
			e.number AS episode_number,
			COUNT(*) AS watch_count,
			MAX(we.watched_at) AS last_watched_at,
			GROUP_CONCAT(we.watched_at, '|') AS all_watches
		FROM watch_events we
		LEFT JOIN episodes e ON e.id = we.episode_id
		LEFT JOIN seasons s ON s.id = e.season_id
		WHERE we.title_id = ?
		GROUP BY we.episode_id
		ORDER BY last_watched_at DESC
	`, titleID)
	if err != nil {
		return nil, fmt.Errorf("history: get: %w", err)
	}
	defer rows.Close()

	var results []EpisodeHistory
	for rows.Next() {
		var h EpisodeHistory
		var allWatches string
		if err := rows.Scan(
			&h.EpisodeID, &h.EpisodeName, &h.SeasonNumber, &h.EpisodeNumber,
			&h.WatchCount, &h.LastWatchedAt, &allWatches,
		); err != nil {
			return nil, fmt.Errorf("history: scan: %w", err)
		}
		if allWatches != "" {
			h.AllWatches = strings.Split(allWatches, "|")
		}
		results = append(results, h)
	}
	return results, rows.Err()
}
```

- [ ] **Step 4: Implement `history.go` handler**

```go
package handler

import (
	"fmt"
	"net/http"

	"github.com/nicolasvasse/plextracker/internal/handler/httputil"
	"github.com/nicolasvasse/plextracker/internal/repository"
)

type HistoryHandler struct {
	history *repository.HistoryRepository
}

func NewHistoryHandler(history *repository.HistoryRepository) *HistoryHandler {
	return &HistoryHandler{history: history}
}

func (h *HistoryHandler) Get(w http.ResponseWriter, r *http.Request) error {
	id, err := httputil.ParseIDParam(r, "id")
	if err != nil {
		return err
	}
	history, err := h.history.GetByTitleID(r.Context(), id)
	if err != nil {
		return fmt.Errorf("history: get: %w", err)
	}
	return httputil.WriteJSON(w, history)
}
```

- [ ] **Step 5: Register route in `cmd/serve.go`**

```go
historyHandler := handler.NewHistoryHandler(repository.NewHistoryRepository(db))
r.With(authMiddleware).Get("/api/titles/{id}/history", httputil.WrapHandler(historyHandler.Get))
```

> Note: This route must be registered AFTER `/api/titles/{id}` for chi to pick the correct handler, or in a separate sub-router for `/api/titles/{id}`. Follow the existing pattern for episode/season routes under a title.

- [ ] **Step 6: Run tests — expect PASS**

```bash
make test
```

- [ ] **Step 7: Commit**

```bash
git add internal/repository/history_repository.go internal/handler/history.go internal/handler/history_test.go cmd/serve.go
git commit -m "$(cat <<'EOF'
feat(handler): ajoute GET /api/titles/{id}/history — historique par titre
EOF
)"
```

---

### Task 4: Frontend — Stats.tsx New Sections

**Files:**
- Modify: `frontend/src/types.ts`
- Modify: `frontend/src/pages/Stats.tsx`
- Modify: `frontend/src/pages/Stats.module.css`

- [ ] **Step 1: Extend `StatsResponse` in `types.ts`**

```ts
export interface StatsResponse {
  // ... existing fields ...
  overview: {
    // ... existing fields ...
    total_watch_minutes: number
  }
  genres: Array<{ genre: string; count: number }>
  streaks: {
    current: number
    best: number
  }
}
```

- [ ] **Step 2: Add watchtime card to `OverviewSection`**

In `OverviewSection`, add to the `cards` array:
```tsx
{
  value: formatWatchtime(overview.total_watch_minutes) ?? '—',
  label: 'TEMPS REGARDÉ'
}
```

Import `formatWatchtime` from `../utils` (defined in the watchtime plan).

- [ ] **Step 3: Add `GenreSection` component**

```tsx
function GenreSection({ genres }: { genres: StatsResponse['genres'] }) {
  if (genres.length === 0) return null
  const max = Math.max(...genres.map(g => g.count), 1)
  return (
    <section className={s.section}>
      <SectionLabel>Top genres</SectionLabel>
      <div className={s.genreBars}>
        {genres.map(g => (
          <div key={g.genre} className={s.genreBarRow}>
            <span className={s.genreBarName}>{g.genre}</span>
            <div className={s.genreBarTrack}>
              <div
                className={s.genreBarFill}
                style={{ width: `${(g.count / max) * 100}%` }}
              />
            </div>
            <span className={s.genreBarCount}>{g.count}</span>
          </div>
        ))}
      </div>
    </section>
  )
}
```

- [ ] **Step 4: Add `StreakSection` component**

```tsx
function StreakSection({ streaks }: { streaks: StatsResponse['streaks'] }) {
  if (streaks.current === 0 && streaks.best === 0) return null
  return (
    <section className={s.section}>
      <div className={s.streakRow}>
        <div className={s.streakCard}>
          <div className={s.streakValue}>🔥 {streaks.current}j</div>
          <div className={s.streakLabel}>Série en cours</div>
        </div>
        <div className={s.streakCard}>
          <div className={s.streakValue}>🏆 {streaks.best}j</div>
          <div className={s.streakLabel}>Meilleure série</div>
        </div>
      </div>
    </section>
  )
}
```

- [ ] **Step 5: Wire new sections into `Stats` component**

```tsx
export function Stats({ path }: { path?: string }) {
  const { data, loading } = useApi<StatsResponse>('/stats')
  if (loading || !data) return <div className={s.loading}>Chargement...</div>

  return (
    <div className={s.page}>
      <h1 className={s.pageTitle}>Stats</h1>
      <OverviewSection overview={data.overview} />
      <GenreSection genres={data.genres ?? []} />
      <RatingsSection ratings={data.ratings} />
      <BreakdownSection breakdown={data.breakdown} />
      <StreakSection streaks={data.streaks ?? { current: 0, best: 0 }} />
      {data.fun_stats.length > 0 && <FunStatsSection stats={data.fun_stats} />}
      <YearSection year={data.year_summary} />
      <ActivitySection />
    </div>
  )
}
```

- [ ] **Step 6: Add CSS for new components in `Stats.module.css`**

```css
.genreBars { display: flex; flex-direction: column; gap: 7px; }
.genreBarRow { display: flex; align-items: center; gap: 8px; }
.genreBarName { font-size: 11px; color: var(--text-muted); width: 90px; flex: 0 0 90px; text-align: right; }
.genreBarTrack { flex: 1; height: 14px; background: var(--surface-raised); border-radius: 4px; overflow: hidden; }
.genreBarFill { height: 100%; background: var(--accent-lavender); border-radius: 4px; }
.genreBarCount { font-size: 11px; color: var(--text-muted); width: 32px; text-align: right; }

.streakRow { display: flex; gap: 8px; }
.streakCard { flex: 1; background: var(--surface-raised); border-radius: 8px; padding: 10px 12px; }
.streakValue { font-size: 18px; font-weight: 700; color: var(--accent-amber); }
.streakLabel { font-size: 10px; color: var(--text-muted); margin-top: 2px; }
```

- [ ] **Step 7: Build frontend**

```bash
make test-front
```

- [ ] **Step 8: Commit**

```bash
git add frontend/src/types.ts frontend/src/pages/Stats.tsx frontend/src/pages/Stats.module.css frontend/src/utils.ts
git commit -m "$(cat <<'EOF'
feat(frontend): ajoute watchtime, genres et séries dans la page Stats
EOF
)"
```

---

### Task 5: Frontend — Activity Feed Section in Stats

**Files:**
- Modify: `frontend/src/pages/Stats.tsx`
- Modify: `frontend/src/pages/Stats.module.css`

- [ ] **Step 1: Add `ActivitySection` component to `Stats.tsx`**

```tsx
function ActivitySection() {
  const [events, setEvents] = useState<ActivityEvent[]>([])
  const [offset, setOffset] = useState(0)
  const [loading, setLoading] = useState(false)
  const [hasMore, setHasMore] = useState(true)
  const LIMIT = 50

  async function loadMore() {
    setLoading(true)
    const data = await apiFetch<ActivityEvent[]>(
      `/api/stats/activity?limit=${LIMIT}&offset=${offset}`
    )
    setEvents(prev => [...prev, ...data])
    setOffset(o => o + LIMIT)
    setHasMore(data.length === LIMIT)
    setLoading(false)
  }

  useEffect(() => { loadMore() }, [])

  // Group events by calendar date
  const grouped = groupByDate(events)

  return (
    <section className={s.sectionLast}>
      <SectionLabel>Activité récente</SectionLabel>
      {Object.entries(grouped).map(([date, evts]) => (
        <div key={date}>
          <div className={s.activityDateHeader}>{formatDateHeader(date)}</div>
          {evts.map((ev, i) => (
            <Link key={i} href={`/titles/${ev.title_id}`} className={s.activityRow}>
              {ev.cover_url
                ? <img className={s.activityThumb} src={ev.cover_url} alt="" role="presentation" />
                : <div className={s.activityThumbPlaceholder} />}
              <div className={s.activityInfo}>
                <span className={s.activityTitle}>{ev.title_name}</span>
                <span className={s.activitySub}>
                  {ev.episode_name
                    ? `S${ev.season_number} E${ev.episode_number} — ${ev.episode_name}`
                    : 'Film'}
                </span>
              </div>
              <span className={`${s.activityBadge} ${s[`badge_${ev.is_completion ? 'done' : ev.title_type}`]}`}>
                {ev.is_completion ? 'Terminé' : ev.episode_name ? 'Épisode' : 'Film'}
              </span>
            </Link>
          ))}
        </div>
      ))}
      {hasMore && (
        <button className={s.loadMoreBtn} onClick={loadMore} disabled={loading}>
          {loading ? 'Chargement…' : 'Voir plus'}
        </button>
      )}
    </section>
  )
}

function groupByDate(events: ActivityEvent[]): Record<string, ActivityEvent[]> {
  return events.reduce((acc, ev) => {
    const date = ev.watched_at.split('T')[0].split(' ')[0]
    if (!acc[date]) acc[date] = []
    acc[date].push(ev)
    return acc
  }, {} as Record<string, ActivityEvent[]>)
}

function formatDateHeader(dateStr: string): string {
  const today = new Date().toISOString().split('T')[0]
  const yesterday = new Date(Date.now() - 86_400_000).toISOString().split('T')[0]
  if (dateStr === today) return "Aujourd'hui"
  if (dateStr === yesterday) return 'Hier'
  return new Date(dateStr).toLocaleDateString('fr-FR', { day: 'numeric', month: 'long' })
}
```

- [ ] **Step 2: Add `ActivityEvent` type to `types.ts`**

```ts
export interface ActivityEvent {
  title_id: number
  title_name: string
  cover_url: string | null
  title_type: string
  episode_id: number | null
  episode_name: string | null
  season_number: number | null
  episode_number: number | null
  watched_at: string
  is_completion: boolean
}
```

- [ ] **Step 3: Add CSS in `Stats.module.css`**

```css
.activityDateHeader {
  padding: 6px 0 4px;
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: var(--text-muted);
  border-bottom: 1px solid var(--border);
  margin-bottom: 2px;
}

.activityRow {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 0;
  border-bottom: 1px solid var(--border-subtle);
  text-decoration: none;
  color: inherit;
}

.activityThumb {
  width: 36px;
  height: 52px;
  border-radius: 5px;
  object-fit: cover;
  flex: 0 0 36px;
}

.activityThumbPlaceholder {
  width: 36px;
  height: 52px;
  border-radius: 5px;
  background: var(--surface-raised);
  flex: 0 0 36px;
}

.activityInfo { flex: 1; min-width: 0; }
.activityTitle { display: block; font-size: 13px; font-weight: 600; color: var(--text-primary); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.activitySub { display: block; font-size: 11px; color: var(--text-muted); margin-top: 2px; }
.activityBadge { font-size: 10px; font-weight: 600; padding: 2px 6px; border-radius: 4px; white-space: nowrap; }
.badge_series { background: color-mix(in srgb, var(--accent-teal) 20%, transparent); color: var(--accent-teal); }
.badge_movie  { background: color-mix(in srgb, var(--accent-lavender) 20%, transparent); color: var(--accent-lavender); }
.badge_done   { background: color-mix(in srgb, var(--accent-green) 20%, transparent); color: var(--accent-green); }
.badge_anime  { background: color-mix(in srgb, var(--accent-lavender) 20%, transparent); color: var(--accent-lavender); }

.loadMoreBtn {
  width: 100%;
  padding: 10px;
  background: var(--surface-raised);
  border: none;
  border-radius: 8px;
  color: var(--text-muted);
  font-size: 13px;
  cursor: pointer;
  margin-top: 10px;
}
```

- [ ] **Step 4: Build frontend**

```bash
make test-front
```

- [ ] **Step 5: Commit**

```bash
git add frontend/src/pages/Stats.tsx frontend/src/pages/Stats.module.css frontend/src/types.ts
git commit -m "$(cat <<'EOF'
feat(frontend): ajoute le flux d'activité récente dans la page Stats
EOF
)"
```

---

### Task 6: Frontend — Per-Title Watch History

**Files:**
- Create: `frontend/src/components/TitleHistory.tsx`
- Create: `frontend/src/components/TitleHistory.module.css`
- Modify: `frontend/src/pages/TitleDetail.tsx`

- [ ] **Step 1: Add `EpisodeHistory` type to `types.ts`**

```ts
export interface EpisodeHistory {
  episode_id: number | null
  episode_name: string | null
  season_number: number | null
  episode_number: number | null
  watch_count: number
  last_watched_at: string
  watches: string[]
}
```

- [ ] **Step 2: Create `TitleHistory.tsx`**

```tsx
import { useApi } from '../hooks/useApi'
import type { EpisodeHistory } from '../types'
import s from './TitleHistory.module.css'

interface Props {
  titleId: number
  onClose: () => void
}

export function TitleHistory({ titleId, onClose }: Props) {
  const { data, loading } = useApi<EpisodeHistory[]>(`/api/titles/${titleId}/history`)

  return (
    <div className={s.container}>
      <div className={s.header}>
        <button className={s.backBtn} onClick={onClose} aria-label="Retour">←</button>
        <span className={s.title}>Historique</span>
      </div>
      {loading && <div className={s.loading}>Chargement…</div>}
      {data?.map((ep, i) => (
        <div key={i} className={s.row}>
          <div className={s.info}>
            <span className={s.epLabel}>
              {ep.episode_number != null
                ? `S${ep.season_number} E${ep.episode_number}${ep.episode_name ? ` — ${ep.episode_name}` : ''}`
                : 'Film'}
            </span>
            <span className={s.date}>
              {new Date(ep.last_watched_at).toLocaleDateString('fr-FR', {
                day: 'numeric', month: 'short', year: 'numeric'
              })}
            </span>
          </div>
          {ep.watch_count > 1 && (
            <span className={s.rewatchBadge}>×{ep.watch_count}</span>
          )}
        </div>
      ))}
    </div>
  )
}
```

- [ ] **Step 3: Create `TitleHistory.module.css`**

```css
.container { background: var(--bg); min-height: 100vh; }
.header { display: flex; align-items: center; gap: 10px; padding: 12px 16px; border-bottom: 1px solid var(--border); }
.backBtn { background: none; border: none; font-size: 18px; color: var(--text-muted); cursor: pointer; padding: 0; }
.title { font-size: 15px; font-weight: 700; }
.loading { padding: 20px 16px; color: var(--text-muted); }
.row { display: flex; align-items: center; gap: 10px; padding: 10px 16px; border-bottom: 1px solid var(--border-subtle); }
.info { flex: 1; min-width: 0; }
.epLabel { display: block; font-size: 13px; font-weight: 600; color: var(--text-primary); }
.date { display: block; font-size: 11px; color: var(--text-muted); margin-top: 2px; }
.rewatchBadge { font-size: 10px; font-weight: 700; background: color-mix(in srgb, var(--accent-lavender) 20%, transparent); color: var(--accent-lavender); border-radius: 4px; padding: 2px 6px; }
```

- [ ] **Step 4: Add "Historique" button to `TitleDetail.tsx`**

In `TitleDetail.tsx`, find the action controls area. Add:

```tsx
const [showHistory, setShowHistory] = useState(false)

// In the JSX, near existing action buttons:
<button className={s.historyBtn} onClick={() => setShowHistory(true)}>
  Historique
</button>

{showHistory && (
  <TitleHistory titleId={title.id} onClose={() => setShowHistory(false)} />
)}
```

Import `TitleHistory` from `../components/TitleHistory`.

- [ ] **Step 5: Add `.historyBtn` CSS to `TitleDetail.module.css`**

```css
.historyBtn {
  font-size: 12px;
  font-weight: 600;
  padding: 6px 12px;
  border-radius: 6px;
  background: var(--surface-raised);
  color: var(--text-muted);
  border: none;
  cursor: pointer;
}
```

- [ ] **Step 6: Build frontend**

```bash
make test-front
```

- [ ] **Step 7: Commit**

```bash
git add frontend/src/components/TitleHistory.tsx frontend/src/components/TitleHistory.module.css frontend/src/pages/TitleDetail.tsx frontend/src/pages/TitleDetail.module.css frontend/src/types.ts
git commit -m "$(cat <<'EOF'
feat(frontend): ajoute l'historique de visionnage par titre sur TitleDetail
EOF
)"
```

---

### Task 7: Update patterns.md

- [ ] **Step 1: Add new endpoints, repositories, and components to `docs/patterns.md`**

Under Routes:
```
GET /api/stats/activity?limit=50&offset=0  List         Yes  paginated watch events
GET /api/titles/{id}/history               Get          Yes  per-title watch log
```

Under Repositories: `ActivityRepository (List)`, `HistoryRepository (GetByTitleID)`.
Under Stats API: note new `total_watch_minutes`, `genres`, `streaks` fields in response.

- [ ] **Step 2: Commit**

```bash
git add docs/patterns.md
git commit -m "$(cat <<'EOF'
docs(patterns): documente les nouveaux endpoints, repositories et champs stats
EOF
)"
```
