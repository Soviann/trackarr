# Flag `is_anime` — enrichissement AniList pour les films anime

## Contexte

Les films anime (ex : "Your Name", "Suzume") arrivant de Plex sont traités comme des films classiques et ne reçoivent que les métadonnées TMDB. On veut qu'un film puisse aussi être un anime, pour bénéficier des métadonnées AniList (note, noms romaji, couverture fallback) sans perdre son identité de film.

Une logique de **fusion automatique** (consolidant les titres splités par saison dans Plex) a été récemment ajoutée et doit être adaptée pour utiliser ce nouveau flag au lieu du type `anime`.

## Comportement attendu

- Un nouveau champ `is_anime` (booléen) indique si un titre est un anime, indépendamment de son type (`movie` ou `series`).
- Le type `anime` disparaît complètement du code et des contraintes DB.
- Un film anime = `type=movie` + `is_anime=true`. Il apparaît dans le filtre "Anime" ET dans le filtre "Films".
- L'enrichissement AniList se déclenche pour tout titre avec `is_anime=true`, quel que soit son type.
- La **fusion intelligente** (via `IMDBID` + Gemini) s'applique à tout titre identifié comme `is_anime`.

## Flux utilisateur

1. **Import Simkl (Primaire)** — L'utilisateur prévoit de vider la base et de ré-importer via Simkl. L'importateur doit mapper les items de la section `anime` vers `movie` ou `series` tout en cochant `is_anime=true`.
2. **Nouveau scrobble Plex** — Détection automatique de l'anime via le pipeline de matching → `is_anime=true`.
3. **Filtres** — Le filtre "Anime" montre séries anime + films anime. Le filtre "Films" montre tous les films (y compris anime).
4. **Fusion** — Si deux titres anime ont le même ID IMDB, ils fusionnent (ex: saisons séparées dans Plex devenant un seul titre avec plusieurs saisons).

## Écrans impactés

- **Filtres** : le chip "Anime" filtre désormais sur `is_anime=true`.
- **Stats** : `total_anime` = `COUNT(*) WHERE is_anime=true`.
- **Page détail** : affichage des scores AniList pour tout titre `is_anime`.

## Implémentation

### Étape 1 — Migration DB + modèle Go

**Fichiers :** `internal/database/migrations/009_is_anime.up.sql`, `internal/model/title.go`

- Ajouter colonne `is_anime INTEGER NOT NULL DEFAULT 0`.
- Ajouter index `idx_titles_is_anime`.
- Modifier la contrainte CHECK de `titles` pour n'autoriser que `('movie', 'series')`.
- Modèle Go : ajouter `IsAnime bool` au struct `Title`, supprimer la constante `TitleTypeAnime`.

### Étape 2 — Repository & Fusion

**Fichiers :** `internal/repository/title.go`, `stats.go`

- Ajouter `IsAnime` dans `TitleFilter`, `TitleUpdate` et les méthodes de sélection/scan.
- **Fusion** : s'assurer que `Merge()` reste agnostique du type mais que l'appelant (TaskQueue) cible les anime.
- **Stats** : mettre à jour `overview()` et `breakdown()` pour utiliser `is_anime` (transversal).

### Étape 3 — Pipeline de matching & Consolidation

**Fichiers :** `internal/service/matching/pipeline.go`, `internal/service/taskqueue.go`

- `MatchResult` : ajouter `IsAnime bool`.
- `Pipeline.Run()` : 
    - Ouvrir la recherche AniList aux films (`TitleTypeMovie`).
    - Si `AniListID != 0`, forcer `IsAnime = true`.
    - Supprimer la conversion forcée du type en `anime`.
- **TaskQueue** : 
    - Modifier `handleEnrichment()` : la logique de fusion (détection de conflit IMDB) doit se déclencher si `result.IsAnime == true` (au lieu de `type == 'anime'`).
    - Maintenir l'appel à Gemini `IdentifyAnimeSeason()` pour le calcul du `seasonOffset`.

### Étape 4 — Import Simkl & Plex

**Fichiers :** `internal/service/simkl.go`, `internal/service/plex.go`

- **Simkl** : dans `Import()`, pour les items de la section `Anime`, mapper le type vers `movie` ou `series` (via `item.AnimeType`) et forcer `IsAnime = true`.
- **Plex** : utiliser `IsAnime` pour décider de l'enrichissement AniList ou des badges UI.

### Étape 5 — Frontend

**Fichiers :** `frontend/src/types.ts`, `FilterDrawer.tsx`, `utils.ts`, `CoverPlaceholder.tsx`

- Supprimer `'anime'` de `TitleType`.
- Ajouter `is_anime: boolean` à l'interface `Title`.
- Adapter `FilterDrawer` : le filtre "Anime" devient indépendant et filtre sur `is_anime=true`.
- Adapter `CoverPlaceholder` : utiliser `is_anime` pour la couleur lavande.

### Étape 6 — Tests

- Mettre à jour `internal/service/fusion_test.go` pour tester la fusion avec le flag `is_anime` au lieu du type.
- Vérifier l'import Simkl avec des anime de type film et série.
- `make test && make test-front`

## Vérification

1. `make db-reset` (local) + Import Simkl pour vérifier la nouvelle structure.
2. Vérifier qu'un film anime (ex: *Your Name*) est bien de type `movie` mais affiche le score AniList.
3. Vérifier qu'une fusion se déclenche si on ajoute deux saisons d'un anime séparément.
4. Vérifier les filtres transversaux.
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
