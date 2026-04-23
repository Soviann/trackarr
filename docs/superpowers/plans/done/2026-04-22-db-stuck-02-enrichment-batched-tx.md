# A2 + B3 — Enrichment batched tx & handleEnrichment split

> **For agentic workers:** Single sequential session. Requires A1 done first (signature stability).

## PO summary

Speeds up title enrichment under load and makes the code readable for future changes. No user-visible behavior change.

## Goal

`handleEnrichment` (122 lines, 5 concerns) currently issues up to 4 sequential writes, each grabbing+releasing the single writer lock. Split method into focused helpers, wrap the persistence section in a single transaction.

## Architecture

- Split `handleEnrichment(ctx, task)` into:
  - `deserializePayload(task) (EnrichmentPayload, error)`
  - `runPipeline(ctx, payload) (*matching.MatchResult, error)`
  - `buildUpdate(result, payload) repository.TitleUpdate`
  - `persistEnrichment(ctx, tx, titleID, update, result) error` (all writes in one tx)
  - `resolveAnimeConflict(ctx, result, payload) (merged bool, err error)`
  - `recalcWatchtime(ctx, tx, titleID, runtime *int) error`
- `handleEnrichment` orchestrates: deserialize → run → build → `WithTxContext { persist; recalc }` → resolveConflict (outside tx; may delete title).

## Tech stack

Go, SQLite, `database/sql`.

---

### Task 1 — Extract helpers (pure split, no behavior change)

**Files:**
- Modify: `internal/service/taskqueue.go:223-343`.

Steps:

- [ ] Extract `buildUpdate(result, payload)` — lines 245-287. Pure function, returns `repository.TitleUpdate`.
- [ ] Extract `resolveAnimeConflict(ctx, result, payload)` — lines 309-324. Returns `(merged bool, err error)`; `merged=true` means caller should return early.
- [ ] Extract `recalcWatchtime(ctx, db, titleID, runtime)` — lines 295-305. Takes `DBTX` for now (phase-compatible with plan 3).
- [ ] Leave `handleEnrichment` calling all three sequentially; behavior identical.
- [ ] `make test` must stay green (no change in logic).

### Task 2 — Batch writes in one tx

Steps:

- [ ] Import `database` package in taskqueue.go if missing.
- [ ] Wrap `titles.Update(payload.TitleID, update)` + optional `titles.Update(payload.TitleID, {TotalWatchMinutes})` + `titles.ReplaceNames(...)` + `genres.ReplaceForTitle(ctx, ...)` in `database.WithTxContext(ctx, w.writeDB, func(tx *sql.Tx) error { ... })`.
- [ ] Create tx-local repo instances: `titles := repository.NewTitleRepository(tx)`, `genres := repository.NewGenreRepository(tx)`.
- [ ] Move `resolveAnimeConflict` **outside** the tx: merge has its own tx and may delete. Sequence: persist tx → if merged, log + return nil. If not merged, continue (already handled).
- [ ] Conflict check SELECT (`FindByExternalID`) stays outside the tx — read-only, uses readDB.

### Task 3 — Tests

**File:** `internal/service/taskqueue_test.go` (create or extend).

Test criteria:

- [ ] `TestHandleEnrichment_SingleTx`: mock pipeline returns a full result (TMDBID, runtime, names, genres). Run; assert only one `BEGIN`/`COMMIT` pair in query log (use `sqlmock` or hook pragma `query_only` counter).
- [ ] `TestHandleEnrichment_PipelineErrorDoesNotWrite`: pipeline returns error → assert no Update called.
- [ ] `TestHandleEnrichment_AnimeConflictTriggersMerge`: IMDB+IsAnime result with existing title → assert `Merge` called with correct IDs, source title removed.
- [ ] `TestHandleEnrichment_CtxCancelRollsBack`: cancel ctx inside tx → assert no row changed.

### Task 4 — Regression

- [ ] `make fmt && make lint && make test`.
- [ ] Manual: trigger enrichment via `POST /api/titles/:id/refresh` (or equivalent) on a movie; tail logs — should see one tx block, not four.

### Session Handoff Protocol

Invoke `session-handoff` skill ONLY when:
- Context-compression warning appears (forced pause).
- User ends the work session.

Do NOT handoff after every task. Execute the full plan in one session when possible. Resume-pointer table maps any forced pause back to the right spot.

Handoff file MUST record:
- Last completed task in this plan.
- Next action (task N+1 in this plan, or next plan file if this one is done).
- Repo state (clean/dirty, branch, uncommitted diff summary).

Resume: run `session-resume` skill — it reads the handoff + this plan and picks up at the recorded step.

#### Resume pointers

| After completing | Next action |
|---|---|
| Task 1 | Commit `refactor(enrichment): découpe handleEnrichment en fonctions dédiées`. Resume at **Task 2**. |
| Task 2 | Commit `perf(enrichment): regroupe les écritures de métadonnées en une transaction`. Resume at **Task 3**. |
| Task 3 | Commit `test(enrichment): ajoute la couverture transaction et conflit d'anime`. Resume at **Task 4**. |
| Task 4 (final) | Move this file to `docs/superpowers/plans/done/`. Next plan: **`2026-04-22-db-stuck-03-tx-only-write-repos.md`**. |
