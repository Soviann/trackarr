# Clickable Cast → Person Titles Page

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make cast/crew names in TitleDetail clickable; clicking navigates to a page listing all titles that person appears in.

**Architecture:** Add `person` filter to the existing `GET /api/titles` endpoint (SQLite `json_each` on the credits JSON column). No schema migration needed. New frontend page `/person/:name` renders a plain title card list.

**Tech Stack:** Go 1.24 / SQLite json_each / chi / Preact 10 / preact-router / CSS Modules

**PO Summary:** Les noms du casting dans la fiche titre deviennent cliquables. Un clic affiche une page listant tous les titres dans lesquels cette personne apparaît. Pas de changement de schéma base de données.

---

## Phase 1 — Backend: person filter [seq]

### Task 1: Add `Person` field to `TitleFilter`

**Files:**
- Modify: `internal/repository/title.go` (struct ~line 49, WHERE block ~line 338)

- [ ] In `TitleFilter` struct, add after the `GenreOp` field:
```go
Person *string // filter by credit name (json_each on credits column)
```

- [ ] In `List()`, after the genres WHERE block, add:
```go
if filter.Person != nil {
    conditions = append(conditions, `t.credits IS NOT NULL AND EXISTS (SELECT 1 FROM json_each(t.credits) je WHERE json_extract(je.value, '$.name') = ?)`)
    args = append(args, *filter.Person)
}
```

### Task 2: Parse `person` query param in handler

**Files:**
- Modify: `internal/handler/title.go` (`List()` function ~line 63)

- [ ] After the genres block, before the final `h.titlesRead.List(filter)` call, add:
```go
if p := r.URL.Query().Get("person"); p != "" {
    filter.Person = &p
}
```

### Task 3: Write and run backend test

**Files:**
- Modify: `internal/repository/title_test.go`

- [ ] Add test (note: `Create()` includes `credits` in the INSERT, so set it directly on the struct):
```go
func TestTitleRepository_List_PersonFilter(t *testing.T) {
    db := setupTestDB(t)
    repo := NewTitleRepository(db)

    credits := `[{"name":"John Doe","role":"Director"}]`
    idA, err := repo.Create(&model.Title{
        Type:        model.TitleTypeMovie,
        Year:        2024,
        Status:      model.TitleStatusWatching,
        MatchStatus: model.MatchStatusConfirmed,
        Credits:     &credits,
    }, []model.TitleName{{Name: "Film A", Language: "en", IsPrimary: true}})
    require.NoError(t, err)

    _, err = repo.Create(&model.Title{
        Type:        model.TitleTypeMovie,
        Year:        2023,
        Status:      model.TitleStatusWatching,
        MatchStatus: model.MatchStatusConfirmed,
    }, []model.TitleName{{Name: "Film B", Language: "en", IsPrimary: true}})
    require.NoError(t, err)

    person := "John Doe"
    result, err := repo.List(TitleFilter{Person: &person})
    require.NoError(t, err)
    require.Len(t, result.Titles, 1)
    assert.Equal(t, idA, result.Titles[0].ID)
}
```

- [ ] Run: `make test` — expected: all pass

- [ ] Commit:
```
git add internal/repository/title.go internal/handler/title.go internal/repository/title_test.go
git commit -m "feat(api): filtre par personne dans le cast sur /api/titles"
```

---

## Phase 2 — Frontend: PersonTitles page + clickable cast [seq]

### Task 4: Create `PersonTitles.tsx` and its CSS module

**Files:**
- Create: `frontend/src/pages/PersonTitles.tsx`
- Create: `frontend/src/pages/PersonTitles.module.css`

- [ ] Create `PersonTitles.tsx`:
```tsx
import { route } from 'preact-router'
import { useApi } from '../hooks/useApi'
import type { PaginatedResponse } from '../types'
import { TitleCard } from '../components/TitleCard'
import { ErrorBanner } from '../components/ErrorBanner'
import s from './PersonTitles.module.css'

export function PersonTitles({ name }: { path?: string; name?: string }) {
  const { data, error, loading, mutate } = useApi<PaginatedResponse>(
    name ? `/titles?person=${encodeURIComponent(name)}&limit=200` : null,
  )

  const titles = data?.titles ?? []

  return (
    <div className={s.page}>
      <div className={s.header}>
        <button type="button" className={s.backBtn} onClick={() => history.back()} aria-label="Back">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <polyline points="15 18 9 12 15 6" />
          </svg>
        </button>
        <div className={s.personName}>{name}</div>
      </div>

      {error && <ErrorBanner message={error} onRetry={mutate} />}

      {loading && <div className={s.centered}>Loading…</div>}

      {!loading && !error && titles.length === 0 && (
        <div className={s.centered}>No titles found.</div>
      )}

      {titles.length > 0 && (
        <div className={s.list}>
          {titles.map((t) => (
            <TitleCard key={t.id} title={t} onUpdate={mutate} />
          ))}
        </div>
      )}
    </div>
  )
}
```

- [ ] Create `PersonTitles.module.css`:
```css
.page {
  padding-bottom: 140px;
}

.header {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 16px var(--space-lg) 10px;
}

.backBtn {
  width: 32px;
  height: 32px;
  background: none;
  border: none;
  padding: 0;
  cursor: pointer;
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.personName {
  font-size: var(--font-size-xl);
  font-weight: 700;
  color: #fff;
  line-height: 1.2;
}

.list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 0 var(--space-lg);
}

.centered {
  padding: 40px var(--space-lg);
  text-align: center;
  color: var(--color-text-secondary);
}
```

### Task 5: Register route in `app.tsx`

**Files:**
- Modify: `frontend/src/app.tsx`

- [ ] Add import alongside existing page imports:
```tsx
import { PersonTitles } from './pages/PersonTitles'
```

- [ ] Add route after `<TitleDetail path="/title/:id" />`:
```tsx
<PersonTitles path="/person/:name" />
```

### Task 6: Make cast names clickable in `TitleDetail.tsx` + CSS

**Files:**
- Modify: `frontend/src/pages/TitleDetail.tsx` (cast block ~line 223)
- Modify: `frontend/src/pages/TitleDetail.module.css` (`.castPerson` ~line 208)

- [ ] In the cast map, replace `<span className={s.castPerson}>{c.name}</span>` with:
```tsx
<button
  type="button"
  className={s.castPerson}
  onClick={() => route('/person/' + encodeURIComponent(c.name))}
>
  {c.name}
</button>
```

(`route` is already imported in TitleDetail.tsx)

- [ ] Replace `.castPerson { color: #fff; }` with:
```css
.castPerson {
  color: #fff;
  background: none;
  border: none;
  padding: 0;
  font-size: inherit;
  font-family: inherit;
  cursor: pointer;
  text-align: left;
  text-decoration: underline;
  text-decoration-color: rgba(255, 255, 255, 0.3);
  text-underline-offset: 2px;
}

.castPerson:hover {
  text-decoration-color: #fff;
}
```

### Task 7: Test, visual verify, and commit

- [ ] Run: `make test-front` — expected: all pass
- [ ] Start dev servers: `make up` + `make dev-frontend`
- [ ] Open a title with credits → click a cast name → verify PersonTitles page shows
- [ ] Check console for errors, verify back button works
- [ ] Commit:
```
git add frontend/src/pages/PersonTitles.tsx frontend/src/pages/PersonTitles.module.css \
        frontend/src/app.tsx frontend/src/pages/TitleDetail.tsx frontend/src/pages/TitleDetail.module.css
git commit -m "feat(ui): rend le casting cliquable et affiche les titres liés à une personne"
```

---

## Verification

1. `make test` → green
2. `make test-front` → green
3. Open any title with credits in browser
4. Click a cast member name → lands on `/person/<name>` listing all matching titles
5. Browser back → returns to title detail
6. Check console: no errors
7. Navigate other pages (Library, Search) → no regressions

## CHANGELOG

Under `## [Unreleased]`:
```
### Added
- Clickable cast/crew names in title detail — click navigates to a page listing all titles featuring that person
```
