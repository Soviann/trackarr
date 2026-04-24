# B6 — Match pipeline strategy pattern

> **For agentic workers:** Single session. Benefits most after B2 is done (cleaner `Run` method to refactor).

## Revision — 2026-04-24

Scope ajusté après lecture du code :

- **Task 1** : 7 tests de caractérisation déjà présents dans `pipeline_test.go` (Step1/2/3/4/NoMatch/IMDBConflict/NilClients). Seul `TestPipeline_Step5_GeminiFuzzy` manque (succès fuzzy → TMDB résolu). `TestRun_AniListForAnime` retiré : code actuel n'utilise pas `IsAnime` pour prioriser AniList sur TMDB (écriture de ce test serait un changement de comportement).
- **Task 2** : stratégies dans le **même package** `internal/service/matching/` (fichiers `strategy_plexids.go`, `strategy_crossref.go`, etc.). Éviter sous-package `strategy/` qui exigerait d'exporter TMDBClient/AniListClient/GeminiClient/CrossRefDB.
- **Architecture** : chaque stratégie porte `*Pipeline` pour réutiliser `enrichFromIDs` et `verifyAndEnrich`. Signature `Try(ctx, input) (*MatchResult, bool, error)` retourne un résultat complet (identification + enrichissement). `Run` devient une simple boucle for.
- **Task 4** : tests par stratégie ciblent la logique d'identification (succès/miss/erreur de recherche) avec dépendances mockées minimalement — l'enrichissement reste couvert par les tests pipeline de Task 1.

## PO summary

Makes the matching pipeline extensible: adding a new metadata source (or reordering for specific title types) becomes a three-line change instead of a nested if/else edit. No user-visible change.

## Goal

`Pipeline.Run` has 5 nested if/else branches (Plex IDs → cross-ref → TMDB → AniList → Gemini fuzzy). Convert to a slice of `MatchStrategy` executed in order; first successful match wins.

## Architecture

```go
type MatchStrategy interface {
    Name() string
    Try(ctx context.Context, input MatchInput) (*MatchResult, bool, error)
    // Returns (result, matched, err). matched=true ends the chain; err=stop.
}

type Pipeline struct {
    strategies []MatchStrategy
    // ... existing fields
}

func (p *Pipeline) Run(ctx context.Context, input MatchInput) (*MatchResult, error) {
    for _, s := range p.strategies {
        if err := ctx.Err(); err != nil {
            return nil, err
        }
        result, matched, err := s.Try(ctx, input)
        if err != nil {
            return nil, fmt.Errorf("%s: %w", s.Name(), err)
        }
        if matched {
            return result, nil
        }
    }
    return &MatchResult{MatchStatus: model.MatchStatusUnmatched}, nil
}
```

Strategies (one per file, under `internal/service/matching/strategy/`):
- `PlexIDStrategy` — uses IDs from payload directly.
- `CrossRefStrategy` — looks up local cross-reference table.
- `TMDBStrategy` — queries TMDB search API.
- `AniListStrategy` — queries AniList GraphQL.
- `GeminiFuzzyStrategy` — fuzzy fallback via Gemini (rate-limited).

Each: ~80 lines, own tests.

## Tech stack

Go.

---

### Task 1 — Characterization tests on current `Run`

**File:** `internal/service/matching/pipeline_test.go`.

- [ ] `TestRun_PlexIDsMatch`: input with TMDBID → returns Plex-ID-sourced result, no external calls.
- [ ] `TestRun_CrossRefMatch`: seed cross-ref table, no Plex IDs → returns cross-ref result.
- [ ] `TestRun_TMDBSearchWinsForMovie`: no IDs, input is movie → TMDB search hit, AniList not called.
- [ ] `TestRun_AniListForAnime`: input `IsAnime=true` → AniList prioritized over TMDB.
- [ ] `TestRun_GeminiFallback`: all prior sources fail → Gemini called.
- [ ] `TestRun_AllFail`: returns Unmatched, no error.

Run; capture baseline.

### Task 2 — Introduce interface + extract strategies

**Files:**
- Create: `internal/service/matching/strategy/interface.go` (the `MatchStrategy` interface).
- Create: one file per strategy.

- [ ] Create interface.
- [ ] Extract each branch of current `Run` into its own struct implementing `MatchStrategy.Try`.
- [ ] Keep internal helpers (e.g. TMDB client, logger) as fields injected by constructor.
- [ ] `Pipeline` constructor now takes `strategies []MatchStrategy` in order; `cmd/serve.go` builds the default chain.

### Task 3 — Rewrite `Pipeline.Run`

- [ ] Replace body with the for-loop shown in Architecture.
- [ ] Delete old branches.
- [ ] Re-run Task 1 tests; all green.

### Task 4 — Per-strategy tests

**Files:** `internal/service/matching/strategy/*_test.go`.

- [ ] One test file per strategy. Mock only that strategy's dependencies. Faster + clearer than testing the whole pipeline.

### Task 5 — Regression

- [ ] `make fmt && make lint && make test && make build`.
- [ ] Manual: create titles for movie, anime, TV show via Plex webhook. Each matches via its expected strategy (check logs — each strategy logs its name).

### Session Handoff Protocol

Invoke `session-handoff` skill ONLY when:
- Context-compression warning appears (forced pause).
- User ends the work session.

Do NOT handoff after each extracted strategy. Execute the plan end-to-end in one session when context allows. Five strategies are mechanical — resume-pointer table handles a forced pause if context fills up.

Handoff file MUST record:
- Last completed task (and, inside Task 2, last strategy extracted).
- Next action.
- Repo state.

Resume: run `session-resume` skill.

#### Resume pointers

| After completing | Next action |
|---|---|
| Task 1 | Commit `test(matching): caractérise la chaîne de stratégies de matching`. Resume at **Task 2** (extract strategies one by one). |
| Task 2 — `PlexIDStrategy` extracted | Resume Task 2: extract `CrossRefStrategy`. |
| Task 2 — `CrossRefStrategy` extracted | Resume Task 2: extract `TMDBStrategy`. |
| Task 2 — `TMDBStrategy` extracted | Resume Task 2: extract `AniListStrategy`. |
| Task 2 — `AniListStrategy` extracted | Resume Task 2: extract `GeminiFuzzyStrategy`. |
| Task 2 — all strategies extracted | Resume at **Task 3** (rewrite `Pipeline.Run`). |
| Task 3 | Commit `refactor(matching): introduit MatchStrategy et itère au lieu de brancher`. Resume at **Task 4**. |
| Task 4 | Commit `test(matching): couvre chaque stratégie isolément`. Resume at **Task 5**. |
| Task 5 (final) | Move this file to `docs/superpowers/plans/done/`. This is the last plan in the chain — move roadmap `2026-04-22-roadmap-srp-stuck-writes.md` to `done/` as well. No next plan. |
