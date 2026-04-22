# B1 — Split `repository/title.go`

> **For agentic workers:** Single session, multi-file. Pure mechanical split, no logic change. Use TDD: ensure existing tests pass after each move.

## PO summary

Breaks up the biggest file in the codebase (1166 lines) into four focused files. Makes finding and changing title queries drastically easier. No behavior change.

## Goal

`internal/repository/title.go` mixes CRUD, complex filter list (178-line `List`), search ranking, pagination, and relation loading. Split into 4 files within the same package; same exported methods on same struct.

## Architecture

Same package `repository`, same struct `TitleRepository`. Methods split across files via Go's multi-file-per-struct support (any method on `*TitleRepository` can live in any file in the package).

- `internal/repository/title.go` — struct def + constructor + `Create`, `Update`, `Delete`, `GetByID`, `SetCoverURL`, `FindByExternalID`, basic CRUD. ~300 lines target.
- `internal/repository/title_list.go` — `List(filter TitleFilter)` + filter-to-SQL helpers + `TitleFilter` struct. ~400 lines.
- `internal/repository/title_search.go` — `Search(query string)` + ranking helpers. ~200 lines.
- `internal/repository/title_relations.go` — `loadTitleRelations`, `loadTitleRelationsLight`, `ReplaceNames`, name/season/episode preloading. ~250 lines.

## Tech stack

Go.

---

### Task 1 — Create new files, move methods

**Files:**
- Modify: `internal/repository/title.go` (shrink).
- Create: `internal/repository/title_list.go`.
- Create: `internal/repository/title_search.go`.
- Create: `internal/repository/title_relations.go`.

Steps:

- [ ] Baseline: `make test` green.
- [ ] Move `List` + `TitleFilter` + `applyTitleFilter` + any list-only helpers → `title_list.go`. Keep same `package repository`, same imports.
- [ ] Move `Search` + ranking helpers → `title_search.go`.
- [ ] Move `loadTitleRelations`, `loadTitleRelationsLight`, `ReplaceNames`, internal preload helpers → `title_relations.go`.
- [ ] Keep CRUD + struct def + constructor in `title.go`.
- [ ] After each move: `make build` + `make test`. No logic change.

### Task 2 — Verify nothing broke

- [ ] `make fmt && make lint && make test`.
- [ ] `rg 'TitleFilter|TitleRepository' --type go` — all imports still resolve (same package).
- [ ] Manual: load `/library` page in browser; filters work; search works; title detail loads.

### Session Handoff Protocol

Invoke `session-handoff` skill ONLY when:
- Context-compression warning appears (forced pause).
- User ends the work session.

Do NOT handoff after every extracted file. Mechanical split — one session, done. Resume-pointer table handles any forced pause.

Handoff file MUST record:
- Last completed task (and, inside Task 1, last file moved).
- Next action (next task, next file, or next plan).
- Repo state (clean/dirty, branch, uncommitted diff summary).

Resume: run `session-resume` skill.

#### Resume pointers

| After completing | Next action |
|---|---|
| Task 1 — `title_list.go` extracted | Resume Task 1: extract `title_search.go`. |
| Task 1 — `title_search.go` extracted | Resume Task 1: extract `title_relations.go`. |
| Task 1 — all files extracted | Resume at **Task 2** (regression). |
| Task 2 (final) | Commit `refactor(repository): découpe title.go en quatre fichiers par responsabilité`. Move this file to `docs/superpowers/plans/done/`. Next plan: **`2026-04-22-srp-02-pipeline-enrichfromids-split.md`**. |
