# B2 — Split `Pipeline.enrichFromIDs`

> **For agentic workers:** Single session. Subtle — preserves field-precedence rules during metadata fusion.

## Revision — 2026-04-23

Original plan assumed TMDB+AniList fusion. Actual code (post-TVDB integration) does cross-ref → TMDB FindByID → AniList ID/name → **parallel TMDB+TVDB fetch** → fusion of TMDB+TVDB → AniList cover fallback. AniList contributes only: ID detection, romaji name, last-resort cover/rating.

Existing tests already cover characterization (PlexIDs, CrossRef, TMDBSearch, AniListSearch, IMDBConflict, NilClients) — Task 1 reduced to a coverage audit instead of writing fresh tests.

## PO summary

Breaks a ~190-line method that mixes ID resolution, parallel API fetches, and metadata fusion into focused helpers. No user-visible change; a future bug in one source can't silently corrupt the others.

## Goal

`internal/service/matching/pipeline.go:enrichFromIDs` (l360-550) split into named steps with an explicit fusion function that documents precedence rules.

## Architecture

```go
func (p *Pipeline) enrichFromIDs(ctx, result *MatchResult, input MatchInput) {
    p.resolveIDsFromSources(ctx, result, input)
    p.appendAniListRomaji(ctx, result)
    if len(result.Names) == 0 {
        result.Names = []model.TitleName{{Name: input.Title, Language: "en", IsPrimary: true}}
    }
    tmdbRes, tvdbRes := p.fetchTMDBAndTVDBParallel(ctx, result)
    mergeSourcesIntoResult(result, tmdbRes, tvdbRes)
    if result.CoverFile == "" && p.anilist != nil && result.AniListID != 0 {
        p.downloadAniListCover(ctx, result)
    }
}
```

- `resolveIDsFromSources(ctx, result, input)` — cross-ref lookup + TMDB FindByID-by-IMDB + AniList search-for-ID + IsAnime detection (current l362-403).
- `appendAniListRomaji(ctx, result)` — appends romaji name from AniList GetNames (current l406-416).
- `fetchTMDBAndTVDBParallel(ctx, result) (tmdbFetchResult, tvdbFetchResult)` — parallel goroutines with 20s timeout (current l423-449).
- `mergeSourcesIntoResult(result, tmdb, tvdb)` — pure-ish (writes to result), documents precedence: overview-longest-wins, genres-union, runtime-TMDB-first, IDs-canonical-from-TMDB, conflict-detection (current l451-545). May further split into `mergeMetadata` / `reconcileIDs` if it stays > 80 lines.

## Tech stack

Go.

---

### Task 1 — Characterization audit (pre-split)

Existing `pipeline_test.go` already covers:
- `TestPipeline_Step1_PlexIDsConfirmed` — TMDB enrichment via PlexIDs path
- `TestPipeline_Step2_CrossRef` — crossref fills TMDB+IMDB+AniList from TVDB-only input; AniList ID triggers IsAnime
- `TestPipeline_Step3_TMDBSearch` / `_TVSearch` — TMDB enrichment via search path
- `TestPipeline_Step4_AniListSearch` — AniList ID populated, IsAnime true
- `TestPipeline_IMDBConflict_TMDBWins` — TMDB+TVDB conflict → pending_review, TMDB IMDB kept
- `TestPipeline_NilClients` — graceful with nil clients
- `TestPipeline_NoMatch` — no source returns anything

Steps:
- [ ] Run `make test PKG=./internal/service/matching/...`; capture baseline pass count.
- [ ] Confirm above tests exercise: cross-ref, TMDB metadata (genres/runtime/overview/ratings/credits), TVDB merge (via IMDBConflict), AniList ID + IsAnime detection, conflict downgrade.
- [ ] Gap noted: no test for `appendAniListRomaji` adding the `x-romaji` name. Add `TestEnrichFromIDs_AppendsAniListRomaji` if not covered.

### Task 2 — Extract helpers

Each step: extract → run `make test PKG=./internal/service/matching/...` → green before next.

- [ ] Extract `resolveIDsFromSources(ctx, result, input)` — current l362-403 (cross-ref + TMDB FindByID + AniList search + IsAnime detection from AniList ID).
- [ ] Extract `appendAniListRomaji(ctx, result)` — current l406-416.
- [ ] Extract `fetchTMDBAndTVDBParallel(ctx, result) (tmdbFetchResult, tvdbFetchResult)` — current l423-449 (returns the two structs; orchestrator binds them).
- [ ] Extract `mergeSourcesIntoResult(result *MatchResult, tmdb tmdbFetchResult, tvdb tvdbFetchResult)` — current l451-545. Doc comment listing precedence rules per field group.
- [ ] If `mergeSourcesIntoResult` > 80 lines, split into `mergeMetadata` (overview/genres/runtime/credits/ratings/release/cover) + `reconcileIDs` (IMDB/TMDB/TVDB ID fills + conflict detection).
- [ ] `enrichFromIDs` becomes a ~10-line orchestrator.

### Task 3 — Regression

- [ ] `make fmt && make lint && make test && make build`.
- [ ] Visual smoke: trigger a Plex sync (or manual rematch on one movie + one anime) via UI; confirm metadata + cover load.

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
| Task 1 | If gap test added: commit `test(matching): caractérise appendAniListRomaji`. Resume at **Task 2**. |
| Task 2 — `resolveIDsFromSources` extracted | Resume Task 2: extract `appendAniListRomaji`. |
| Task 2 — `appendAniListRomaji` extracted | Resume Task 2: extract `fetchTMDBAndTVDBParallel`. |
| Task 2 — `fetchTMDBAndTVDBParallel` extracted | Resume Task 2: extract `mergeSourcesIntoResult`. |
| Task 2 — all helpers extracted | Resume at **Task 3** (regression). |
| Task 3 (final) | Commit `refactor(srp-02): découpe enrichFromIDs par étape (resolve/fetch/merge)`. Move this file to `docs/superpowers/plans/done/`. Next plan: **`2026-04-22-srp-03-handler-title-cleanup.md`**. |
