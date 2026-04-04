# Correctifs post-audit PO — PlexTracker

## Contexte

Apres l'import Simkl de 7 459 titres, un audit PO a revele plusieurs problemes : crash JS sur les pages principales (Library, Search, Detail), performance degradee (7,1 Mo de JSON, 449 000 px de DOM), metadonnees manquantes sur les episodes, et quelques ajustements UX. Ce plan corrige ces problemes par priorite.

**Exclusions explicites :** page Stats (volontairement "Coming soon"), distinction Search/Add (pas de changement), endpoint dev login (deja protege par `DEBUG_LOGIN`).

---

## Tache 1 — P0 : Corriger le crash JS (Library, Search, Detail)

**Probleme :** 21 saisons sur 1 815 ont `episodes: null` dans le JSON. Le tag `omitempty` sur le champ Go + slices non initialisees = `null` en JSON. Le build frontend embarque est en retard sur le code source (qui a des gardes `?? []`).

**Fichiers :**
- `internal/model/season.go:11` — retirer `omitempty` du tag JSON de `Episodes`
- `internal/repository/title.go:107,247` — initialiser `Episodes` a `[]Episode{}` apres le scan de chaque saison (dans `GetByID` et `List`)
- Rebuild frontend via Makefile

**Criteres d'acceptation :**
- `/api/titles` et `/api/titles/{id}` retournent toujours `"episodes": []` (jamais `null`)
- Library, Search, Detail s'affichent sans erreur console
- `make test` passe

---

## Tache 2 — P1 : Onglet "Watching" par defaut, "All" en dernier

**Probleme :** La Library s'ouvre sur "All" (7 459 titres). L'utilisateur veut voir ses series en cours en premier.

**Fichiers :**
- `frontend/src/app.tsx:26` — changer le state initial de `'all'` a `'watching'`
- `frontend/src/components/FilterBar.tsx:5-12` — reordonner : Watching, Up to date, Completed, Dropped, Plan, All

**Criteres d'acceptation :**
- A l'ouverture, l'onglet "Watching" est actif
- "All" est le dernier onglet a droite
- Chaque onglet filtre comme avant

---

## Tache 3 — P2 : Pagination serveur pour `/api/titles`

**Probleme :** L'endpoint renvoie tout (7,1 Mo JSON, toutes saisons + tous episodes). Le DOM fait 449 355 px. Inutilisable sur mobile.

**Etape 1 — Reponse legere (pas d'episodes dans le listing)**
- `internal/repository/title.go` methode `List` : ne plus charger les episodes. Ajouter des compteurs par saison (`watched_count`, `total_count`) via une sous-requete SQL `COUNT`.
- `internal/handler/title.go` : la serialisation change (saisons avec compteurs, sans tableau d'episodes).
- `GET /api/titles/{id}` reste inchange (detail complet).

**Etape 2 — Pagination `limit`/`offset`**
- Ajouter `Limit` et `Offset` a `TitleFilter` (`internal/repository/title.go`)
- Le handler lit `?limit=50&offset=0` (defaut : limit=50)
- Reponse : `{ "items": [...], "total": 7459 }`

**Etape 3 — Filtre "Up to date" cote serveur**
- `isUpToDate` est aujourd'hui calcule cote frontend. Avec la pagination, il faut une clause SQL : un titre "watching" dont tous les episodes sont vus (comparaison `watched_count == total_count` par saison).

**Etape 4 — Frontend "Charger plus"**
- `Library.tsx` : utiliser le nouveau format pagine, bouton "Charger plus" en bas de liste
- `hooks/useApi.ts` : adapter ou creer un hook pour les reponses paginee

**Etape 5 — Rebuild + tests**

**Criteres d'acceptation :**
- "Watching" charge en < 500 ms (~95 titres)
- "All" charge 50 titres puis affiche "Charger plus"
- La page Detail affiche toujours tous les episodes
- `make test` et `make test-front` passent

---

## Tache 4 — P3 : Enrichir les metadonnees episodes (nom, date)

**Probleme :** Tous les episodes affichent "E1", "E2" sans nom ni date. L'import Simkl ne fournit pas ces infos. Le job de rafraichissement (`background.go:191-192`) appelle `GetOrCreate` mais ignore les champs `Name` et `AirDate` du struct `TMDBEpisode`.

**Fichiers :**
- `internal/repository/episode.go` — ajouter `UpdateMetadata(id int64, name *string, airDate *string) error`
- `internal/service/background.go:191-192` — apres `GetOrCreate`, appeler `UpdateMetadata` avec `tmdbEp.Name` et `tmdbEp.AirDate`

**Criteres d'acceptation :**
- Apres un cycle de refresh, les episodes ont nom et date en base
- La page Detail affiche le nom (ex: "E3 — The One Where...") et la date
- Les episodes deja nommes ne sont pas ecrases a vide
- `make test` passe

---

## Tache 5 — P4 : Desactiver "Confirm" sans identifiant externe

**Probleme :** Le bouton "Confirm" est toujours cliquable sur Match Review, meme pour un titre sans aucun ID externe. Confirmer un titre sans ID n'a pas de sens.

**Fichier :** `frontend/src/components/MatchReviewCard.tsx`
- La variable `hasAnyID` existe deja (ligne 25). L'utiliser pour `disabled={!hasAnyID}` sur le bouton, avec opacite reduite et curseur `not-allowed`.

**Criteres d'acceptation :**
- Titre sans ID : bouton grise, non cliquable
- Titre avec au moins un ID : bouton vert, fonctionnel

---

## Verification globale

Apres chaque tache :
1. `make test` + `make test-front` + `make lint`
2. Verification visuelle dans le navigateur (Playwright MCP, port 5173) :
   - Library charge sur "Watching" par defaut, sans erreur console
   - Pagination fonctionne sur "All"
   - Detail affiche noms d'episodes (apres refresh)
   - Match Review : "Confirm" grise sans ID
