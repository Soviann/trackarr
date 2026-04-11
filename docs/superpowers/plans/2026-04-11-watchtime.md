# Watchtime Tracking — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Track and display total watch time per title (including rewatches) via a `total_watch_minutes` column, maintained at the application layer.

**Architecture:** New `total_watch_minutes INTEGER NOT NULL DEFAULT 0` column on `titles`. Incremented/decremented in every code path that creates or removes a watch event (LibraryService, SimklImporter, Plex webhook). Recalculated on rematch using `COUNT(watch_events) × new_runtime`. Displayed on TitleDetail frontend.

**Tech Stack:** Go 1.24, SQLite migrations, testify/assert, Preact 10 / TypeScript

---

## File Map

| Action | File |
|---|---|
| Create | `internal/database/migrations/015_watchtime.up.sql` |
| Create | `internal/database/migrations/015_watchtime.down.sql` |
| Modify | `internal/repository/title.go` — add field to model struct + queries |
| Modify | `internal/repository/title_search.go` — add field to SELECT + scan |
| Modify | `internal/repository/watch_event.go` — add `CountByTitleID` method |
| Modify | `internal/service/library.go` — increment/decrement on watch toggle |
| Modify | `internal/service/simkl.go` — increment per imported watch event |
| Modify | `internal/handler/webhook.go` — increment on Plex scrobble |
| Modify | `internal/handler/title.go` — recalculate on rematch |
| Modify | `frontend/src/types.ts` — add `total_watch_minutes` to Title type |
| Modify | `frontend/src/pages/TitleDetail.tsx` — display formatted watchtime |

---

### Task 1: Migration

**Files:**
- Create: `internal/database/migrations/015_watchtime.up.sql`
- Create: `internal/database/migrations/015_watchtime.down.sql`

- [x] **Step 1: Create up migration**

`internal/database/migrations/015_watchtime.up.sql`:
```sql
ALTER TABLE titles ADD COLUMN total_watch_minutes INTEGER NOT NULL DEFAULT 0;
```

- [x] **Step 2: Create down migration**

`internal/database/migrations/015_watchtime.down.sql`:
```sql
ALTER TABLE titles DROP COLUMN total_watch_minutes;
```

- [x] **Step 3: Run migration to verify it applies**

```bash
make migrate
```

Expected: exits 0, no errors.

- [x] **Step 4: Commit**

```bash
git add internal/database/migrations/015_watchtime.up.sql internal/database/migrations/015_watchtime.down.sql
git commit -m "$(cat <<'EOF'
chore(db): ajoute la colonne total_watch_minutes sur titles (migration 015)
EOF
)"
```

---

### Task 2: Repository — Model & Queries

**Files:**
- Modify: `internal/repository/title.go`
- Modify: `internal/repository/title_search.go`

- [x] **Step 1: Add `TotalWatchMinutes` to the repository Title struct in `title.go`**

In the struct that maps DB rows (around line 78 where `Runtime *int` appears), add:
```go
TotalWatchMinutes int `json:"total_watch_minutes"`
```

- [x] **Step 2: Add field to INSERT in `title.go`**

In the INSERT query (around line 102), add `total_watch_minutes` to the column list and `title.TotalWatchMinutes` to the values list.

- [x] **Step 3: Add field to SELECT scan in `title.go`**

In the SELECT + Scan for `GetByID` (around line 130), add `total_watch_minutes` to the column list and `&title.TotalWatchMinutes` to the Scan call.

- [x] **Step 4: Add field to UPDATE in `title.go`**

Find the UPDATE query for title metadata and add:
```sql
total_watch_minutes = ?,
```
with the corresponding `title.TotalWatchMinutes` argument.

- [x] **Step 5: Add field to `title_search.go` SELECT + scan**

In `baseCols` (line 68 and 352) add `t.total_watch_minutes`. In the corresponding Scan calls (line 125 and 416), add `&t.TotalWatchMinutes`.

- [x] **Step 6: Build to verify no compile errors**

```bash
make build
```

- [x] **Step 7: Commit**

```bash
git add internal/repository/title.go internal/repository/title_search.go
git commit -m "$(cat <<'EOF'
feat(repository): intègre total_watch_minutes dans les requêtes title
EOF
)"
```

---

### Task 3: Repository — Watch Event Count

**Files:**
- Modify: `internal/repository/watch_event.go`

- [x] **Step 1: Write the failing test**

Add to `internal/repository/watch_event_test.go` (or create it):
```go
func TestWatchEventRepo_CountByTitleID(t *testing.T) {
	db := database.OpenTestDB(t)
	repo := repository.NewWatchEventRepository(db)
	titleRepo := repository.NewTitleRepository(db)

	title := &repository.Title{Type: "movie", Runtime: intPtr(120)}
	titleRepo.Create(title)

	// No events yet
	count, err := repo.CountByTitleID(title.ID)
	assert.NoError(t, err)
	assert.Equal(t, 0, count)

	// Add two events
	repo.Create(&repository.WatchEvent{TitleID: title.ID})
	repo.Create(&repository.WatchEvent{TitleID: title.ID})

	count, err = repo.CountByTitleID(title.ID)
	assert.NoError(t, err)
	assert.Equal(t, 2, count)
}
```

- [x] **Step 2: Run test — expect FAIL**

```bash
make test
```

Expected: compile error or FAIL — `CountByTitleID` not yet defined.

- [x] **Step 3: Implement `CountByTitleID`**

In `internal/repository/watch_event.go`, add:
```go
func (r *WatchEventRepository) CountByTitleID(titleID int64) (int, error) {
	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM watch_events WHERE title_id = ?`, titleID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("watch_event: count by title: %w", err)
	}
	return count, nil
}
```

- [x] **Step 4: Run test — expect PASS**

```bash
make test
```

- [x] **Step 5: Commit**

```bash
git add internal/repository/watch_event.go internal/repository/watch_event_test.go
git commit -m "$(cat <<'EOF'
feat(repository): ajoute CountByTitleID sur WatchEventRepository
EOF
)"
```

---

### Task 4: Service Layer — Increment/Decrement on Watch Toggle

**Files:**
- Modify: `internal/service/library.go`

- [x] **Step 1: Locate `MarkWatched`, `ToggleWatched`, and `BatchMarkWatched` in `library.go`**

Each path that calls the equivalent of "create a watch event" or "delete a watch event" needs to also update `total_watch_minutes` on the title. The runtime to use is `title.Runtime` (dereference safely; if nil, treat as 0).

- [x] **Step 2: Add helper to safely get runtime**

At the top of the file or in a local helper:
```go
func safeRuntime(r *int) int {
	if r == nil {
		return 0
	}
	return *r
}
```

- [x] **Step 3: In the "mark as watched" path, increment total_watch_minutes**

After creating the watch event and before persisting the title, add:
```go
title.TotalWatchMinutes += safeRuntime(title.Runtime)
```
Then persist via the existing `titles.Update(title)` call.

- [x] **Step 4: In the "mark as unwatched" path, decrement total_watch_minutes**

```go
title.TotalWatchMinutes -= safeRuntime(title.Runtime)
if title.TotalWatchMinutes < 0 {
	title.TotalWatchMinutes = 0
}
```

- [x] **Step 5: In `BatchMarkWatched`, apply the same increment per episode marked**

Multiply: `title.TotalWatchMinutes += safeRuntime(title.Runtime) * countNewlyWatched` where `countNewlyWatched` is the number of previously-unwatched episodes being marked.

- [x] **Step 6: Write the test**

Add to `internal/service/library_test.go`:
```go
func TestLibraryService_WatchtimeUpdates(t *testing.T) {
	db := database.OpenTestDB(t)
	svc := setupLibraryService(t, db)

	// Create a movie with runtime=120
	title := createTestTitle(t, db, "movie", 120)

	// Mark watched — total_watch_minutes should become 120
	err := svc.MarkWatched(context.Background(), title.ID)
	assert.NoError(t, err)
	updated, _ := titleRepo(db).GetByID(title.ID)
	assert.Equal(t, 120, updated.TotalWatchMinutes)

	// Mark watched again (rewatch) — should become 240
	err = svc.MarkWatched(context.Background(), title.ID)
	assert.NoError(t, err)
	updated, _ = titleRepo(db).GetByID(title.ID)
	assert.Equal(t, 240, updated.TotalWatchMinutes)
}
```

- [x] **Step 7: Run tests — expect PASS**

```bash
make test
```

- [x] **Step 8: Commit**

```bash
git add internal/service/library.go internal/service/library_test.go
git commit -m "$(cat <<'EOF'
feat(library): met à jour total_watch_minutes lors des toggles de visionnage
EOF
)"
```

---

### Task 5: Simkl Import & Plex Webhook

**Files:**
- Modify: `internal/service/simkl.go`
- Modify: `internal/handler/webhook.go`

- [x] **Step 1: In `simkl.go`, after creating each watch event, add runtime to title**

In the import loop, after persisting a watch event:
```go
title.TotalWatchMinutes += safeRuntime(title.Runtime)
```
Apply the same `safeRuntime` helper (move it to a shared location if needed, or duplicate — 2 occurrences is acceptable).

- [x] **Step 2: In `webhook.go`, after recording a Plex scrobble watch event, increment**

In the scrobble handler, after the watch event is persisted:
```go
title.TotalWatchMinutes += safeRuntime(title.Runtime)
// persist title update
```

- [x] **Step 3: Build**

```bash
make build
```

- [x] **Step 4: Commit**

```bash
git add internal/service/simkl.go internal/handler/webhook.go
git commit -m "$(cat <<'EOF'
feat(import): comptabilise le temps de visionnage lors des imports Simkl et webhooks Plex
EOF
)"
```

---

### Task 6: Recalculate on Rematch

**Files:**
- Modify: `internal/handler/title.go` (rematch handler)

- [x] **Step 1: Locate the rematch handler in `title.go`**

Find the `Rematch` handler method. After the new runtime is available from enrichment, add a recalculation step.

- [x] **Step 2: Add recalculation after enrichment completes**

```go
// After title is enriched and new Runtime is set:
count, err := h.watchEventRepo.CountByTitleID(title.ID)
if err != nil {
	slog.WarnContext(r.Context(), "rematch: count watch events", "title_id", title.ID, "err", err)
}
title.TotalWatchMinutes = count * safeRuntime(title.Runtime)
```

> Note: `safeRuntime` must be accessible from this package. If it was defined in `service/library.go`, either move it to a shared `internal/util` package or inline it here:
> ```go
> func safeRuntime(r *int) int { if r == nil { return 0 }; return *r }
> ```

- [x] **Step 3: Write the test**

```go
func TestTitleHandler_Rematch_RecalculatesWatchtime(t *testing.T) {
	// Setup: title with runtime=60, 3 watch events → total=180
	// Trigger rematch with new runtime=90
	// Assert: total_watch_minutes = 3 * 90 = 270
}
```

- [x] **Step 4: Run tests — expect PASS**

```bash
make test
```

- [x] **Step 5: Commit**

```bash
git add internal/handler/title.go
git commit -m "$(cat <<'EOF'
feat(rematch): recalcule total_watch_minutes après un changement de runtime
EOF
)"
```

---

### Task 7: Frontend — Display on TitleDetail

**Files:**
- Modify: `frontend/src/types.ts`
- Modify: `frontend/src/pages/TitleDetail.tsx`

- [x] **Step 1: Add `total_watch_minutes` to the Title type in `types.ts`**

Find the `Title` interface and add:
```ts
total_watch_minutes: number
```

- [x] **Step 2: Add a format helper in `frontend/src/utils.ts`**

```ts
export function formatWatchtime(minutes: number): string | null {
  if (minutes <= 0) return null
  const h = Math.floor(minutes / 60)
  const m = minutes % 60
  if (h === 0) return `${m}m`
  if (m === 0) return `${h}h`
  return `${h}h ${m}m`
}
```

- [x] **Step 3: Display on TitleDetail**

In `TitleDetail.tsx`, find where status, rating, and progress are displayed. Add the watchtime line:

```tsx
import { formatWatchtime } from '../utils'

// Inside the render, near the status/progress area:
{title.total_watch_minutes > 0 && (
  <span className={s.watchtime}>
    ~{formatWatchtime(title.total_watch_minutes)} watched
  </span>
)}
```

- [x] **Step 4: Add the CSS class in `TitleDetail.module.css`**

```css
.watchtime {
  font-size: 12px;
  color: var(--text-dimmed);
  margin-top: 4px;
}
```

- [x] **Step 5: Run frontend tests**

```bash
make test-front
```

- [x] **Step 6: Commit**

```bash
git add frontend/src/types.ts frontend/src/pages/TitleDetail.tsx frontend/src/pages/TitleDetail.module.css frontend/src/utils.ts
git commit -m "$(cat <<'EOF'
feat(frontend): affiche le temps de visionnage total sur la fiche titre
EOF
)"
```

---

### Task 8: Update patterns.md

**Files:**
- Modify: `docs/patterns.md`

- [x] **Step 1: Add `total_watch_minutes` to the models section and `CountByTitleID` to repositories**

Under the Models section, note `total_watch_minutes` on Title.
Under Repositories, note `CountByTitleID(titleID) (int, error)` on WatchEventRepository.

- [x] **Step 2: Commit**

```bash
git add docs/patterns.md
git commit -m "$(cat <<'EOF'
docs(patterns): documente total_watch_minutes et CountByTitleID
EOF
)"
```
