# Intégration TheTVDB (v4)

## Contexte

TMDB couvre très bien les films et les grandes séries mainstream. TheTVDB est une base complémentaire historique, particulièrement riche sur les séries de niche, l'anime (avec des ordres d'épisode spécifiques), et certains catalogues internationaux mal servis par TMDB. Une clef `TVDB_API_KEY` a été ajoutée au `.env.local`.

L'objectif : interroger TheTVDB **en parallèle** de TMDB à chaque import/refresh, puis **fusionner intelligemment** les deux sources (et AniList quand applicable) pour aboutir à la fiche la plus complète et la plus fiable possible. Les données d'une source confirment, infirment ou complètent les autres.

Pas de nouvelle interface de recherche TVDB : la fonctionnalité doit rester invisible pour l'utilisateur dans le flux normal. Elle se matérialise uniquement par des fiches plus riches, un champ "TVDB ID" dans la feuille Rematch, et la prise en charge des URLs `thetvdb.com` dans l'Add (et donc dans la cible de partage PWA qui pointe déjà sur l'Add).

## Ce qui change pour l'utilisateur

### Import et refresh (automatique, invisible)
- À chaque import de backup Simkl ou Refresh sur un titre, PlexTracker interroge maintenant TMDB **et** TheTVDB en parallèle, puis fusionne.
- Résultat visible : plus de titres avec affiche, synopsis plus complets, noms multilingues plus nombreux, et une nouvelle **note TVDB** affichée à côté des notes TMDB et AniList.
- Les titres déjà en base qui n'avaient pas de `tvdb_id` le récupèrent au prochain Refresh.

### Fiche titre
- Nouveau badge **note TVDB** (couleur distincte) à côté des badges note TMDB et note AniList.
- Nouveau lien externe **TVDB** dans la zone des liens (IMDb, AniList…) quand un `tvdb_id` est connu.
- Aucun autre changement visuel : le synopsis, les genres, la durée, les noms multilingues peuvent désormais venir de TVDB quand TMDB ne les avait pas, mais l'affichage reste identique.

### Feuille "Fix match" (Rematch)
- Dans la section dépliable **Manual IDs**, ajout d'un champ **TVDB ID** sous les champs TMDB / IMDB / AniList.
- Coller un ID TVDB puis "Save & re-enrich" met à jour le titre et relance l'enrichissement complet (TMDB + TVDB + AniList le cas échéant).
- **Pas de recherche TVDB** dans l'UI — l'utilisateur saisit l'ID ou utilise une URL via le champ Add.

### Page "Add" et partage PWA
- Coller une URL `thetvdb.com` dans le champ Add résout automatiquement le titre :
  - `https://thetvdb.com/series/<slug>` → série
  - `https://thetvdb.com/movies/<slug>` → film
- La cible de partage PWA (share target) passe par le même endpoint que le champ Add. Partager un lien TheTVDB depuis le navigateur mobile vers l'icône PlexTracker fonctionne donc automatiquement de la même façon (pas d'écran supplémentaire à concevoir).
- Les URLs contiennent des slugs, pas d'IDs numériques. PlexTracker les résout en interrogeant TVDB (`/series/slug/{slug}` ou `/movies/slug/{slug}`).

## Règles de fusion des données (décisions PO)

Quand TMDB et TVDB répondent tous les deux pour un même titre, voici comment PlexTracker combine les champs :

| Champ | Règle de fusion |
|---|---|
| Nom principal (affiché en gros) | TMDB prioritaire ; TVDB si TMDB vide |
| Noms multilingues (FR, EN, …) | **Union** des deux sources, dédupliqués par langue (TMDB gagne en cas de doublon exact sur une même langue) |
| Affiche / cover | TMDB prioritaire ; TVDB si TMDB vide ; AniList en dernier recours (inchangé par rapport à aujourd'hui) |
| Synopsis (overview) | La version **la plus longue et la plus complète** des deux (TMDB si égalité) |
| Genres | **Union** des deux listes, dédupliqués (insensible à la casse) |
| Durée d'épisode / film | TMDB prioritaire ; TVDB si TMDB vide |
| Note TMDB | Affichée telle quelle (inchangé) |
| **Note TVDB** | **Nouveau** — stockée dans une nouvelle colonne `tvdb_rating`, affichée en badge à côté |
| Note AniList | Inchangé |
| Date de sortie / première diffusion | TMDB prioritaire ; TVDB si TMDB vide |
| IDs externes (IMDb, …) | On complète les champs manquants avec ce que TVDB fournit (cross-référencement) |
| Type (film / série) | TMDB prioritaire ; divergence → TMDB gagne |
| Flag `is_anime` | Confirmation : si AniList OU genre TVDB contient "Anime" / "Animation" → `is_anime = true` |

**Divergences fortes** : si TMDB et TVDB donnent des années de sortie qui diffèrent de plus d'un an, on privilégie TMDB et on log l'écart côté serveur pour audit. Pas de signalement visible à l'utilisateur dans cette première version.

**Dégradation gracieuse** : si TVDB est indisponible ou que la clef est manquante, tout retombe sur le comportement actuel (TMDB seul) sans erreur visible.

## Critères d'acceptation

1. Avec `TVDB_API_KEY` valide, l'app démarre en se connectant à TVDB et log `TVDB client ready` dans les logs serveur.
2. Clef absente ou invalide → warning dans les logs, l'app démarre normalement sans TVDB.
3. Rafraîchir un titre existant qui n'avait pas de `tvdb_id` → après Refresh, il a récupéré `tvdb_id` + `tvdb_rating` (quand TVDB connaît le titre). Badge note TVDB visible sur la fiche.
4. Importer un backup Simkl contenant des films et des séries → les titres importés sortent avec TMDB **et** TVDB renseignés quand les deux sources connaissent le titre.
5. Un titre dont TMDB n'a pas de cover récupère le cover TVDB (visible dans la grille Library).
6. Un titre dont TMDB n'a pas de synopsis reçoit le synopsis TVDB sur la fiche.
7. Saisir un ID TVDB dans la feuille Rematch → Manual IDs → Save & re-enrich → le titre est re-matché et les trois sources sont re-fetchées.
8. Coller `https://thetvdb.com/series/<slug>` dans le champ Add → le bon titre est détecté et prérempli comme série.
9. Coller `https://thetvdb.com/movies/<slug>` dans le champ Add → détecté comme film.
10. Partager un lien `thetvdb.com` depuis Chrome Android / Safari iOS vers l'icône PWA PlexTracker → même résultat que l'Add.
11. Un titre anime (ex : "Frieren") déjà matché via AniList reçoit également son `tvdb_id` + `tvdb_rating` au Refresh (trois sources coexistent).
12. Si TVDB tombe en panne (timeout, 5xx) pendant un Refresh → le pipeline continue avec TMDB seul, warning dans les logs, aucune erreur bloquante visible.
13. Les tests backend (`make test`) et frontend (`make test-front`) passent.

## Fichiers principaux impactés

**Nouveaux**
- `internal/service/matching/tvdb.go` — client TVDB, auth JWT cachée, méthode `get()` partagée
- `internal/service/matching/tvdb_search.go` — recherche par titre/année (utilisable en fallback automatique si TMDB ne trouve rien ; pas exposée en UI)
- `internal/service/matching/tvdb_details.go` — `GetSeriesDetails`, `GetMovieDetails`, `GetSeriesBySlug`, `GetMovieBySlug`, extraction genres/runtime/rating/noms multilingues
- `internal/service/matching/tvdb_covers.go` — `DownloadCover`
- `internal/service/matching/tvdb_test.go` — tests client avec serveur HTTP mock (même pattern que `tmdb_test.go`)
- `internal/database/migrations/NNN_tvdb_rating.up.sql` + `.down.sql` — colonne `tvdb_rating INTEGER`

**Modifiés**
- `internal/config/config.go` — nouvelle clef `TVDBAPIKey` (lue depuis `TVDB_API_KEY`)
- `cmd/serve.go` — instanciation `tvdbClient` et injection dans le pipeline
- `internal/service/matching/pipeline.go` — champ `tvdb *TVDBClient`, fetch parallèle dans `enrichFromIDs`, fonctions de fusion pour chaque champ (inline — pas d'abstraction prématurée à 2 sources)
- `internal/service/matching/urls.go` + `urls_test.go` — regex `thetvdb.com/(series|movies)/<slug>` + résolution slug→ID dans `ResolveURL` via appel TVDB
- `internal/model/title.go` — champ `TVDBRating *int`
- `internal/repository/title.go` — `TitleUpdate` accepte `TVDBRating`, `Update` le persiste
- `internal/handler/title.go` — endpoint rematch accepte `tvdb_id` dans le body
- `internal/handler/settings.go` — drapeau `tvdb_connected` (optionnel, cohérence avec `anilist_connected`)
- `frontend/src/components/RematchSheet.tsx` — nouveau champ `manualTvdb`
- `frontend/src/pages/TitleDetail.tsx` — badge note TVDB + lien externe TVDB
- `frontend/src/types.ts` — champ `tvdb_rating` dans `Title` / `MatchResult`
- `CHANGELOG.md` — entrée `## [Unreleased] — feat` pour l'intégration TVDB
- `docs/patterns.md` — section provider TVDB (pattern client + fusion)
- `docs/user-guide.md` — mention de la note TVDB et des URLs TheTVDB reconnues

## Vérification (à exécuter après implémentation)

1. `make lint && make test` — `tvdb_test.go`, `urls_test.go`, `pipeline_test.go` passent.
2. `make test-front` — build TS + tests frontend verts.
3. `make build` — build Docker réussit.
4. `make up` puis `make dev-frontend`, login avec les credentials `DEBUG_LOGIN*` de `.env.local`.
5. **Flux Refresh** : ouvrir un titre connu de TVDB (ex. Breaking Bad), appuyer sur Refresh depuis l'ActionDrawer, vérifier dans l'onglet Network de DevTools que l'appel TVDB a bien lieu, rafraîchir la fiche et vérifier la présence du badge note TVDB.
6. **Flux Rematch manuel** : ouvrir Fix match → Manual IDs → coller un `tvdb_id` valide → Save & re-enrich → vérifier que la fiche est mise à jour et que les autres IDs (TMDB/IMDB) sont cross-référencés quand TVDB les fournit.
7. **Flux URL Add** : coller `https://thetvdb.com/series/frieren-beyond-journeys-end` (ou équivalent) dans le champ Add, vérifier que la résolution fonctionne et que le type détecté est "série".
8. **Flux URL film** : idem avec une URL `/movies/<slug>`, vérifier type "film".
9. **Partage PWA** : sur mobile, partager une page `thetvdb.com` vers l'icône PlexTracker installée, vérifier que le champ Add est prérempli.
10. **Fusion de cover** : forcer un titre où TMDB n'a pas de cover. Vérifier que le cover TVDB prend le relais dans la grille Library.
11. **Dégradation gracieuse** : commenter `TVDB_API_KEY` dans `.env.local`, relancer `make up`, vérifier warning dans les logs et fonctionnement normal TMDB seul. Aucun titre affiché cassé.
12. **Console JS** : aucune erreur / warning rouge sur toutes les pages touchées (Library, TitleDetail, Add, Rematch).
13. **Régression** : naviguer rapidement sur d'autres pages (Settings, Match Review, Validate) pour vérifier l'absence de régression.

## Notes techniques (scope estimation)

<details>

- **Auth TheTVDB v4** : `POST /login` avec `{apikey}` retourne un JWT valide ~30 jours. Le client doit cacher le token en mémoire et re-login automatiquement en cas de 401. Pas de PIN utilisateur pour l'API gratuite. Base URL : `https://api4.thetvdb.com/v4`.
- **Rate limiting** : appliquer le même token bucket que TMDB (2 req/s, burst 1) — TVDB ne publie pas de limites dures. Réutiliser `APILimiter` existant dans `BackgroundService` / `TaskQueueWorker`.
- **Résolution de slug** : les URLs TVDB contiennent des slugs, pas d'IDs. `urls.go::ParseURL` reste pur (ne fait que du regex et retourne le slug brut dans un nouveau champ `ExternalIDs.TVDBSlug` ou équivalent) ; la conversion slug→ID se fait dans `Pipeline.ResolveURL` avec un appel TVDB supplémentaire (`GetSeriesBySlug` / `GetMovieBySlug`). Gracieux si TVDB client est nil → retour d'erreur "URL TVDB non résolvable" exposée à l'utilisateur.
- **Fetch parallèle dans `enrichFromIDs`** : un `sync.WaitGroup` avec deux goroutines (TMDB et TVDB), `context.WithTimeout` raisonnable (~10s), mutex si nécessaire sur le `result` ou séparation en structures temporaires fusionnées après. AniList reste séquentiel après (dépend du type résolu).
- **Migration** : `ALTER TABLE titles ADD COLUMN tvdb_rating INTEGER`. Pas de nouvel index nécessaire — `tvdb_id` est déjà indexé depuis la migration 001.
- **Noms multilingues TVDB** : l'endpoint `/series/{id}/translations/{lang}` renvoie titre/overview par langue. Appeler en + fr comme pour TMDB.
- **Covers TVDB** : URLs retournées directement par l'API (format `https://artworks.thetvdb.com/...`). Download + stockage dans `{dataDir}/covers/` comme les autres sources, avec un préfixe de nom de fichier pour éviter les collisions (`tvdb_<id>.jpg`).
- **PWA share target** : vérifier `frontend/public/manifest.json` → le share_target pointe sur une route qui appelle `/titles/resolve` → qui appelle `pipeline.ResolveURL`. L'ajout du parsing TVDB dans `urls.go` + la résolution slug→ID couvrent donc automatiquement le share target sans changement UI.
- **Pas d'abstraction prématurée** : avec seulement 2 sources qui fusionnent systématiquement (TMDB+TVDB), pas de `MetadataMerger` générique. Fusion inline, lisible, par champ. Si un jour on ajoute une 3e source au même niveau, refactor à ce moment-là (règle DRY de CLAUDE.md : extraire à 3+ occurrences).
- **Tests** : mock HTTP server pour `tvdb_test.go` (pattern identique à `tmdb_test.go`), fixtures JSON minimales couvrant `/login`, `/series/{id}`, `/movies/{id}`, `/series/slug/{slug}`, `/movies/slug/{slug}`, `/series/{id}/translations/{lang}`, `/search`. Test de la fusion dans `pipeline_test.go` avec TMDB et TVDB mocks qui retournent des données partiellement contradictoires pour valider les règles du tableau.
- **Scope estimé** : ~6 nouveaux fichiers backend + 1 migration + ~13 fichiers modifiés (back + front + docs). Une session complète de développement + tests + vérification visuelle.
- **Réf API** : https://thetvdb.github.io/v4-api/

</details>
