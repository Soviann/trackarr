# Instant episode toggle and rating

## Context

Aujourd'hui, sur la fiche d'un titre (`/title/:id`), cocher un épisode ou enregistrer une note fait disparaître toute la page et la remplace une fraction de seconde par « Loading... » avant de la re-rendre. C'est désagréable, ça perd la position de scroll, et c'est inutile : le backend renvoie déjà le titre complet à jour dans la réponse de l'action.

L'objectif est de faire en sorte que **cocher un épisode et noter un titre soient instantanés et invisibles côté navigation** : la coche se peint immédiatement, la note s'affiche immédiatement, la page ne flashe pas, le scroll ne bouge pas, et tout ce qui dérive (progression de la saison, total visionné, prochaine épisode dans le drawer) se met à jour sans coupure.

## Expérience cible

### 1. Cocher / décocher un épisode (liste d'épisodes)
- Au clic, la coche s'inverse **instantanément** (pas d'attente du serveur).
- La barre de progression de la saison se met à jour immédiatement (`X of Y episodes watched`).
- Le titre du « prochain épisode » dans le bouton « Mark next » du drawer se met à jour immédiatement.
- Si le backfill côté serveur a aussi marqué les épisodes précédents (cas où on saute en avant), ces épisodes apparaissent cochés dès le retour de la requête, sans flash.
- Si la requête échoue (rare, hors ligne) : la coche revient à son état initial et un bandeau d'erreur apparaît.

### 2. Noter un titre (RatingPrompt)
- À la validation, la note (« 8/10 ») apparaît immédiatement dans la carte « My rating », la sheet se ferme.
- Aucune zone de la page ne disparaît pendant la sauvegarde.
- Si l'option « Save & open IMDb » est utilisée, le comportement reste identique côté UX (note instantanée + onglet IMDb ouvert).

### 3. Cas connexes inclus dans la même intervention
Pour rester cohérent, **trois autres actions sur cette page utilisent aujourd'hui le même mécanisme de flash** et doivent être traitées en même temps (l'effort marginal est nul) :
- « Mark next » dans l'ActionDrawer (raccourci pour cocher l'épisode suivant)
- Édition du type / statut (EditSheet)
- Confirmation d'un rematch (RematchSheet `onDone`)

Toutes ces actions doivent suivre la même règle : la page ne flashe plus, l'état se met à jour à partir de la réponse serveur (ou par re-fetch silencieux quand la réponse n'inclut pas le nouvel état du titre).

## Hors-périmètre

- La page Library (liste des titres) — pas concernée par cette demande.
- Les actions par lot (`batch-watch`, `batch-status`, `batch-delete`) — pas évoquées par l'utilisateur.
- L'animation visuelle « micro-feedback » de la coche (ex. petit rebond) — peut faire l'objet d'un plan séparé si besoin.

## Approche technique (pour implémentation)

Trois changements minimaux, tous dans le frontend (le backend renvoie déjà le titre à jour dans `PATCH /titles/:id` et `PATCH /titles/:id/episodes/:eid`) :

1. **Étendre `useApi`** (`frontend/src/hooks/useApi.ts`) pour exposer un `setData` permettant d'injecter la réponse d'une mutation directement dans l'état, sans relancer un GET ni passer par `loading: true`. Conserver `mutate` (re-fetch silencieux) pour les cas où la réponse de la mutation n'inclut pas le nouvel état complet.
   - Optionnellement, séparer `loading` (initial) de `revalidating` (refetch) pour ne plus jamais effacer l'écran après le premier chargement — c'est la racine du flash.

2. **Adapter `EpisodeRow`** (`frontend/src/components/EpisodeRow.tsx`) :
   - Patch optimiste : inverser `episode.watched` localement avant la requête (via une prop `onOptimisticToggle` ou en remontant la mise à jour au parent).
   - Utiliser la réponse `PATCH` (titre complet à jour, incluant le backfill éventuel) pour écraser l'état du titre côté parent — réutilise le `setData` ajouté à `useApi`.
   - Rollback sur erreur + bannière (l'`ErrorBanner` existe déjà dans `TitleDetail`).

3. **Adapter `TitleDetail`** (`frontend/src/pages/TitleDetail.tsx`) :
   - `handleSaveRating`, `handleMarkNext`, `handleSaveEdit` : récupérer la réponse du `apiFetch` et la passer à `setData` au lieu d'appeler `mutate()`.
   - `RematchSheet onDone` : conserver `mutate()` (le rematch ne renvoie pas le titre complet) mais s'appuyer sur la séparation `loading` / `revalidating` pour ne plus afficher « Loading... » pendant le re-fetch.
   - Passer le `setData` (ou un callback `onTitleUpdate(title)`) à `EpisodeRow` à la place du `onToggle: mutate`.

### Fichiers à modifier

| Fichier | Nature |
|---|---|
| `frontend/src/hooks/useApi.ts` | Ajouter `setData` et séparation `loading`/`revalidating` |
| `frontend/src/components/EpisodeRow.tsx` | Patch optimiste + utilisation de la réponse PATCH |
| `frontend/src/pages/TitleDetail.tsx` | `setData` au lieu de `mutate` pour les 3 actions concernées, rendu non-bloquant pendant revalidation |

Aucune modification backend nécessaire — `PATCH /titles/:titleID/episodes/:episodeID` (`internal/handler/episode.go:58`) et `PATCH /titles/:id` (`internal/handler/title.go:325`) renvoient déjà le titre complet à jour.

## Vérification (visuelle, en navigateur)

Sur `cmux browser surface:32` contre `http://localhost:8080` (rappel : `make dev-frontend` doit tourner ; après modif Go ou pour rebuster le service worker, `?t=$(date +%s)` sur l'URL) :

1. Aller sur la fiche d'une série multi-saisons avec plusieurs épisodes non vus.
2. Scroller en bas de la liste d'épisodes et cocher un épisode au milieu.
   - **Attendu** : la coche s'inverse instantanément, la barre de progression se met à jour, **aucune disparition de la page**, le scroll reste exactement là où il était.
3. Décocher le même épisode.
   - **Attendu** : idem, instantané.
4. Cocher un épisode bien plus loin que ce qu'on avait vu (déclenche le backfill serveur).
   - **Attendu** : la coche cliquée apparaît immédiatement ; les épisodes précédents apparaissent cochés au retour de la requête (200-500 ms), sans flash.
5. Ouvrir l'ActionDrawer → « Mark next ».
   - **Attendu** : la fiche reste affichée, le « next » dans le drawer avance d'un cran.
6. Ouvrir l'ActionDrawer → « Rate », mettre 8/10, valider.
   - **Attendu** : la sheet se ferme, « My rating » affiche 8/10 sans aucun flash de la page.
7. Ouvrir l'ActionDrawer → « Edit » → changer le statut → valider.
   - **Attendu** : statut mis à jour sans flash.
8. Ouvrir la console pendant les étapes 2-7 — aucune erreur.
9. Test offline / erreur (couper le réseau dans devtools) : cocher un épisode.
   - **Attendu** : la coche revient à l'état initial, l'`ErrorBanner` s'affiche.
10. Régression : page Library inchangée, recherche inchangée, page Stats inchangée.

Tests automatiques à lancer : `make test-front` (vitest sur `useApi`, `EpisodeRow`, `TitleDetail`) et `make lint` avant commit.
