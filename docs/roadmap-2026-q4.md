# Roadmap d'Évolution & Performance Trackarr (Q4 2026)

Date de cadrage : 2026-10-11  
Statut : **Validé (Prêt pour exécution séquentielle)**  
Philosophie : **Chaque lot dispose de son plan d'implémentation dédié et fait l'objet d'une session de travail indépendante.**

---

## 🧭 Vue d'Ensemble & Objectifs

Cette roadmap rassemble les arbitrages issus de l'analyse multi-agents et des retours d'usage. Elle vise à :
1. **Soulager le stockage NAS et fluidifier le writer unique SQLite** (`synchronous=NORMAL`, cache mémoire 32 Mo, purge automatique des tâches terminées).
2. **Diviser par deux le poids du bundle initial** via du lazy-loading réel sur l'administration et des en-têtes de cache immuables.
3. **Sécuriser la recherche et l'ajout de titres** en fournissant des liens d'inspection externes (`TMDB`, `IMDb`, `AniList`) en nouvel onglet sans jamais perdre la recherche en cours.
4. **Sublimer l'expérience utilisateur sur tous les formats** : aperçu instantané au survol sur Desktop (Hover Card), studio Bento équilibré sur la fiche média (Saisons/Épisodes), et gestuelle tactile au pouce sur mobile (Swipe-to-Complete).
5. **Boucler la boucle de l'écosystème média** : tracker de sagas/franchises avec ordre chronologique et actions 1-tap, et intégration des webhooks descendants Radarr/Sonarr avec notification dédiée optionnelle.

---

## 🗂️ Découpage des Chantiers en Sessions Dédiées

```
┌────────────────────────────────────────────────────────────────────────┐
│ LOT 1 : Socle Technique, Performance SQLite & Cache Frontend           │
│ Plan : docs/plans/2026-10-11-lot-1-sqlite-perf-and-spa-cache.md        │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │
┌───────────────────────────────────▼────────────────────────────────────┐
│ LOT 2 : Recherche & Vérification Externe sans Perte d'État             │
│ Plan : docs/plans/2026-10-11-lot-2-search-external-links.md            │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │
┌───────────────────────────────────▼────────────────────────────────────┐
│ LOT 3 : Confort Visuel, Hover Card & Studio Bento TitleDetail          │
│ Plan : docs/plans/2026-10-11-lot-3-bento-studio-hover-swipe.md         │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │
┌───────────────────────────────────▼────────────────────────────────────┐
│ LOT 4 : Hub de Sagas Chronologique & Webhooks Arr Temps Réel           │
│ Plan : docs/plans/2026-10-11-lot-4-saga-hub-and-arr-webhooks.md        │
└────────────────────────────────────────────────────────────────────────┘
```

---

### 📦 LOT 1 : Socle Technique, Performance SQLite WAL & Cache Frontend
- **Objectif** : Éliminer les latences I/O disque sur NAS pour le writer unique (`MaxOpenConns=1`) et alléger le chargement initial du client web/PWA.
- **Plan dédié** : `docs/plans/2026-10-11-lot-1-sqlite-perf-and-spa-cache.md`
- **Périmètre & Livrables** :
  1. *SQLite WAL I/O* : Ajout de `_synchronous=NORMAL&_cache_size=-32000&_temp_store=MEMORY` dans `internal/database/database.go`.
  2. *Maintenance & Purge DB* : Méthode `PurgeCompletedTasks(olderThan)` dans `internal/repository/task_writer.go` et exécution planifiée quotidienne dans `internal/service/scheduler.go` suivie de `PRAGMA wal_checkpoint(PASSIVE)`.
  3. *Lazy-loading Frontend* : Chargement différé des 8 pages d'administration (`Admin*.tsx`) et de `Stats.tsx` dans `frontend/src/App.tsx` (allègement initial de ~256 Ko JS/CSS).
  4. *En-têtes HTTP Cache* : Configuration de `Cache-Control: public, max-age=31536000, immutable` pour `/assets/*` et `no-cache` pour `index.html` dans `internal/handler/spa.go`.
- **Validation** : `make test`, `make test-front`, vérification de la taille des bundles et des pragmas SQLite.

---

### 📦 LOT 2 : Recherche & Liens d'Inspection Externes (Sans Perte de Contexte)
- **Objectif** : Permettre à l'utilisateur de vérifier avec certitude la fiche officielle d'un média proposé lors de l'ajout sans quitter ni réinitialiser sa recherche sur Trackarr.
- **Plan dédié** : `docs/plans/2026-10-11-lot-2-search-external-links.md`
- **Périmètre & Livrables** :
  1. *Composants d'ajout et recherche* : `frontend/src/pages/Add.tsx` et `frontend/src/pages/Search.tsx`.
  2. *Liens externes d'inspection* :
     - Boutons / puces cliquables discrets sur chaque carte de résultat externe : `TMDB ↗`, `IMDb ↗`, `AniList ↗`.
     - Attributs stricts : `target="_blank" rel="noopener noreferrer"` pour ouvrir dans un onglet séparé.
     - Conservation intégrale de la chaîne de recherche, des filtres et du défilement dans l'onglet Trackarr.
  3. *Internationalisation (i18n)* : Traduction des labels dans `frontend/src/i18n/locales/en.ts` et `fr.ts`.
- **Validation** : `make test-front`, `npm run lint:i18n`, test manuel du clic et préservation de la saisie.

---

### 📦 LOT 3 : Confort Visuel, Hover Card Desktop & Studio Bento
- **Objectif** : Offrir une ergonomie moderne adaptée aux grands écrans (≥ 1024px) et sublimer la manipulation au pouce sur mobile.
- **Plan dédié** : `docs/plans/2026-10-11-lot-3-bento-studio-hover-swipe.md`
- **Périmètre & Livrables** :
  1. *Hover Card Desktop (`UX-1`)* : Bulle contextuelle au survol d'une jaquette (280ms de délai, conditionnée à `@media (hover: hover)`) affichant le synopsis court, le prochain épisode avec bouton `+1`, les plateformes de streaming et une lueur douce de `accent_hex`.
  2. *Studio Bento sur `TitleDetail` (`UX-2`)* : Réorganisation sur grand écran en deux colonnes synchronisées (Saisons à gauche, Épisodes à droite) supprimant le scroll asymétrique de 2000px. Ajout des raccourcis clavier (`W` pour valider le prochain épisode, touches `1..9` pour changer de saison).
  3. *Swipe-to-Complete Mobile (`UX-3`)* : Geste tactile de glissement horizontal au pouce sur les lignes d'épisodes avec retour haptique pour valider instantanément un épisode sans viser la case 18px.
- **Validation** : `make test-front`, vérification visuelle Chrome DevTools sur mobile et desktop.

---

### 📦 LOT 4 : Hub de Sagas Chronologique & Webhooks Arr Temps Réel
- **Objectif** : Unifier le suivi des franchises de films et univers anime, et connecter les téléchargements effectifs de Radarr/Sonarr à l'interface.
- **Plan dédié** : `docs/plans/2026-10-11-lot-4-saga-hub-and-arr-webhooks.md`
- **Périmètre & Livrables** :
  1. *Hub de Sagas (`MED-1`)* :
     - Évolution de `FranchiseRelationsSection.tsx` : fin de la redirection vers `/admin/validate`.
     - Actions 1-tap directement sur les cartes non possédées : `+ À voir` (création immédiate) ou `Pousser vers Radarr/Sonarr` via `ArrPushSheet`.
     - Bascule d'affichage entre ordre de sortie et ordre chronologique fictionnel avec jauge de progression globale (% complété).
  2. *Webhooks Radarr/Sonarr Descendants (`MED-2`)* :
     - Endpoint `POST /api/webhook/arr/{secret}` décodant l'événement `On Download` / `On Upgrade`.
     - Persistance de l'état `downloaded_at` en base SQLite.
     - Puce visuelle verte « Disponible » sur le héros `NextEpisodeHero`.
     - **Déclencheur de notification dédié** dans `frontend/src/pages/AdminNotifications.tsx` (`notif_media_downloaded`) activable/désactivable à volonté par l'utilisateur.
- **Validation** : `make test`, `make test-front`, tests d'ingestion de payloads webhook Radarr/Sonarr.
