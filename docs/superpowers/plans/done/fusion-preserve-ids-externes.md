# Fusion — Préservation des IDs externes + rattrapage des relectures Plex

## PO summary

Quand tu fusionnes deux fiches qui pointent vers la même œuvre (par exemple « Hercule Poirot » dupliqué entre Simkl et Plex), la fiche conservée récupère désormais les identifiants externes de celle qu'on supprime — seulement s'ils manquaient. Conséquence concrète : après la fusion, les rediffusions suivantes sur Plex tombent sur la bonne fiche et le duplicata ne se recrée plus. En parallèle, si un webhook Plex `media.play` arrive sur un épisode que l'app ignorait (cas de rattrapage après une panne de scrobble), l'épisode est désormais marqué vu au lieu d'être silencieusement jeté.

## Contexte

Deux bugs observés en prod sur `Hercule Poirot` (titres `5994` Simkl ↔ `7570` Plex) :

1. **Fusion perd les IDs externes.** `mergeInTx` (`internal/repository/title.go:885`) transfère saisons/épisodes/noms/events mais pas `imdb_id`, `tmdb_id`, `tvdb_id`, `anilist_id`, `plex_rating_key`. Après fusion de `7570` (plex_rating_key=19908) dans `5994` (plex_rating_key NULL), le prochain webhook Plex sur `grandparentRatingKey=19908` ne trouve plus `5994` et recrée un duplicata.
2. **Silent-skip sur `media.play`.** `handleEpisodePlayInTx` (`internal/service/plex.go:150-153`) retourne sans rien faire si `!ep.Watched`, en supposant que `media.scrobble` passera ensuite. Mais un `media.play` peut arriver en premier (rattrapage après redémarrage, scrobble manqué) : l'épisode (ex. S6E2 row 363667) reste non vu pour toujours.

Vérifié : `grandparentRatingKey` est bien la clé de la série dans Plex (code ligne 134, fix commit `2b77e62`), donc le transfert NULL-only du `plex_rating_key` rétablit correctement le lien futur.

## Phase 1 — `mergeInTx` transfère les IDs externes NULL-only `[seq]`

**Cible :** `internal/repository/title.go:885-970` (fonction `mergeInTx`).

**Modification :** avant la section `// 4. Delete source title`, ajouter une étape qui fait `UPDATE titles SET col = source.col WHERE id = destID AND destID.col IS NULL` pour chacune des cinq colonnes : `imdb_id`, `tmdb_id`, `tvdb_id`, `anilist_id`, `plex_rating_key`.

**Comportement attendu :**
- Si la dest a déjà une valeur non-NULL → ne pas écraser (protège contre la propagation d'IDs pollués).
- Si la dest est NULL et la source est NULL → no-op.
- Si la dest est NULL et la source a une valeur → copier.

**Implémentation recommandée :** une seule requête SQL qui fait les 5 colonnes en un passage, via corrélation sur le titre source :

```sql
UPDATE titles SET
    imdb_id         = COALESCE(imdb_id,         (SELECT imdb_id         FROM titles WHERE id = ?)),
    tmdb_id         = COALESCE(tmdb_id,         (SELECT tmdb_id         FROM titles WHERE id = ?)),
    tvdb_id         = COALESCE(tvdb_id,         (SELECT tvdb_id         FROM titles WHERE id = ?)),
    anilist_id      = COALESCE(anilist_id,      (SELECT anilist_id      FROM titles WHERE id = ?)),
    plex_rating_key = COALESCE(plex_rating_key, (SELECT plex_rating_key FROM titles WHERE id = ?))
WHERE id = ?
```

Args : `sourceID × 5, destID`. Doit être exécuté **avant** le `DELETE FROM titles WHERE id = sourceID`.

**Tests à ajouter** dans `internal/repository/title_test.go` :

- `TestTitleRepository_Merge_TransfersMissingExternalIDs` : dest a tous les IDs à NULL, source a des valeurs → après merge, dest récupère les valeurs source.
- `TestTitleRepository_Merge_PreservesExistingExternalIDs` : dest a déjà `imdb_id="X"`, source a `imdb_id="Y"` → après merge, dest garde `"X"` (pas écrasé). Valider la même règle pour au moins deux autres colonnes (`tmdb_id`, `plex_rating_key`).
- `TestTitleRepository_Merge_DeletesSource` : sanity — le titre source n'existe plus après merge (couvre le cas de régression où l'UPDATE échouerait silencieusement).

Tests utilisent `setupTestDB` existant (`title_test.go:16`).

## Phase 2 — `handleEpisodePlayInTx` marque l'épisode vu en rattrapage `[seq]`

**Cible :** `internal/service/plex.go:127-174` (fonction `handleEpisodePlayInTx`).

**Modification :**
- Remplacer le `if !ep.Watched { return nil }` (lignes 150-153) par : `if !ep.Watched { episodes.MarkWatched(ep.ID, now) }` avec `now := time.Now().UTC()` hissé avant la condition.
- Conserver la création du `watch_event` et les `UpdateLastWatchedAt` pour épisode et titre qui suivent — ils s'appliquent à la fois au rattrapage et à la relecture.
- Adapter le commentaire : `// media.play on unwatched episode — catch-up (media.scrobble missed)`.

**Signature inchangée.** Utilise `episodes.MarkWatched` existant (`internal/repository/episode.go:162`) qui renseigne `first_watched_at` si NULL et `last_watched_at`.

**Tests à ajouter** dans `internal/service/plex_test.go` :

- `TestHandleEpisodePlay_UnwatchedEpisode_MarksWatched` : épisode existe en DB avec `watched=0`, envoyer un `media.play` → l'épisode devient `watched=1` avec `first_watched_at` et `last_watched_at` renseignés, et un `watch_event` est créé.
- Conserver / compléter le test existant de rebalayage sur épisode déjà vu (si présent) pour garantir la non-régression du comportement de relecture.

## Phase 3 — Vérification `[seq]`

1. `make test` — nouveaux tests verts, aucune régression.
2. `make lint` — zéro warning sur les fichiers modifiés.
3. Test manuel sur la DB prod pullée localement :
   - `cp /tmp/plextracker_before_merge.db data/plextracker.db` (restaure l'état avant fusion)
   - `make up`, login dev (`POST /api/auth/dev`)
   - Déclencher la fusion `7570 → 5994` (UI ou endpoint) avec `season_offset=0` puis réessayer avec `=2` sur backup restauré
   - Vérifier via `sqlite3` : `SELECT imdb_id, tmdb_id, tvdb_id, plex_rating_key FROM titles WHERE id=5994;` → les champs qui étaient NULL doivent contenir les valeurs de `7570` (en particulier `plex_rating_key=19908`), les autres doivent rester aux valeurs Simkl originales.
4. Vérification visuelle Chrome DevTools MCP sur la page détail de Poirot : `last_watched_at` s'affiche, rediffusions listées.

## Fichiers critiques

- `internal/repository/title.go:885` — ajout du transfert NULL-only dans `mergeInTx`.
- `internal/repository/title_test.go` — 3 nouveaux tests.
- `internal/service/plex.go:127` — logique catch-up dans `handleEpisodePlayInTx`.
- `internal/service/plex_test.go` — 1 nouveau test.
- `CHANGELOG.md` — ligne sous `## [Unreleased]` : `fix(fusion): préserve les IDs externes manquants sur la fiche conservée` + `fix(plex): marque l'épisode vu quand media.play arrive avant media.scrobble`.

## Hors-scope

- Nettoyage des données prod existantes (titre `5994` sans `plex_rating_key`, épisode 363667 non vu) : sera corrigé par une nouvelle fusion via l'UI après déploiement, plus le prochain `media.play`. Le réimport Simkl prévu en mai 2026 couvre toute incohérence résiduelle.
- Aucune migration DB nécessaire — le schéma actuel suffit.
