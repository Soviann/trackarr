# Fix 504 : séparer connexions SQLite lecture/écriture

## Context

Les pages de liste retournent des 504 en production. Le background refresh (écritures séquentielles avec rate limiter TMDB) monopolise l'unique connexion SQLite (`MaxOpenConns=1`), bloquant toutes les requêtes HTTP de lecture. Le reverse proxy coupe à ~60s → 504.

SQLite WAL supporte N readers + 1 writer en parallèle sur des connexions séparées.

## Step 1 — `database.Open` retourne 2 connexions

**Fichier : `internal/database/database.go`**

Modifier `Open` pour retourner `(writeDB, readDB *sql.DB, error)` :
- `writeDB` : config actuelle inchangée (`MaxOpenConns=1`, WAL, FK, busy_timeout=5000)
- `readDB` : nouvelle connexion **en lecture seule** — ajouter `&mode=ro` au DSN, `MaxOpenConns=4`
- `Migrate` reste inchangé (prend un `*sql.DB`, appelé avec `writeDB`)

```go
func Open(dsn string) (writeDB, readDB *sql.DB, err error) {
    base := dsn + "?_journal_mode=WAL&_foreign_keys=on&_busy_timeout=5000"
    
    writeDB, err = sql.Open("sqlite3", base)
    // ... MaxOpenConns(1), Ping
    
    readDB, err = sql.Open("sqlite3", base+"&mode=ro")
    // ... MaxOpenConns(4), Ping
    
    return writeDB, readDB, nil
}
```

## Step 2 — `cmd/serve.go` : propager les 2 connexions

**Fichier : `cmd/serve.go`**

```go
writeDB, readDB, err := database.Open(dbPath)
defer writeDB.Close()
defer readDB.Close()
database.Migrate(writeDB)
```

- Repos du **background service** (l.44-48) : `writeDB` (inchangé, juste renommer `db` → `writeDB`)
- Repos du **task worker** (l.88) : `writeDB`
- `TitleService` (l.78) : `writeDB`
- Passer `writeDB` ET `readDB` au router : `router.New(ctx, cfg, writeDB, readDB, distFS, bgSvc, pipeline)`

## Step 3 — Router : utiliser `readDB` pour les lectures

**Fichier : `internal/router/router.go`**

Signature : `func New(ctx, cfg, writeDB, readDB *sql.DB, distFS, bgSvc, pipeline)`

Créer les repos :
- `titleReadRepo := repository.NewTitleRepository(readDB)` — pour List, GetByID, stats-counts
- `titleRepo := repository.NewTitleRepository(writeDB)` — pour Create, Update, Merge, Rematch
- `statsRepo := repository.NewStatsRepository(readDB)` — lecture seule
- Tous les autres repos restent sur `writeDB` (épisodes, seasons, events, settings, tasks font des écritures)

Passer les 2 au TitleHandler :
```go
titles := handler.NewTitleHandler(writeDB, titleRepo, titleReadRepo, seasonRepo, episodeRepo, eventRepo, taskRepo, pipeline, titleSvc)
```

## Step 4 — TitleHandler : champ `titlesRead`

**Fichier : `internal/handler/title.go`**

Ajouter un champ `titlesRead *repository.TitleRepository` à la struct :

```go
type TitleHandler struct {
    db         *sql.DB
    titles     *repository.TitleRepository     // writeDB — pour Create, Update
    titlesRead *repository.TitleRepository     // readDB — pour List, GetByID
    // ... reste inchangé
}
```

Mettre à jour `NewTitleHandler` pour accepter `titlesRead`.

Utiliser `h.titlesRead` dans :
- `List` (l.90, 101, 136, 147) — `h.titlesRead.FindByExternalID`, `h.titlesRead.List`, `h.titlesRead.GetStatusCounts`
- `GetByID` (l.163) — `h.titlesRead.GetByID`
- `Resolve` — `h.titlesRead.FindByExternalID`

Garder `h.titles` (writeDB) dans :
- `Create` (l.221, 226) — `h.titles.Create` + `h.titles.GetByID` (read-after-write consistency)
- `Update` (l.257, 261) — `h.titles.Update` + `h.titles.GetByID` (read-after-write consistency)
- `Rematch` (l.290) — `h.titles.GetByID` (read-after-write)
- `Merge` — idem

## Step 5 — Vérification

1. `make build` — compile
2. `make test` — tous les tests passent (les tests utilisent une seule DB en mémoire, non impactés)
3. `make lint`
4. Déployer → les requêtes `/api/titles` répondent même pendant un background refresh
