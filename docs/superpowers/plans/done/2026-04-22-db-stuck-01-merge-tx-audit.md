# A1 — `Merge` nested-tx audit & fix

> **For agentic workers:** Single sequential session. Use `superpowers:executing-plans`.

## PO summary

Eliminates a rare 30-second app freeze that can trigger during title enrichment. No user-visible change. Ships as one small PR.

## Goal

`TitleService.Merge` signature currently accepts `database.DBTX` — hides whether caller is in a tx. `Merge` internally calls `WithTxContext`, so passing a `*sql.Tx` deadlocks on the single-writer connection. Tighten signature to `*sql.DB`; enforce at compile time.

## Architecture

- Signature: `Merge(ctx context.Context, db database.DBTX, destID, sourceID int64, explicitOffset *int) error` → `Merge(ctx context.Context, db *sql.DB, destID, sourceID int64, explicitOffset *int) error`.
- Callers must pass the pool handle (writeDB), never a tx.

## Tech stack

Go, SQLite, `database/sql`.

---

### Task 1 — Tighten `Merge` signature

**Files:**
- Modify: `internal/service/title.go:189` (Merge signature + body).
- Modify: `internal/service/taskqueue.go:315` (call site uses `w.titles.DB()`).

Steps:

- [ ] Audit all `Merge(` call sites: `rg 'titleSvc\.Merge|TitleService.*Merge' --type go`.
- [ ] Change `Merge` signature: second param `db *sql.DB`.
- [ ] Body: replace `database.WithTxContext(ctx, db, ...)` — already correct, but confirm `db` is used only for tx open (no direct Exec).
- [ ] `TaskQueueWorker`: add `writeDB *sql.DB` field; inject in `NewTaskQueueWorker(...)`; use it at line 315 instead of `w.titles.DB()`.
- [ ] If `TitleRepository.DB()` has no other caller, delete it. Verify with `rg '\.DB\(\)' --type go`.

### Task 2 — Test the contract

**File:**
- Add or extend: `internal/service/title_test.go`.

Test criteria:

- [ ] `TestMerge_OpensOwnTx`: creates 2 titles, calls `Merge(ctx, db, destID, sourceID, nil)`, asserts dest has merged names/seasons and source deleted.
- [ ] `TestMerge_CancelledContext`: pass ctx with `cancel()` called before `Merge`; asserts `Merge` returns `context.Canceled`, source title still exists.

### Task 3 — Regression run

- [ ] `make fmt && make lint && make test`.
- [ ] `make up` + fire a Plex scrobble for an existing anime title; confirm no errors in logs.

### Session Handoff Protocol

Invoke `session-handoff` skill ONLY when:
- Context-compression warning appears (forced pause).
- User ends the work session.

Do NOT handoff after every task. This plan is small — execute in one session when possible. The resume-pointer table below maps any forced pause back to the right spot.

Handoff file MUST record:
- Last completed task in this plan.
- Next action (task N+1 in this plan, or next plan file if this one is done).
- Repo state (clean/dirty, branch, uncommitted diff summary).

Resume: run `session-resume` skill — it reads the handoff + this plan and picks up at the recorded step. Single command, no context needed.

#### Resume pointers

| After completing | Next action |
|---|---|
| Task 1 | Resume this plan at **Task 2**. |
| Task 2 | Resume this plan at **Task 3**. |
| Task 3 (final) | Commit `fix(enrichment): Merge exige un pool SQL et non une transaction`. Move this file to `docs/superpowers/plans/done/`. Next plan: **`2026-04-22-db-stuck-02-enrichment-batched-tx.md`**. |
