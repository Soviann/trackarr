# B4 — `handler/title.go` cleanup

> **For agentic workers:** Single session. Independent of other SRP plans.

## Revision — 2026-04-23

**Scope obsolete.** Pre-existing work (srp-01 / srp-02 / earlier refactors) already delivered the bulk of this plan:

- **Task 1 (extract URL parser):** Already done — see `internal/service/matching/urls.go` (`ParseURL`, `ParseURLFull`, `ParsedURL` struct covering IMDB / AniList / TMDB / TVDB slugs). Tests: `TestParseURL`, `TestParseURLFull_TVDBSlugs`.
- **Task 2 (service method for match-by-URL):** Already done — `TitleService.ResolveURL` (`internal/service/title.go:187`) delegates to `pipeline.ResolveURL` (`internal/service/matching/pipeline.go:202`).
- **Task 3 (drop TMDBClient / AniListClient fields from handler):** Already done — `TitleHandler` no longer holds API clients, only repos + services.

Only residual duplication: 2 inline regexes (`reIMDB`, `reAniList`) in `handler/title.go` used by the `List` handler's *local-DB search shortcut* when the user pastes an IMDB/AniList URL into the search box. This is not match-by-URL enrichment — it's a lookup in the existing library by external ID.

**Actual change delivered (2026-04-23):** Replaced inline regexes with `matching.ParseURL`, extracted lookup into private `findTitleByURL` helper. Dropped `regexp` + `strconv` imports. Existing test `TestTitleHandler_List/search_by_IMDb_URL` covers the flow.

No functional change. Next plan: `2026-04-22-srp-04-background-service-split.md`.

---

## PO summary

Removes direct coupling between the HTTP layer and external API clients. Future API swaps or additions touch only the service layer. No user-visible change.

## Goal

`internal/handler/title.go` currently has a 10-field struct holding `TMDBClient`, `AniListClient` directly + embeds regex URL parsing for IMDB/AniList links. HTTP layer should only depend on services.

## Architecture

- Move URL parsing: create `internal/service/matchurl/parse.go` with `ParseMatchURL(raw string) (*MatchHint, error)` where `MatchHint` = `{IMDBID, AniListID, TMDBID, TVDBID}`.
- Route all match-by-URL requests through `TitleService.MatchByURL(ctx, titleID, url)` which uses `matchurl.Parse` + existing matching pipeline.
- Drop `TMDBClient`, `AniListClient` fields from handler struct.
- Handler struct goes from 10 fields to ~5 (repos + services).

## Tech stack

Go.

---

### Task 1 — Extract URL parser

**Files:**
- Create: `internal/service/matchurl/parse.go`.
- Create: `internal/service/matchurl/parse_test.go`.

```go
package matchurl

type Hint struct {
    IMDBID    string
    AniListID int64
    TMDBID    int64
    TVDBID    int64
}

func Parse(raw string) (*Hint, error) { ... }
```

- [ ] Move regexes from `handler/title.go` (IMDB, AniList, TMDB, TVDB URL patterns).
- [ ] Return first matching source; `nil` + error if no match.
- [ ] Tests: table-driven for each pattern + invalid input.

### Task 2 — Add service method

**File:** `internal/service/title.go`.

- [ ] Add `TitleService.MatchByURL(ctx, titleID int64, rawURL string) error`.
- [ ] Body: `hint, err := matchurl.Parse(rawURL)` → run pipeline with hint → persist.
- [ ] If handler previously did direct TMDB/AniList calls, replace with service call.

### Task 3 — Slim handler

**Files:**
- Modify: `internal/handler/title.go`.

- [ ] Drop `tmdb`, `anilist` fields from handler struct.
- [ ] Handler method that handled match-by-URL: replace body with `h.titleSvc.MatchByURL(r.Context(), titleID, url)`.
- [ ] Update constructor signature; fix call sites in `cmd/serve.go`.

### Task 4 — Tests + regression

- [ ] Handler test: POST match-by-URL → service mock called with right args.
- [ ] `make fmt && make lint && make test && make build`.
- [ ] Manual: Add a title by IMDB URL + AniList URL via the UI; both match.

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
| Task 1 | Commit `refactor(matching): extrait le parsing d'URL de correspondance`. Resume at **Task 2**. |
| Task 2 | Commit `feat(title): centralise MatchByURL dans TitleService`. Resume at **Task 3**. |
| Task 3 | No commit yet — handler compiles but regression not confirmed. Resume at **Task 4**. |
| Task 4 (final) | Commit `refactor(handler): supprime les dépendances aux clients TMDB/AniList`. Move this file to `docs/superpowers/plans/done/`. Next plan: **`2026-04-22-srp-04-background-service-split.md`**. |
