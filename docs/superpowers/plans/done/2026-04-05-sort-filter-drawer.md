# Sort in Filter Drawer — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a sort selector (5 fields, toggleable direction) to the filter drawer, persisted in localStorage, hidden during search.

**Architecture:** Add `Sort` and `Order` fields to the existing `TitleFilter` struct. The backend validates against an allowlist and builds `ORDER BY` dynamically. The frontend adds sort state to the Zustand store and a new sort chip row in `FilterDrawer`.

**Tech Stack:** Go / SQLite / chi (backend), Preact / Zustand / CSS Modules (frontend)

---

### Task 1: Backend — Add sort fields to TitleFilter and validate in handler

**Files:**
- Modify: `internal/repository/title.go:21-31` (TitleFilter struct)
- Modify: `internal/handler/title.go:22-52` (List handler)

- [ ] **Step 1: Write failing test — handler passes sort/order to repository**

Add to `internal/handler/title_test.go`:

```go
func TestTitleHandler_List_WithSort(t *testing.T) {
	h, titleRepo := setupHandler(t)

	_, _ = titleRepo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2020, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Old Movie", Language: "en", IsPrimary: true}})
	_, _ = titleRepo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "New Movie", Language: "en", IsPrimary: true}})

	req := httptest.NewRequest("GET", "/api/titles?status=completed&sort=year&order=asc", nil)
	rr := httptest.NewRecorder()
	require.NoError(t, h.List(rr, req))

	assert.Equal(t, http.StatusOK, rr.Code)
	var result repository.PaginatedResult
	_ = json.NewDecoder(rr.Body).Decode(&result)
	require.Len(t, result.Titles, 2)
	assert.Equal(t, "Old Movie", result.Titles[0].PrimaryName())
	assert.Equal(t, "New Movie", result.Titles[1].PrimaryName())
}

func TestTitleHandler_List_InvalidSortFallsBack(t *testing.T) {
	h, titleRepo := setupHandler(t)

	_, _ = titleRepo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Test", Language: "en", IsPrimary: true}})

	req := httptest.NewRequest("GET", "/api/titles?status=watching&sort=bobby_tables&order=sideways", nil)
	rr := httptest.NewRecorder()
	require.NoError(t, h.List(rr, req))

	assert.Equal(t, http.StatusOK, rr.Code)
	var result repository.PaginatedResult
	_ = json.NewDecoder(rr.Body).Decode(&result)
	assert.Len(t, result.Titles, 1)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `make test`
Expected: compilation errors — `TitleFilter` has no `Sort`/`Order` fields.

- [ ] **Step 3: Add Sort and Order to TitleFilter**

In `internal/repository/title.go`, add two fields to `TitleFilter` (after line 30):

```go
type TitleFilter struct {
	Status         *model.TitleStatus
	Type           *model.TitleType
	Search         *string
	MatchStatus    *model.MatchStatus
	SeriesStatus   *model.SeriesStatus
	UpToDate       bool
	WatchingBehind bool
	Limit          int
	Offset         int
	Sort           string // column name: updated_at, original_title, year, my_rating, created_at
	Order          string // asc or desc
}
```

- [ ] **Step 4: Parse and validate sort/order in the handler**

In `internal/handler/title.go`, add after filter initialization (after line 26):

```go
// Validate sort field against allowlist
var allowedSorts = map[string]bool{
	"updated_at":     true,
	"original_title": true,
	"year":           true,
	"my_rating":      true,
	"created_at":     true,
}

func (h *TitleHandler) List(w http.ResponseWriter, r *http.Request) error {
	filter := repository.TitleFilter{
		Limit:  httputil.ParseQueryInt(r, "limit", repository.DefaultPageSize),
		Offset: httputil.ParseQueryInt(r, "offset", 0),
	}

	// Sort
	if sortField := r.URL.Query().Get("sort"); allowedSorts[sortField] {
		filter.Sort = sortField
	}
	if order := r.URL.Query().Get("order"); order == "asc" || order == "desc" {
		filter.Order = order
	}

	// ... rest unchanged
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `make test`
Expected: both new tests pass (repository still uses `updated_at DESC` default, but sort values are passed through).

- [ ] **Step 6: Commit**

```bash
git add internal/repository/title.go internal/handler/title.go internal/handler/title_test.go
git commit -m "feat(sort): ajoute les champs Sort/Order au filtre et parsing handler"
```

---

### Task 2: Backend — Dynamic ORDER BY in repository

**Files:**
- Modify: `internal/repository/title.go:232` (List query)

- [ ] **Step 1: Write failing test — sort by year ascending**

Add to `internal/repository/title_test.go`:

```go
func TestTitleRepository_List_SortByYear(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	repo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2020, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Old", Language: "en", IsPrimary: true}})
	repo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "New", Language: "en", IsPrimary: true}})

	result, err := repo.List(repository.TitleFilter{
		Status: ptr(model.TitleStatusCompleted),
		Sort:   "year",
		Order:  "asc",
	})
	require.NoError(t, err)
	require.Len(t, result.Titles, 2)
	assert.Equal(t, "Old", result.Titles[0].PrimaryName())
	assert.Equal(t, "New", result.Titles[1].PrimaryName())
}

func TestTitleRepository_List_SortByRating_NullsLast(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewTitleRepository(db)

	rating := 8
	repo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed, MyRating: &rating}, []model.TitleName{{Name: "Rated", Language: "en", IsPrimary: true}})
	repo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2023, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "Unrated", Language: "en", IsPrimary: true}})

	result, err := repo.List(repository.TitleFilter{
		Status: ptr(model.TitleStatusCompleted),
		Sort:   "my_rating",
		Order:  "desc",
	})
	require.NoError(t, err)
	require.Len(t, result.Titles, 2)
	assert.Equal(t, "Rated", result.Titles[0].PrimaryName())
	assert.Equal(t, "Unrated", result.Titles[1].PrimaryName())
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `make test`
Expected: `TestTitleRepository_List_SortByYear` may fail on order (still hardcoded `updated_at DESC`). `TestTitleRepository_List_SortByRating_NullsLast` likely fails because unrated title may appear first.

- [ ] **Step 3: Implement dynamic ORDER BY**

In `internal/repository/title.go`, replace the hardcoded ORDER BY at line 232 with:

```go
	// Build ORDER BY
	orderBy := "t.updated_at DESC" // default
	if filter.Sort != "" {
		dir := "DESC"
		if filter.Order == "asc" {
			dir = "ASC"
		} else if filter.Order == "desc" {
			dir = "DESC"
		}
		col := "t." + filter.Sort
		// NULLS LAST: for nullable columns (my_rating, year), sort nulls to the end
		switch filter.Sort {
		case "my_rating", "year":
			orderBy = fmt.Sprintf("CASE WHEN %s IS NULL THEN 1 ELSE 0 END, %s %s", col, col, dir)
		default:
			orderBy = fmt.Sprintf("%s %s", col, dir)
		}
	}

	query := `SELECT DISTINCT ` + baseCols + ` FROM titles t` + whereClause + ` ORDER BY ` + orderBy + ` LIMIT ? OFFSET ?`
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `make test`
Expected: all tests pass, including new sort tests.

- [ ] **Step 5: Commit**

```bash
git add internal/repository/title.go internal/repository/title_test.go
git commit -m "feat(sort): ajoute le ORDER BY dynamique au listing des titres"
```

---

### Task 3: Frontend — Add sort state to Zustand store

**Files:**
- Modify: `frontend/src/store.ts`

- [ ] **Step 1: Add sort/order to store state and localStorage persistence**

In `frontend/src/store.ts`, add sort state:

```typescript
import { create } from 'zustand'
import { apiFetch } from './api'
import { Title, PaginatedResponse, StatusCounts } from './types'

const PAGE_SIZE = 50

const SORT_STORAGE_KEY = 'title-sort'

export type SortField = 'updated_at' | 'original_title' | 'year' | 'my_rating' | 'created_at'
export type SortOrder = 'asc' | 'desc'

export interface SortState {
  field: SortField
  order: SortOrder
}

const DEFAULT_SORT: SortState = { field: 'updated_at', order: 'desc' }

function loadSort(): SortState {
  try {
    const raw = localStorage.getItem(SORT_STORAGE_KEY)
    if (raw) {
      const parsed = JSON.parse(raw)
      if (parsed.field && parsed.order) return parsed
    }
  } catch { /* ignore */ }
  return DEFAULT_SORT
}

function saveSort(sort: SortState) {
  localStorage.setItem(SORT_STORAGE_KEY, JSON.stringify(sort))
}

interface TitleState {
  titles: Title[]
  total: number
  hasMore: boolean
  counts: StatusCounts | null
  loading: boolean
  loadingMore: boolean
  error: string | null
  sort: SortState
  filter: {
    status?: string
    type?: string
    search?: string
    match_status?: string
    series_status?: string
  }
  setFilter: (filter: Partial<TitleState['filter']>) => void
  setSort: (sort: SortState) => void
  fetchTitles: () => Promise<void>
  loadMore: () => Promise<void>
  invalidate: () => Promise<void>
}
```

- [ ] **Step 2: Initialize sort from localStorage and add setSort action**

In the `create<TitleState>` call, add:

```typescript
  sort: loadSort(),

  setSort: (sort) => {
    set({ sort })
    saveSort(sort)
    get().fetchTitles()
  },
```

- [ ] **Step 3: Include sort params in fetchTitles and loadMore (skip when search is active)**

In `fetchTitles`, after the existing filter params and before `params.set('limit', ...)`:

```typescript
      if (!f.search) {
        params.set('sort', get().sort.field)
        params.set('order', get().sort.order)
      }
```

Same in `loadMore`, after the filter params and before `params.set('limit', ...)`:

```typescript
      if (!filter.search) {
        params.set('sort', get().sort.field)
        params.set('order', get().sort.order)
      }
```

- [ ] **Step 4: Run frontend build to verify**

Run: `make build`
Expected: no TypeScript errors.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/store.ts
git commit -m "feat(sort): ajoute l'état de tri au store Zustand avec persistance localStorage"
```

---

### Task 4: Frontend — Add sort UI to FilterDrawer

**Files:**
- Modify: `frontend/src/components/FilterDrawer.tsx`
- Modify: `frontend/src/components/FilterDrawer.module.css`
- Modify: `frontend/src/app.tsx`

- [ ] **Step 1: Add sort props to FilterDrawer**

In `frontend/src/components/FilterDrawer.tsx`, add imports and props:

```typescript
import type { SortField, SortOrder, SortState } from '../store'
```

Add to `FilterDrawerProps`:

```typescript
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
}
```

- [ ] **Step 2: Define sort options array**

Add after the `seriesStatusFilters` array:

```typescript
const sortOptions: { field: SortField; label: string; defaultOrder: SortOrder }[] = [
  { field: 'updated_at', label: 'Last updated', defaultOrder: 'desc' },
  { field: 'original_title', label: 'Title', defaultOrder: 'asc' },
  { field: 'year', label: 'Year', defaultOrder: 'desc' },
  { field: 'my_rating', label: 'Rating', defaultOrder: 'desc' },
  { field: 'created_at', label: 'Date added', defaultOrder: 'desc' },
]
```

- [ ] **Step 3: Add sort chip click handler and render sort section**

In the component function, add the handler:

```typescript
  const handleSortClick = (option: typeof sortOptions[number]) => {
    if (sort.field === option.field) {
      onSortChange({ field: sort.field, order: sort.order === 'asc' ? 'desc' : 'asc' })
    } else {
      onSortChange({ field: option.field, order: option.defaultOrder })
    }
  }
```

Add to the drawer content, **before** the Status section (`<div className={clsx(s.filterLabel, s.filterLabelFirst)}>Status</div>`):

```tsx
        {!isSearchActive && (
          <>
            <div className={clsx(s.filterLabel, s.filterLabelFirst)}>Sort</div>
            <div className={s.filterRow}>
              {sortOptions.map((opt) => {
                const active = sort.field === opt.field
                return (
                  <button
                    key={opt.field}
                    className={clsx(s.chip, active && s.chipActive)}
                    style={active ? { background: accentWash(colors.accentTeal), color: colors.accentTeal } : undefined}
                    onClick={() => handleSortClick(opt)}
                  >
                    {opt.label}
                    {active && (
                      <span className={s.sortArrow}>
                        {sort.order === 'asc' ? ' ↑' : ' ↓'}
                      </span>
                    )}
                  </button>
                )
              })}
            </div>
          </>
        )}
```

When sort section is shown, remove `s.filterLabelFirst` from the Status label (since Sort is now first):

```tsx
        <div className={clsx(s.filterLabel, isSearchActive && s.filterLabelFirst)}>Status</div>
```

- [ ] **Step 4: Add sort active tag in collapsed state**

In the `activeTags` building section, add the active sort tag (before the return):

```typescript
  const activeSort = sortOptions.find((o) => o.field === sort.field)
  if (!isSearchActive && activeSort && sort.field !== 'updated_at') {
    activeTags.push({ label: `${activeSort.label} ${sort.order === 'asc' ? '↑' : '↓'}`, color: colors.accentTeal })
  }
```

- [ ] **Step 5: Add CSS for sort arrow**

In `frontend/src/components/FilterDrawer.module.css`, add:

```css
.sortArrow {
  font-size: 8px;
  margin-left: 2px;
}
```

- [ ] **Step 6: Wire sort props in App**

In `frontend/src/app.tsx`, import `SortState` and pass sort props to `FilterDrawer`:

```typescript
import { useTitleStore, SortState } from './store'
```

Add after existing filter hooks:

```typescript
  const { filter, setFilter, sort, setSort } = useTitleStore()
```

Update FilterDrawer usage:

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
                isSearchActive={!!filter.search}
                defaultOpen={currentPath === '/'}
              />
```

- [ ] **Step 7: Run build to verify**

Run: `make build`
Expected: no TypeScript errors, build succeeds.

- [ ] **Step 8: Commit**

```bash
git add frontend/src/components/FilterDrawer.tsx frontend/src/components/FilterDrawer.module.css frontend/src/app.tsx
git commit -m "feat(sort): ajoute les options de tri dans le tiroir de filtres"
```

---

### Task 5: Visual verification

- [ ] **Step 1: Start dev environment**

Run: `make up` then `make dev-frontend`

- [ ] **Step 2: Open app and verify sort UI**

Login with DEBUG_LOGIN credentials from `.env.local`. On the library page:
1. Verify Sort section appears above Status in the filter drawer
2. Tap each sort chip — verify it becomes active with an arrow indicator
3. Tap the active chip — verify the arrow flips direction
4. Verify the list re-fetches and order changes
5. Close and reopen the drawer — verify the collapsed state shows the sort tag (when not "Last updated")

- [ ] **Step 3: Verify search hides sort**

Navigate to search, type a query. Verify:
1. Sort section disappears
2. Clear the search — sort section reappears with previous selection

- [ ] **Step 4: Verify persistence**

Select "Year ↑", then refresh the page. Verify sort is still "Year ↑".

- [ ] **Step 5: Verify null handling**

Sort by Rating desc — titles without a rating should appear at the bottom. Sort by Rating asc — titles without a rating should still appear at the bottom.

- [ ] **Step 6: Fix any issues found, re-verify, commit**

---

### Task 6: Run full test suite & lint

- [ ] **Step 1: Run backend tests**

Run: `make test`
Expected: all pass.

- [ ] **Step 2: Run frontend tests**

Run: `make test-front`
Expected: all pass.

- [ ] **Step 3: Run linters**

Run: `make lint`
Expected: no new issues.

- [ ] **Step 4: Fix any failures, commit**

---

### Task 7: Update docs

- [ ] **Step 1: Update patterns.md**

Add sort query params to the title listing route documentation.

- [ ] **Step 2: Update CHANGELOG.md**

Add entry under current version:
```
- Ajout du tri dans le tiroir de filtres (dernière mise à jour, titre, année, note, date d'ajout)
```

- [ ] **Step 3: Commit**

```bash
git add docs/patterns.md CHANGELOG.md
git commit -m "docs: met à jour patterns et changelog pour le tri"
```
