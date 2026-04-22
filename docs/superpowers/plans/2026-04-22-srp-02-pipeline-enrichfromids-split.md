# B2 — Split `Pipeline.enrichFromIDs`

> **For agentic workers:** Single session. Subtle — preserves field-precedence rules during metadata fusion.

## PO summary

Breaks a 221-line method that mixes cross-reference lookup with three external API fetches into focused helpers. No user-visible change; a future bug in one source can't silently corrupt the others.

## Goal

`internal/service/matching/pipeline.go:enrichFromIDs` does: DB cross-ref lookup → TMDB fetch → AniList fetch → metadata fusion (with TMDB overriding nil fields, AniList overriding specific fields). Split into named steps with an explicit fusion function that documents precedence rules.

## Architecture

```go
func (p *Pipeline) enrichFromIDs(ctx, result *MatchResult, input MatchInput) error {
    if err := p.enrichFromCrossRef(ctx, result); err != nil { ... }
    if err := p.enrichFromTMDB(ctx, result, input); err != nil { ... }
    if err := p.enrichFromAniList(ctx, result, input); err != nil { ... }
    return nil
}
```

- `enrichFromCrossRef(ctx, result)` — resolves missing external IDs from local cross-ref table.
- `enrichFromTMDB(ctx, result, input)` — fetches TMDB details, sets fields via `mergeFromTMDB(result, details)`.
- `enrichFromAniList(ctx, result, input)` — same for AniList via `mergeFromAniList(result, anime)`.
- `mergeFromTMDB(result, details)` — pure function; documents exactly which fields TMDB provides and the precedence (TMDB fills only nil/empty fields, EXCEPT genres which always replace).
- `mergeFromAniList(result, anime)` — pure function; documents AniList precedence (AniList overrides IsAnime, AniListRating, specific genre aliases).

## Tech stack

Go.

---

### Task 1 — Characterization test (pre-split)

**File:** `internal/service/matching/pipeline_test.go`.

Before touching code, lock down current behavior:

- [ ] `TestEnrichFromIDs_TMDBOnly`: mock TMDB, no AniList. Assert output fields match current behavior (genres, runtime, overview, ratings).
- [ ] `TestEnrichFromIDs_BothSources_AniListOverrides`: both mocks. Assert AniList rating wins over TMDB rating, IsAnime flipped true, genre list merged.
- [ ] `TestEnrichFromIDs_TMDBFails`: TMDB returns error. Assert partial result with AniList data only, no panic.
- [ ] `TestEnrichFromIDs_CrossRefFillsMissingIDs`: pre-seed cross-ref row; assert missing IMDBID filled in.
- [ ] Run; capture baseline outputs.

### Task 2 — Extract helpers

- [ ] Extract `enrichFromCrossRef(ctx, result)` — lines ~365-400 of current method.
- [ ] Extract `enrichFromTMDB(ctx, result, input)` + `mergeFromTMDB(result, details)`.
- [ ] Extract `enrichFromAniList(ctx, result, input)` + `mergeFromAniList(result, anime)`.
- [ ] Add doc comments stating field-precedence rules on each merge function.
- [ ] `enrichFromIDs` becomes a thin orchestrator (~20 lines).
- [ ] Existing tests + new ones all green after each extract.

### Task 3 — Regression

- [ ] `make fmt && make lint && make test`.
- [ ] Manual: trigger enrichment for a movie + an anime; confirm both end up with correct metadata in UI.

### Session Handoff Protocol

Invoke `session-handoff` skill ONLY when:
- Context-compression warning appears (forced pause).
- User ends the work session.

Do NOT handoff after each helper extract. Execute the plan end-to-end in one session when context allows.

Handoff file MUST record:
- Last completed task (and, inside Task 2, last helper extracted).
- Next action.
- Repo state.

Resume: run `session-resume` skill.

#### Resume pointers

| After completing | Next action |
|---|---|
| Task 1 | Commit `test(matching): caractérise enrichFromIDs avant découpage`. Resume at **Task 2** (extract helpers). |
| Task 2 — `enrichFromCrossRef` extracted | Resume Task 2: extract `enrichFromTMDB` + `mergeFromTMDB`. |
| Task 2 — `enrichFromTMDB` extracted | Resume Task 2: extract `enrichFromAniList` + `mergeFromAniList`. |
| Task 2 — all helpers extracted | Resume at **Task 3** (regression). |
| Task 3 (final) | Commit `refactor(matching): découpe enrichFromIDs par source de métadonnées`. Move this file to `docs/superpowers/plans/done/`. Next plan: **`2026-04-22-srp-03-handler-title-cleanup.md`**. |
