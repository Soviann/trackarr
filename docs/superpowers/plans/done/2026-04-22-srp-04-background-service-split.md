# B5 — Split `BackgroundService`

> **For agentic workers:** Single session. Independent of other SRP plans.

## Revision — 2026-04-24

Plan adapté pour refléter le code réel :

- **Nombre de champs :** 9 (non 12). Pas de `httpClient` ni `coverDir string` dédié — chaque client externe (TMDB / AniList / TVDB) possède son propre HTTP client et sa propre méthode `DownloadCover`. Le seul helper restant côté `BackgroundService` était `coversDir()` (concaténation `dataDir + "covers"`).
- **Méthodes cibles abandonnées :** pas de `DownloadCover(ctx, titleID, url)`, `DeleteCover(titleID, filename)` ni `MigrateLegacyCovers(ctx)` — ces méthodes n'existaient pas, leur extraction aurait introduit une abstraction sans usager.
- **Ce qui a effectivement été extrait dans `CoverService`** (`internal/service/cover.go`) :
  - `FetchMissingCovers(ctx) int`
  - `CleanupUnusedCovers(ctx, day)` + `processCoverBatch` + `coverDailyPrefixes` (renommé depuis `getDailyPrefixes`)
  - `DownloadAniListCover(ctx, title) bool` (exporté depuis `downloadAniListCover`)
  - `enqueueCoverOnRetryable` (privé)
  - `Dir() string` (remplace `BackgroundService.coversDir()`)
- **Appels inline `s.tmdb/tvdb/anilist.DownloadCover(...)` dans les méthodes de refresh :** conservés. Déjà 1-liners délégant aux clients API. Routent désormais `s.covers.Dir()` pour le chemin cible.
- **Constructeur `NewBackgroundService` :** perd les paramètres `anilist` et `dataDir`, gagne `covers *CoverService`. Wiring fait dans `cmd/serve.go`.
- **Tests :** `TestBackgroundService_CleanupUnusedCovers` migré vers `TestCoverService_CleanupUnusedCovers` dans `internal/service/cover_test.go`. `setupBackgroundService` construit un `CoverService` minimal (clients nil) pour les tests.

**Gain mesuré :** `background.go` passe de 727 à 508 lignes ; `cover.go` fait ~280 lignes. Structure `BackgroundService` 9 → 8 champs (perd `anilist`, `dataDir`, gagne `covers`).

Aucun changement fonctionnel, aucun impact utilisateur.

---

## PO summary

Separates the code that downloads cover images from the code that refreshes title metadata. Each piece becomes easier to read and to test. No user-visible change.

## Goal

`internal/service/background.go` is a 12-field struct mixing:
1. Refresh orchestration (scheduling, API calls per title).
2. Cover file management (download, rename, delete from disk).
3. Notification sending (push to user).

Extract `CoverService`. Keep `BackgroundService` as refresh orchestrator.

## Architecture

- New: `internal/service/cover.go` — `CoverService` struct.
  - Fields: `coverDir string`, `httpClient *http.Client`, `titles *repository.TitleRepository`.
  - Methods: `DownloadCover(ctx, titleID, url) (filename string, err error)`, `DeleteCover(titleID, filename) error`, `MigrateLegacyCovers(ctx) error`.
- `BackgroundService`: drop cover-related fields/methods; inject `*CoverService`; call it where needed.
- Notification: already uses `PushNotifier` interface — leave as is.

## Tech stack

Go, file I/O, HTTP.

---

### Task 1 — Create `CoverService`

**Files:**
- Create: `internal/service/cover.go`.
- Create: `internal/service/cover_test.go`.

- [ ] Move `downloadCover`, `coverDir`, cover HTTP client, cover-related file path helpers from `background.go`.
- [ ] Constructor: `NewCoverService(coverDir string, titles *repository.TitleRepository) *CoverService`.
- [ ] Expose methods listed in Architecture.
- [ ] Tests: mock HTTP server, download to tmp dir, assert file + DB updated.

### Task 2 — Refactor `BackgroundService`

**Files:**
- Modify: `internal/service/background.go`.
- Modify: `cmd/serve.go` (construction order).

- [ ] Drop cover fields/methods from `BackgroundService`.
- [ ] Add `covers *CoverService` field.
- [ ] Where refresh loops previously called internal cover methods, call `s.covers.DownloadCover(...)` instead.
- [ ] `cmd/serve.go`: build `CoverService` before `BackgroundService`; inject.

### Task 3 — Regression

- [ ] `make fmt && make lint && make test && make build`.
- [ ] Manual: add a title → cover downloads to disk. Delete title → cover removed. Trigger metadata refresh → cover re-checked.

### Session Handoff Protocol

Invoke `session-handoff` skill ONLY when:
- Context-compression warning appears (forced pause).
- User ends the work session.

Do NOT handoff after every task. Small plan — one session.

Handoff file MUST record:
- Last completed task.
- Next action.
- Repo state.

Resume: run `session-resume` skill.

#### Resume pointers

| After completing | Next action |
|---|---|
| Task 1 | Commit `feat(cover): extrait CoverService responsable du téléchargement et de la suppression`. Resume at **Task 2**. |
| Task 2 | No commit yet — wiring done but regression not confirmed. Resume at **Task 3**. |
| Task 3 (final) | Commit `refactor(background): délègue la gestion des couvertures à CoverService`. Move this file to `docs/superpowers/plans/done/`. Next plan: **`2026-04-22-srp-05-pipeline-strategy-pattern.md`**. |
