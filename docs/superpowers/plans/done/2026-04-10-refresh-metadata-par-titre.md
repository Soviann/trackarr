# Rafraîchir les métadonnées d'un titre individuel

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for checktracking.

**Goal:** Ajouter un bouton "Refresh metadata" dans l'ActionDrawer d'un titre pour déclencher le rafraîchissement de ses métadonnées (couverture, épisodes, statut) sans avoir à lancer le refresh global.

**Architecture:** Nouveau endpoint `POST /titles/{id}/refresh` sur `TitleHandler` (qui reçoit un nouveau champ `bgSvc`). Le service `BackgroundService` expose une méthode publique `RefreshByID`. Côté frontend, l'`ActionDrawer` reçoit une prop `onRefresh` et affiche un nouveau bouton dans la section "Manage".

**Tech Stack:** Go 1.24 / chi / Preact 10 / TypeScript

---

## Fichiers modifiés

| Fichier | Rôle |
|---|---|
| `internal/service/background.go` | Expose `RefreshByID(ctx, titleID)` |
| `internal/handler/title.go` | Ajoute champ `bgSvc`, méthode `RefreshOne` |
| `internal/router/router.go` | Passe `bgSvc` à `NewTitleHandler`, enregistre la route |
| `frontend/src/components/ActionDrawer.tsx` | Prop `onRefresh`, bouton dans "Manage" |
| `frontend/src/pages/TitleDetail.tsx` | Handler `handleRefresh`, passage de la prop |

---

### Task 1 : Exposer `RefreshByID` dans le service background

**Files:**
- Modify: `internal/service/background.go`

- [ ] **Step 1 : Ajouter la méthode publique**

Dans `internal/service/background.go`, après la méthode `RefreshAllTitles` (ligne 80), ajouter :

```go
// RefreshByID refreshes metadata for a single title.
func (s *BackgroundService) RefreshByID(ctx context.Context, titleID int64) error {
	title, err := s.titles.GetByID(titleID)
	if err != nil {
		return fmt.Errorf("background: get title %d: %w", titleID, err)
	}
	s.refreshTitle(ctx, title)
	return nil
}
```

- [ ] **Step 2 : Vérifier la compilation**

```bash
make build
```

Expected: compilation sans erreur.

- [ ] **Step 3 : Commit**

```bash
git add internal/service/background.go
git commit -m "feat(background): expose RefreshByID pour rafraîchir un titre"
```

---

### Task 2 : Handler `RefreshOne` dans `TitleHandler`

**Files:**
- Modify: `internal/handler/title.go`

- [ ] **Step 1 : Ajouter `bgSvc` au struct et au constructeur**

Dans `internal/handler/title.go`, modifier le struct (ligne 17) et le constructeur (ligne 29) :

```go
type TitleHandler struct {
    db         *sql.DB
    titles     *repository.TitleRepository
    titlesRead *repository.TitleRepository
    seasons    *repository.SeasonRepository
    episodes   *repository.EpisodeRepository
    events     *repository.WatchEventRepository
    tasks      *repository.TaskRepository
    pipeline   *matching.Pipeline
    service    *service.TitleService
    bgSvc      *service.BackgroundService
}

func NewTitleHandler(db *sql.DB, titles *repository.TitleRepository, titlesRead *repository.TitleRepository, seasons *repository.SeasonRepository, episodes *repository.EpisodeRepository, events *repository.WatchEventRepository, tasks *repository.TaskRepository, pipeline *matching.Pipeline, svc *service.TitleService, bgSvc *service.BackgroundService) *TitleHandler {
    return &TitleHandler{
        db:         db,
        titles:     titles,
        titlesRead: titlesRead,
        seasons:    seasons,
        episodes:   episodes,
        events:     events,
        tasks:      tasks,
        pipeline:   pipeline,
        service:    svc,
        bgSvc:      bgSvc,
    }
}
```

- [ ] **Step 2 : Ajouter le handler `RefreshOne`**

À la fin de `internal/handler/title.go`, ajouter :

```go
// RefreshOne triggers a metadata refresh for a single title.
func (h *TitleHandler) RefreshOne(w http.ResponseWriter, r *http.Request) error {
    id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
    if err != nil {
        return httputil.BadRequest("invalid title id")
    }

    if h.bgSvc == nil {
        return httputil.InternalError("refresh title", fmt.Errorf("background service not available"))
    }

    go func() {
        if err := h.bgSvc.RefreshByID(context.Background(), id); err != nil {
            log.Printf("refresh title %d: %v", id, err)
        }
    }()

    w.WriteHeader(http.StatusAccepted)
    return nil
}
```

Ajouter `"context"` et `"log"` aux imports si absents.

- [ ] **Step 3 : Vérifier la compilation**

```bash
make build
```

Expected: compilation sans erreur.

- [ ] **Step 4 : Commit**

```bash
git add internal/handler/title.go
git commit -m "feat(handler): ajoute RefreshOne pour rafraîchir les métadonnées d'un titre"
```

---

### Task 3 : Route et câblage dans le router

**Files:**
- Modify: `internal/router/router.go`

- [ ] **Step 1 : Passer `bgSvc` à `NewTitleHandler`**

Dans `internal/router/router.go`, ligne 63 :

```go
titles := handler.NewTitleHandler(writeDB, titleRepo, titleReadRepo, seasonRepo, episodeRepo, eventRepo, taskRepo, pipeline, titleSvc, bgSvc)
```

- [ ] **Step 2 : Enregistrer la route**

Dans `internal/router/router.go`, après la ligne `r.Post("/titles/{id}/merge", ...)` (ligne 110) :

```go
r.Post("/titles/{id}/refresh", httputil.WrapHandler(titles.RefreshOne))
```

- [ ] **Step 3 : Vérifier la compilation**

```bash
make build
```

Expected: compilation sans erreur.

- [ ] **Step 4 : Mettre à jour `title_test.go` pour le nouveau paramètre**

Dans `internal/handler/title_test.go`, ligne 36, ajouter `nil` pour `bgSvc` :

```go
h := handler.NewTitleHandler(db, titleRepo, titleRepo, seasonRepo, episodeRepo, eventRepo, taskRepo, nil, titleSvc, nil)
```

- [ ] **Step 5 : Lancer les tests**

```bash
make test
```

Expected: tous les tests passent.

- [ ] **Step 6 : Commit**

```bash
git add internal/router/router.go internal/handler/title_test.go
git commit -m "feat(router): enregistre POST /titles/{id}/refresh"
```

---

### Task 4 : Bouton dans l'ActionDrawer

**Files:**
- Modify: `frontend/src/components/ActionDrawer.tsx`

- [ ] **Step 1 : Ajouter la prop `onRefresh`**

Dans `ActionDrawer.tsx`, modifier l'interface (lignes 6-15) et la destructuration (ligne 17-20) :

```tsx
interface ActionDrawerProps {
  title: Title
  nextEpisode: Episode | null
  nextSeasonNumber?: number
  onMarkNext?: () => void
  onRate: () => void
  onEdit: () => void
  onRematch: () => void
  onMerge: () => void
  onRefresh: () => void
}

export function ActionDrawer({
  title, nextEpisode, nextSeasonNumber,
  onMarkNext, onRate, onEdit, onRematch, onMerge, onRefresh,
}: ActionDrawerProps) {
```

- [ ] **Step 2 : Ajouter le bouton dans la section "Manage"**

Dans la section Manage (après le bouton "🔗 Merge into...", ligne 111) :

```tsx
<button onClick={onRefresh} className={s.manage}>
  ↻ Refresh
</button>
```

- [ ] **Step 3 : Vérifier la compilation TypeScript**

```bash
make test-front
```

Expected: aucune erreur TypeScript.

- [ ] **Step 4 : Commit**

```bash
git add frontend/src/components/ActionDrawer.tsx
git commit -m "feat(ui): ajoute le bouton Refresh dans l'ActionDrawer"
```

---

### Task 5 : Handler dans TitleDetail

**Files:**
- Modify: `frontend/src/pages/TitleDetail.tsx`

- [ ] **Step 1 : Ajouter `handleRefresh`**

Dans `TitleDetail.tsx`, après les handlers existants (vers ligne 83 où se trouvent `handleMarkNext` etc.), ajouter :

```tsx
const handleRefresh = async () => {
  await apiFetch(`/titles/${title.id}/refresh`, { method: 'POST' })
}
```

- [ ] **Step 2 : Passer la prop à `ActionDrawer`**

Dans le JSX (ligne 288), ajouter `onRefresh={handleRefresh}` :

```tsx
<ActionDrawer
  title={title}
  nextEpisode={next?.episode ?? null}
  nextSeasonNumber={next?.season.season_number}
  onMarkNext={handleMarkNext}
  onRate={() => setShowRating(true)}
  onEdit={() => setShowEdit(true)}
  onRematch={() => setShowRematch(true)}
  onMerge={() => route(`/search?mergeSourceId=${title.id}&mergeSourceName=${encodeURIComponent(name)}`)}
  onRefresh={handleRefresh}
/>
```

- [ ] **Step 3 : Vérifier la compilation TypeScript**

```bash
make test-front
```

Expected: aucune erreur TypeScript.

- [ ] **Step 4 : Commit**

```bash
git add frontend/src/pages/TitleDetail.tsx
git commit -m "feat(detail): branche le bouton Refresh sur l'endpoint de rafraîchissement"
```

---

## Vérification end-to-end

1. `make up` + `make dev-frontend`
2. Ouvrir un titre dans le navigateur
3. Ouvrir l'ActionDrawer (tirer vers le haut)
4. Cliquer "↻ Refresh"
5. Vérifier dans `make logs` qu'un refresh est déclenché pour ce titre uniquement
6. Vérifier la console Chrome (pas d'erreur)
7. Vérifier que les autres boutons de l'ActionDrawer fonctionnent toujours
