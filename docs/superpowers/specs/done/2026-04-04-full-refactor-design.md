# PlexTracker — Refactor complet

## Objectif

Améliorer la maintenabilité, la performance et la qualité du code de l'ensemble de l'application, en 6 phases incrémentales. Chaque phase est autonome, testable et déployable indépendamment.

## Principes

- Solutions éprouvées et stables plutôt que réinvention (`x/time/rate`, Zustand, CSS Modules, Testing Library)
- Rétrocompatibilité : aucun changement d'URL API, aucun changement de schéma DB
- Chaque phase déployable — on peut s'arrêter après n'importe quelle phase
- Tests verts à chaque commit

---

## Phase 1 — Fondations backend

### Gestion d'erreurs centralisée

Aujourd'hui chaque handler répète le pattern `if err != nil { log.Printf(...); http.Error(...) }` (~20 occurrences). On introduit un type wrapper `HandlerFunc func(w, r) error` et un middleware qui intercepte les erreurs retournées, log et renvoie la réponse HTTP appropriée. Les erreurs métier utilisent un type `APIError` avec code HTTP et message.

### Helpers de réponse et parsing

Package `internal/handler/httputil` contenant :
- `WriteJSON(w, status, v)` — sérialisation JSON avec status code
- `ReadJSON(r, v)` — désérialisation avec validation de Content-Type
- `ParseIDParam(r, key) (int64, error)` — extraction d'un paramètre URL entier
- `ParseQueryInt(r, key, defaultVal) int` — extraction d'un query param entier

Élimine la duplication dans tous les handlers (actuellement `strconv.ParseInt(chi.URLParam(...))` répété partout).

### Rate limiter

Remplacer les `time.Sleep(500ms)` ad-hoc dans `BackgroundService` par `golang.org/x/time/rate` (token bucket, extension standard library). Un limiter par client API externe (TMDB, AniList, Gemini).

### Résultat attendu

Chaque handler passe de ~15 lignes de boilerplate à ~5 lignes de logique métier. Le code d'erreur est centralisé et cohérent.

---

## Phase 2 — Fondations frontend

### State management — Zustand

Zustand (1.5 KB gzippé, API minimale, standard React/Preact). Un store pour le cache des titres avec invalidation après mutation. Remplace les callbacks `mutate()` ad-hoc entre composants.

### Design system — CSS Modules

CSS Modules (natif Vite, zéro configuration). Migration progressive des inline styles vers des modules `.module.css`. Extraction des tokens de spacing/sizing dans `theme.ts` (actuellement seules les couleurs sont tokenisées).

### Gestion d'erreurs UI

- Composant `ErrorBanner` réutilisable pour afficher les erreurs API
- Hook `useApi` enrichi pour exposer l'erreur de façon exploitable
- Feedback visuel sur les mutations échouées (toast ou banner)

### Utilitaires partagés

Fichier `utils.ts` — `getName(title)`, `getTypeLabel(type)`, `formatDate()`. Supprime la duplication entre pages (actuellement ces fonctions sont copiées dans plusieurs composants).

---

## Phase 3 — Domaine Titres

### Split TitleRepository (505 lignes → 3 fichiers)

- `title_repo.go` — CRUD : Create, GetByID, Update, Delete, List
- `title_search_repo.go` — FTS5 prefix search, LIKE fallback, fuzzy Levenshtein
- `title_loader.go` — chargement batch des relations (noms, saisons, épisodes)

### Correction du pattern N+1

Le loader actuel fait 1 + N requêtes (une par titre pour charger ses relations). Remplacement par des requêtes batch : `SELECT * FROM seasons WHERE title_id IN (...)`, puis association en mémoire. Pattern "data loader" simplifié.

### Pagination serveur

`List()` accepte `offset` et `limit`. Le frontend passe en scroll infini avec chargement paginé côté serveur au lieu de tout charger côté client.

### Lazy loading des relations

- `List()` ne charge plus les épisodes par défaut (uniquement titres + noms + stats saisons)
- `GetByID()` charge le graphe complet (saisons + épisodes)
- Réduit drastiquement la charge sur la page bibliothèque

---

## Phase 4 — Domaine Matching & Services

### Split des gros services

- `tmdb.go` (261 lignes) → `tmdb_search.go`, `tmdb_details.go`, `tmdb_covers.go` (même struct `TMDBClient`, fichiers séparés par responsabilité)
- `anilist.go` (247 lignes) → `anilist_search.go`, `anilist_sync.go`

### No-op client pattern

Remplacer les checks `if s.tmdb != nil` éparpillés dans les services par un pattern "no-op client" : interface avec implémentation vide retournée quand la clé API n'est pas configurée. Élimine les conditionnels défensifs.

### Nettoyage pipeline

- Constantes nommées pour les seuils de confiance (actuellement magic numbers)
- Formaliser la gestion des erreurs partielles : un step qui échoue est loggé mais n'empêche pas les suivants

---

## Phase 5 — Domaine Épisodes & Scrobble

### Transactions SQLite

Le `PlexService` qui crée titre + épisodes + watch event dans une séquence le fait dans une transaction unique. Ajout d'un helper `WithTx(ctx, db, func(tx) error)` réutilisable.

### Batch operations

Le batch mark episodes utilise une seule requête `UPDATE ... WHERE id IN (...)` au lieu de N updates individuels.

### Webhook Plex

Pas de changement d'URL (rétrocompatibilité Plex). Ajout de validation plus stricte du payload et logging structuré pour faciliter le debug.

---

## Phase 6 — Qualité & Documentation

### Tests

- Backend : augmenter la couverture des repositories et handlers, table-driven tests systématiques
- Frontend : tests de composants avec Vitest + `@testing-library/preact`
- Viser les chemins critiques : CRUD titres, pipeline matching, scrobble

### Lint strict

- golangci-lint : activer `errcheck`, `gocritic`, `exhaustive`
- Frontend : vérifier l'absence de `any` TypeScript, activer les règles ESLint strictes

### Documentation API

Fichier OpenAPI 3.0 statique (~15 routes, un fichier YAML suffit). Pas de génération automatique (overhead disproportionné pour le volume).

### Mise à jour patterns.md

Refléter les nouveaux packages, helpers et patterns introduits durant le refactor.

---

## Hors périmètre

- Changement de framework (chi, Preact restent)
- Migration de base de données (schéma SQLite inchangé)
- Nouvelles fonctionnalités utilisateur
- i18n (prévu séparément)
