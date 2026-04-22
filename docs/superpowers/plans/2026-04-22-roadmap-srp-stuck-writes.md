# Roadmap — SRP cleanup & stuck-write hardening

## PO summary

Closes the last remaining single-writer SQLite hang risk, removes an entire class of "did I pass a tx or a db" bugs by making write repositories require a transaction, then splits four oversized code areas for easier future work. Zero user-visible changes expected.

## Source audit

Archival findings: `/Users/nicolasvasse/.claude/plans/search-for-any-srp-flickering-candle.md`.

## Execution order (one session per plan)

1. `2026-04-22-db-stuck-01-merge-tx-audit.md` — A1 `Merge` requires `*sql.DB`. ~30 min.
2. `2026-04-22-db-stuck-02-enrichment-batched-tx.md` — A2 + B3 batched tx + split `handleEnrichment`.
3. `2026-04-22-db-stuck-03-tx-only-write-repos.md` — A4 + A5 tx-only write repos + ctx on Exec. Largest; phased.
4. `2026-04-22-db-stuck-04-background-refresh-ctx.md` — A3 ctx cancellation in refresh loops.
5. `2026-04-22-srp-01-title-repo-split.md` — B1 split `title.go` into 4 files.
6. `2026-04-22-srp-02-pipeline-enrichfromids-split.md` — B2 split `enrichFromIDs`.
7. `2026-04-22-srp-03-handler-title-cleanup.md` — B4 drop client coupling from handler.
8. `2026-04-22-srp-04-background-service-split.md` — B5 extract `CoverService`.
9. `2026-04-22-srp-05-pipeline-strategy-pattern.md` — B6 strategy chain for match pipeline.

## Dependencies

- 2 depends on 1 (Merge signature must stabilize before touching enrichment).
- 3 depends on 2 (batched tx uses new tx-only contract cleanly).
- 5 can run before 3 with minor rework; recommended order is 3 → 5.
- 6, 7, 8, 9, 4 independent; any order after 3.

## Verification entry point

After each plan ships: `make lint && make build && make test`, then `make up` + send a Plex webhook to a local server; tail logs for context-deadline or lock errors.

## Session handoff discipline

Each plan ends with a **Session Handoff Checkpoints** section. When pausing mid-plan:

1. Run `session-handoff` skill to record exact step reached + repo state.
2. Commit WIP if clean boundary (tests green). Otherwise leave uncommitted; handoff skill records the dirty diff summary.
3. Next session: `session-resume` skill reads the handoff file.

Natural break points in every plan:
- After signature/struct change compiles (even if tests red).
- After each completed Task (tests green).
- Between phases in multi-phase plans.

## Completion

When a plan ships: move file from `docs/superpowers/plans/` to `docs/superpowers/plans/done/`. Roadmap stays in `plans/` until all children done, then moves to `done/` last.
