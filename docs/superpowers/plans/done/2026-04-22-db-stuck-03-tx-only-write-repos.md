# A4 + A5 — Tx-only write repositories + ctx propagation

> **For agentic workers:** Multi-phase. Requires A1 and A2 done. Phases 2-4 are parallel across repos (use `superpowers:subagent-driven-development`). Phase 5 sequential.

## PO summary

Removes a whole class of "did I pass a tx or a pool" bugs at compile time. Also ensures cancelled HTTP requests no longer leave write statements running. Invisible to users.

## Goal

Today every repo takes `database.DBTX` — could be pool or tx. Silently dangerous: a caller passing `*sql.DB` to a method that expects tx (or vice versa) compiles fine and fails at runtime via deadlock or lost write. Fix: write-mutating methods require `*sql.Tx`. Read methods keep `DBTX`. All mutating methods take `ctx` and use `ExecContext`/`QueryRowContext`.

## Architecture

- **Split each repository** into:
  - Read half: constructor `NewXxxReader(db database.DBTX)`, methods `Get...`, `List...`, `Find...`, etc.
  - Write half: constructor `NewXxxWriter(tx *sql.Tx)`, methods `Create`, `Update`, `Delete`, `Replace...`, each taking `ctx context.Context` as first param.
- Or keep one struct `XxxRepository` but document + enforce via method receivers:
  - Mutating methods require `ctx` + `*sql.Tx` (embedded in constructor).
  - Reader methods take neither (constructor-injected `DBTX`).
- **Chosen approach:** single struct, dual constructor (`New` for read use, `NewTx` for write use). Write methods panic if created with `DBTX` that isn't a `*sql.Tx`.
- `DBTX` interface stays (used by readers and for backward-compat reader paths).
- Add `WriteDBTX` interface for `ExecContext`, `QueryRowContext` (since `*sql.Tx` satisfies it).

## Tech stack

Go, SQLite, `database/sql`, `context`.

---

### Phase 1 — Introduce `WriteDBTX` + helper [seq]

**Files:**
- Modify: `internal/database/database.go` — add interface.

```go
type WriteDBTX interface {
    ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
    QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
    QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}
```

- [ ] Add interface.
- [ ] Verify `*sql.Tx` satisfies it (compile-time assert: `var _ WriteDBTX = (*sql.Tx)(nil)`).
- [ ] No other changes in this phase.

### Phase 2 — Migrate `TitleRepository` write methods [seq]

**Files:**
- Modify: `internal/repository/title.go`.

Steps:

- [ ] Add `NewTitleWriter(tx *sql.Tx) *TitleRepository` constructor.
- [ ] Change mutating methods (`Create`, `Update`, `Delete`, `ReplaceNames`, `Merge`, `UpdateWatchTime`, `SetCoverURL`, etc.) to take `ctx context.Context` first.
- [ ] Replace `r.db.Exec(...)` with `r.db.ExecContext(ctx, ...)` inside mutating methods.
- [ ] Update callers: wrap callers in `WithTxContext(ctx, writeDB, func(tx){ r := NewTitleWriter(tx); r.Update(ctx, ...) })`.
- [ ] Caller audit: `rg 'titles\.(Create|Update|Delete|Replace|Merge|SetCover)' --type go`.

### Phase 3 — Migrate `EpisodeRepository`, `SeasonRepository`, `WatchEventRepository` [parallel per repo]

Same pattern as Phase 2. Three independent tasks, same steps. Dispatch 3 Sonnet sub-agents.

**Files:**
- `internal/repository/episode.go`
- `internal/repository/season.go`
- `internal/repository/watch_event.go`

For each:
- [ ] Add `NewXxxWriter(tx *sql.Tx)` constructor.
- [ ] Mutating methods take `ctx` first + use `*Context` SQL calls.
- [ ] Update call sites.

### Phase 4 — Migrate `GenreRepository`, `TaskRepository`, `SettingRepository`, `HistoryRepository`, `ActivityRepository`, `StatsRepository` [parallel per repo]

Same pattern. `GenreRepository.ReplaceForTitle` already takes `ctx` — simplify now that callers always wrap in tx (no more runtime type assertion).

**Files:** all remaining `internal/repository/*.go`.

### Phase 5 — Update all services to use tx writers [seq]

**Files:**
- `internal/service/*.go`
- `internal/handler/*.go`

Steps:

- [ ] Each service method that writes: wrap in `WithTxContext`, build writers with tx.
- [ ] Handlers: pass request ctx down; never call writer methods directly (services do that).
- [ ] Run `rg 'NewTitleRepository|NewEpisodeRepository|...' --type go` — every write path should construct via `...Writer(tx)`.

### Phase 6 — Tests + regression [seq]

- [ ] Add a test that ensures passing `*sql.DB` where `*sql.Tx` expected is a compile-time error (documentation test — or skip: compiler handles it).
- [ ] `TestWriterPanicsOnPool`: `NewTitleWriter` given a non-tx → panic. (If we enforce via strong typing, this becomes irrelevant — prefer strong typing.)
- [ ] `make fmt && make lint && make test`.
- [ ] Manual: Plex webhook + enrichment task + background refresh + UI title edit. All four paths write successfully.
- [ ] `make build` + `make up` + curl some endpoints.

### Risks / rollback

- Largest plan. If Phase 5 explodes, revert to before Phase 2 (repos back to `DBTX`) — all intermediate commits form a clean revert chain.
- Test-first for each repo: before migrating, ensure existing tests cover each write path. If gap, add test at current behavior, then migrate.

### Session Handoff Protocol

Invoke `session-handoff` skill when:
- Context-compression warning appears (forced pause).
- User ends the work session.
- End of a Phase if context budget is already >50% — large plan, phase boundaries are the natural breath points.

Do NOT handoff after every task or every migrated repo. Keep a phase in one session when context allows. Resume-pointer table handles any pause.

Handoff file MUST record:
- Last completed phase + (for Phase 3/4) last completed repo.
- Next action — next phase OR next repo within phase OR next plan file.
- Repo state (clean/dirty, branch, uncommitted diff summary).

Resume: run `session-resume` skill — picks up at recorded phase/repo.

#### Resume pointers

| After completing | Next action |
|---|---|
| Phase 1 | Commit `refactor(db): ajoute WriteDBTX pour le contrat d'écriture transactionnel`. Resume at **Phase 2**. |
| Phase 2 | Commit `refactor(title): migre les écritures vers le pattern tx-only + ctx`. Resume at **Phase 3** (dispatch 3 sub-agents in parallel). |
| Phase 3 — repo `episode` done | Commit `refactor(episode): migre les écritures vers tx-only + ctx`. Resume Phase 3 with remaining repos. |
| Phase 3 — repo `season` done | Commit `refactor(season): migre les écritures vers tx-only + ctx`. Resume Phase 3 with remaining repos. |
| Phase 3 — repo `watch_event` done | Commit `refactor(watch_event): migre les écritures vers tx-only + ctx`. Resume at **Phase 4**. |
| Phase 4 — each repo done | Commit per repo (`refactor(<repo>): migre les écritures vers tx-only + ctx`). Resume Phase 4 with remaining, or **Phase 5** when all done. |
| Phase 5 | Commit `refactor(services): construit les writers depuis la transaction`. Resume at **Phase 6**. |
| Phase 6 (final) | Move this file to `docs/superpowers/plans/done/`. Next plan: **`2026-04-22-db-stuck-04-background-refresh-ctx.md`**. |
