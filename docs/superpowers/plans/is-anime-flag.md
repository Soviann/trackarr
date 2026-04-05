# Flag `is_anime` — enrichissement AniList pour les films anime

## Contexte

Les films anime (ex : "Your Name", "Suzume") arrivant de Plex sont traités comme des films classiques et ne reçoivent que les métadonnées TMDB. On veut qu'un film puisse aussi être un anime, pour bénéficier des métadonnées AniList (note, noms romaji, couverture fallback) sans perdre son identité de film.

## Comportement attendu

- Un nouveau champ `is_anime` (booléen) indique si un titre est un anime, indépendamment de son type (`movie` ou `series`).
- Le type `anime` disparaît : les anciens titres `type=anime` deviennent `type=series` + `is_anime=true`.
- Un film anime = `type=movie` + `is_anime=true`. Il apparaît dans le filtre "Anime" ET dans le filtre "Films".
- L'enrichissement AniList se déclenche pour tout titre avec `is_anime=true`, quel que soit son type.
- Le routage TMDB reste basé sur le type (`movie` → API TMDB films, `series` → API TMDB TV).

## Flux utilisateur

1. **Nouveau scrobble film anime** — "Your Name" arrive de Plex comme `movie` → TMDB le trouve → enrichissement AniList détecte un anime → `is_anime=true` → le titre a sa note AniList et son nom romaji.
2. **Film existant** — Le rafraîchissement métadonnées peut enrichir un film existant via AniList.
3. **Filtres** — Le filtre "Anime" montre séries anime + films anime. Le filtre "Films" montre tous les films (y compris anime). Le filtre "Séries" montre les séries non-anime.
4. **Pas de faux positifs** — La recherche AniList ne reclassifie pas un film live-action en anime.

## Écrans impactés

- **Filtres** : le filtre "Anime" devient un filtre transversal (indépendant du type). UX à définir : soit un filtre séparé, soit le filtre type montre `Films | Séries | Anime` où "Anime" filtre sur `is_anime=true`.
- **Page détail** : badge "Anime" visible sur les films anime. Pas de changement structurel, les métadonnées AniList s'affichent déjà.
- **Stats** : `total_anime` = COUNT WHERE `is_anime=true`.

## Implémentation

### Étape 1 — Migration DB + modèle Go

**Fichiers :** nouvelle migration `internal/database/migrations/00N_is_anime.{up,down}.sql`, `internal/model/title.go`

- Ajouter colonne `is_anime INTEGER NOT NULL DEFAULT 0`
- Migrer : `UPDATE titles SET is_anime = 1 WHERE type = 'anime'`
- Puis : `UPDATE titles SET type = 'series' WHERE type = 'anime'`
- Modifier CHECK constraint : `type IN ('movie', 'series')` (supprimer `'anime'`)
- Ajouter index `idx_titles_is_anime`
- Modèle Go : ajouter `IsAnime bool` au struct `Title`, supprimer `TitleTypeAnime`

### Étape 2 — Repository

**Fichiers :** `internal/repository/title.go`, `title_search.go`, `stats.go`

- Ajouter `IsAnime` dans `TitleFilter` et `TitleUpdate`
- INSERT/UPDATE : inclure `is_anime`
- SELECT : scanner `is_anime`
- Stats : `total_anime` = `SUM(CASE WHEN is_anime = 1 ...)`
- Requêtes `WHERE type IN ('series', 'anime')` → `WHERE type = 'series' OR is_anime = 1` (selon le contexte)

### Étape 3 — Pipeline de matching

**Fichier :** `internal/service/matching/pipeline.go`, `anilist_search.go`

- `MatchResult` : ajouter `IsAnime bool`, supprimer usage de `TitleTypeAnime`
- `verifyAndEnrich()` : tenter la recherche AniList pour TOUS les types (pas seulement séries), si AniList trouve → `IsAnime = true`
- Step 4 (recherche AniList directe) : ouvrir aux films aussi
- Exposer le champ `Format` des résultats AniList pour distinguer anime de non-anime
- Conversion `series→anime` devient `IsAnime = true` (sans changer le type)

### Étape 4 — Services

**Fichiers :** `internal/service/plex.go`, `background.go`, `taskqueue.go`, `simkl.go`

- Plex : ne plus comparer à `TitleTypeAnime`, utiliser `IsAnime`
- Background : enrichissement AniList si `is_anime` ou si AniList ID présent
- TaskQueue : idem
- Simkl : importer avec `is_anime=true` au lieu de `type=anime`

### Étape 5 — Handler + API

**Fichier :** `internal/handler/title.go`

- Exposer `is_anime` dans les réponses JSON (déjà via le struct)
- Accepter `is_anime` dans les filtres de liste (`?is_anime=true`)
- Create/Update : accepter `is_anime`

### Étape 6 — Frontend

**Fichiers :** `frontend/src/types.ts`, `FilterDrawer.tsx`, `TitleDetail.tsx`, composants divers

- `TitleType` = `'movie' | 'series'` (supprimer `'anime'`)
- Ajouter `is_anime: boolean` à l'interface `Title`
- FilterDrawer : le chip "Anime" filtre sur `is_anime=true` (filtre séparé ou intégré au filtre type)
- Adapter les conditions `type === 'anime'` → `is_anime`
- Stats : adapter `total_anime`

### Étape 7 — Tests

- Mettre à jour les tests unitaires existants (repository, pipeline, handlers)
- Ajouter des cas : film anime, série anime, film non-anime
- `make test && make test-front`

### Vérification

- `make test && make test-front && make lint`
- Chrome DevTools MCP : vérifier qu'un film anime affiche ses métadonnées AniList
- Vérifier que les filtres fonctionnent (anime montre films + séries anime)
- Vérifier qu'un film live-action n'est pas marqué anime
- Vérifier les stats
