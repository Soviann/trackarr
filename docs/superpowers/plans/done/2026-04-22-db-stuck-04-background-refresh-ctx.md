# A3 — Background refresh ctx cancellation

> **For agentic workers:** Single session. Independent of other plans.

## PO summary

Stops the background metadata refresh from blocking app shutdown. Server `make down` now terminates within one second instead of waiting up to ten seconds per stuck API call.

## Goal

`refreshMovieFromTMDB`, `refreshSeriesFromTMDB`, `refreshTVDBSeries`, `refreshAniListAnime` iterate titles and do HTTP calls per iteration. No ctx check between iterations → SIGINT waits for current HTTP client timeout (10s).

## Architecture

- Each loop head: `if err := ctx.Err(); err != nil { return err }`.
- Return `ctx.Err()` so caller logs a cancellation, not an error.
- Keep existing per-call ctx propagation (already done).

## Tech stack

Go, `context`.

---

### Task 1 — Add ctx checks to refresh loops

**Files:**
- Modify: `internal/service/background.go:191-228` (refreshMovieFromTMDB).
- Modify: `internal/service/background.go:241-303` (refreshSeriesFromTMDB).
- Modify: `internal/service/background.go:288-370` (refreshTVDBSeries + refreshAniListAnime).

For each loop over titles:

```go
for _, title := range titles {
    if err := ctx.Err(); err != nil {
        return err
    }
    // ... existing body
}
```

- [ ] Identify each outer loop (one per refresh function).
- [ ] Insert ctx check as first statement in loop body.
- [ ] If inner loops also do HTTP (e.g. per-episode within a series), check ctx there too.

### Task 2 — Tests

**File:** `internal/service/background_test.go`.

Test criteria:

- [ ] `TestRefresh_CancelledContextStopsLoop`: seed 10 titles. Mock TMDB client with 100ms delay per call. Start refresh in goroutine, cancel ctx after 150ms. Assert returns `context.Canceled` in < 300ms (not 1000ms).

### Task 3 — Manual verification

- [ ] `make up`.
- [ ] Trigger manual refresh (admin endpoint or startup auto-refresh).
- [ ] Within 2 seconds: `make down`.
- [ ] Expect shutdown < 2s; no "context deadline exceeded after timeout" in logs.

### Session Handoff Protocol

Invoke `session-handoff` skill ONLY when:
- Context-compression warning appears (forced pause).
- User ends the work session.

Do NOT handoff after every task. This plan is tiny — one session, done.

Handoff file MUST record:
- Last completed task in this plan.
- Next action (task N+1, or next plan file if done).
- Repo state (clean/dirty, branch, uncommitted diff summary).

Resume: run `session-resume` skill.

#### Resume pointers

| After completing | Next action |
|---|---|
| Task 1 | Commit `perf(background): vérifie l'annulation du contexte dans les boucles de rafraîchissement`. Resume at **Task 2**. |
| Task 2 | Commit `test(background): couvre l'annulation du rafraîchissement en cours`. Resume at **Task 3**. |
| Task 3 (final) | Move this file to `docs/superpowers/plans/done/`. Next plan: **`2026-04-22-srp-01-title-repo-split.md`**. |
