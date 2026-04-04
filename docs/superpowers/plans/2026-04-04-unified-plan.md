# PlexTracker — Plan unifie (correctifs, UX, refactor)

## Contexte

Fusion des 3 plans existants (`post-audit-fixes`, `ux-ui-audit`, `full-refactor`) en un seul plan ordonne. Apres l'import Simkl de 7 459 titres, un audit PO a revele des bugs et problemes UX. En parallele, un plan de refactoring technique ameliore la maintenabilite. Les taches sont ordonnees pour que les prerequis techniques soient faits avant les ameliorations UX qui en dependent.

**Obsolete :** la tache "Stats Coming soon" de l'audit UX — la page Stats est maintenant implementee.

**A supprimer apres validation :** les 3 anciens plans dans `docs/superpowers/plans/`.

---

## Phase 1 — Correctifs critiques (autonomes)

### T1 — P0 : Corriger le crash JS (episodes null)
21 saisons sur 1 815 ont `episodes: null` dans le JSON. Le frontend crashe sur Library, Search, Detail.

**Comportement attendu :**
- `/api/titles` et `/api/titles/{id}` retournent toujours `"episodes": []` (jamais `null`)
- Library, Search, Detail s'affichent sans erreur console

**Fichiers :** `internal/model/season.go`, `internal/repository/title.go`

### T2 — P1 : Onglet "Watching" par defaut
La Library s'ouvre sur "All" (7 459 titres). L'utilisateur veut voir ses series en cours en premier.

**Comportement attendu :**
- A l'ouverture, l'onglet "Watching" est actif
- Ordre : Watching, Up to date, Completed, Dropped, Plan, All

**Fichiers :** `frontend/src/app.tsx`, `frontend/src/components/FilterBar.tsx`

### T3 — P4 : Desactiver "Confirm" sans identifiant externe
Le bouton "Confirm" est cliquable sur Match Review meme pour un titre sans ID externe.

**Comportement attendu :**
- Titre sans ID : bouton grise, non cliquable
- Titre avec au moins un ID : bouton vert, fonctionnel

**Fichier :** `frontend/src/components/MatchReviewCard.tsx`

---

## Phase 2 — Fondations backend

### T4 — Helpers HTTP (response/request)
Creer le package `internal/handler/httputil/` avec WriteJSON, ReadJSON, ParseIDParam, ParseQueryInt.

**Fichiers :** `internal/handler/httputil/response.go`, `request.go`, + tests

### T5 — Gestion d'erreurs centralisee
APIError type + HandlerFunc wrapper retournant `error` + middleware de conversion.

**Fichiers :** `internal/handler/httputil/errors.go` + tests

### T6 — Migration des handlers vers httputil
Tous les handlers passent de `func(w, r)` a `func(w, r) error` avec helpers httputil. Le router wrappe via `httputil.WrapHandler()`.

**Fichiers :** tous les handlers + `internal/router/router.go`

### T7 — Rate limiter pour services externes
Remplacer `time.Sleep` par `golang.org/x/time/rate` dans BackgroundService.

**Fichiers :** `internal/service/ratelimiter.go`, `internal/service/background.go`

---

## Phase 3 — Fondations frontend

### T8 — Store Zustand pour les titres
Cache centralise avec fetch, invalidation, filtre, pagination.

**Fichier :** `frontend/src/store.ts`

### T9 — Utilitaires partages
Extraire getName, getTypeLabel, getStatusLabel, formatDate, watchedCount, totalEpisodes.

**Fichier :** `frontend/src/utils.ts`

### T10 — Composant ErrorBanner
Banniere d'erreur reutilisable avec bouton "Reessayer".

**Fichiers :** `frontend/src/components/ErrorBanner.tsx` + CSS module

### T11 — Tokens de theme (spacing, radius, font-size)

**Fichier :** `frontend/src/theme.ts`

---

## Phase 4 — Ameliorations UX (dependent des phases 2-3)

### T12 — P1 : Etats d'erreur au lieu de "Loading..." infini
*(Utilise ErrorBanner de T10 + gestion d'erreurs backend de T5)*

**Comportement attendu :**
- En cas d'echec API : message d'erreur + bouton "Reessayer"
- Le "Loading..." ne reste jamais plus de 10s sans feedback
- Un crash JS non gere affiche un ecran de secours

**Criteres :** Library, Search, Detail, Match Review affichent un message d'erreur clair

### T13 — P1 : Corriger la recherche (resultats non pertinents)
*(Inclut l'extraction de la logique de recherche — refactor T11)*

Chercher "Naruto" retourne 4 845 resultats dont aucun n'est Naruto.

**Comportement attendu :**
- Match exact en premier, puis prefixe, puis fuzzy
- Pas plus de ~50 resultats pour un terme precis
- "Breaking Bad" retourne Breaking Bad en premier

**Fichiers :** `internal/repository/title_search.go` (nouveau), `internal/repository/title.go`

### T14 — P2 : Pagination serveur
*(Necessite batch loader — refactor T12, + store Zustand T8)*

**Etapes :**
1. Reponse legere : plus d'episodes dans le listing, compteurs par saison
2. Pagination `limit`/`offset` (defaut 50)
3. Filtre "Up to date" cote serveur
4. Bouton "Charger plus" en frontend

**Comportement attendu :**
- "Watching" charge en < 500 ms
- "All" charge 50 titres puis "Charger plus"
- Detail affiche toujours tous les episodes

**Fichiers :** `internal/repository/title_loader.go` (nouveau), `internal/repository/title.go`, `internal/handler/title.go`, `frontend/src/pages/Library.tsx`, `frontend/src/store.ts`

### T15 — P2 : Placeholder pour titres sans couverture
**Comportement attendu :**
- Carte Library : icone ou fond colore selon le type (film/serie/anime)
- Page Detail : fond degrade + icone au lieu d'un bloc noir
- Le titre reste lisible

### T16 — P2 : Page Match Review accessible directement
**Comportement attendu :**
- `/match-review` s'affiche correctement en navigation directe (bookmark, refresh)
- Si rien a revoir : message "Aucun titre a verifier"

---

## Phase 5 — Ameliorations UX secondaires

### T17 — P3 : Enrichir les metadonnees episodes (nom, date)
L'import Simkl ne fournit pas ces infos. Le job de refresh TMDB les ignore.

**Comportement attendu :**
- Apres un cycle de refresh, les episodes ont nom et date
- Detail affiche "E3 — The One Where..." et la date
- Les episodes deja nommes ne sont pas ecrases a vide

**Fichiers :** `internal/repository/episode.go`, `internal/service/background.go`

### T18 — P3 : Coherence de langue sur Login
Melange francais/anglais sur la page Login. Tout doit etre dans la meme langue.

### T19 — P3 : Accessibilite boutons retour/edit (Detail)
Les boutons sont des `<div>` sans role ni label. Passer en `<button>` avec `aria-label`.

---

## Phase 6 — Refactoring domaine

### T20 — Decouper tmdb.go et anilist.go
Separer par responsabilite : search, details, covers / search, sync.

### T21 — Pattern no-op client (PushService)
Interface PushNotifier + implementation no-op quand non configure. Supprime les `if x != nil` partout.

### T22 — Constantes pipeline + documentation erreurs
Constantes nommees pour confidence levels. Documentation du comportement de degradation gracieuse.

### T23 — Helper transaction (WithTx)
Helper generique pour les transactions SQLite.

**Fichier :** `internal/database/database.go`

### T24 — Scrobble Plex en transaction
Wrapper processMovie et processEpisode dans WithTx.

### T25 — Operations batch episodes
BatchCreate pour watch events.

---

## Phase 7 — Migration frontend

### T26 — Migrer les pages vers Zustand + utils
Remplacer les appels locaux par le store et les utilitaires partages.

### T27 — Migration CSS Modules
Extraire les styles inline dans des fichiers `.module.css` pour tous les composants.

---

## Phase 8 — Qualite et documentation

### T28 — Configuration golangci-lint
`.golangci.yml` avec errcheck, gocritic, govet.

### T29 — Tests backend supplementaires
Table-driven tests pour TitleRepository, tests d'erreur pour handlers.

### T30 — Tests frontend
Tests unitaires utils.ts + ErrorBanner + @testing-library/preact.

### T31 — Specification OpenAPI 3.0
Documentation de tous les endpoints API.

### T32 — Mise a jour patterns.md
Documenter les nouveaux packages et patterns.

---

## Verification

Apres chaque tache :
1. `make test` + `make test-front` + `make lint`
2. Verification visuelle en emulation mobile (Chrome DevTools MCP, port 5173)
3. Pas de regression sur les pages existantes
