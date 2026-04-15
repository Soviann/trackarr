# Changelog

## [Unreleased]

### Ajouté
- Plex : les relectures sont maintenant suivies — regarder un épisode déjà vu sur Plex déclenche un événement de visionnage et remonte le titre dans la liste « Derniers regardés »
- Épisodes et titres : deux timestamps distincts — `first_watched_at` (première vision) et `last_watched_at` (dernière relecture)

### Corrigé
- `make ssh-db-pull` : inclut maintenant les fichiers WAL et SHM pour ne pas rater les écritures récentes non checkpointées

## [v0.15.0] — 2026-04-12

### Ajouté
- Casting cliquable : les noms du casting dans la fiche titre sont cliquables et ouvrent une page listant tous les titres dans lesquels cette personne apparaît

### Amélioré
- Historique : les épisodes consécutifs d'une même saison sont regroupés en plages (ex: "S1 E1-12") au lieu d'une ligne par épisode — dans l'overlay History d'un titre et dans le feed Recent Activity de la page Stats

## [v0.14.0] — 2026-04-12

### Ajouté
- PWA : appui long (~500 ms) sur une vignette de la Bibliothèque pour entrer en mode sélection — vibration courte + vignette automatiquement cochée, comme Google Photos
- PWA : pull-to-refresh personnalisé sur Library, Search, Validate, MatchReview et AdminNotifications — indicateur circulaire animé qui suit le doigt, passage en teal + haptic au franchissement du seuil, rotation (spinner) pendant le refresh, idempotent si un refresh est déjà en cours
- PWA : swipe actions sur les cartes de Match Review — glisser à gauche révèle Confirm (vert) et Fix match (orange), glisser loin exécute l'action principale automatiquement
- PWA : raccourcis depuis l'icône — appui long sur l'icône PlexTracker dans le launcher Android affiche 3 raccourcis : Ajouter un titre, Bibliothèque, Recherche
- PWA : badge sur l'icône — affiche le nombre de titres en attente de révision (pending_review + unconfirmed) ; se met à jour à l'ouverture de l'app et après chaque action dans Match Review
- API : `GET /api/titles/review-count` — nombre de titres à réviser pour le badge PWA

### Amélioré
- PWA : BottomSheet amélioré — glisser-pour-fermer sur toute la surface du sheet (plus seulement la barre), blocage du scroll en arrière-plan, bouton retour Android ferme le sheet au lieu de quitter la page
- PWA : le rafraîchissement natif de Chrome (tirer vers le bas) est bloqué globalement — seul le pull-to-refresh personnalisé s'active

### Corrigé
- Bibliothèque : le mode sélection multiple est de nouveau utilisable — la grille ne déraille plus quand on active « Select » (une colonne s'élargissait à cause d'un wrapper sans `overflow: hidden`) et toucher une affiche en mode sélection coche la pastille au lieu d'ouvrir la fiche détail (le listener global de preact-router sur les `<a>` forçait la navigation en parallèle)
- PWA : le pull-to-refresh n'interfère plus avec le scroll tactile — migration de pointer events vers touch events et retrait de `touch-action: pan-x` qui bloquait le défilement vertical

### Modifié
- Bibliothèque : le bouton « Select » est remplacé par l'appui long — bouton Cancel accessible dans la barre de sélection

## [v0.13.1] — 2026-04-11

### Modifié
- i18n : traduction complète des pages Stats et Admin (plus divers libellés épars : Login, Search, Library, MatchReview, TitleHistory, aria-labels) du français vers l'anglais — toute l'interface est maintenant en anglais, conformément à la convention projet

## [v0.13.0] — 2026-04-11

### Modifié
- Affichage des titres : priorité de fallback passée de « anglais d'abord » à « français d'abord » (fr → en → fallback). Pour les titres marqués anime, ajout d'un 3ᵉ niveau de fallback japonais (x-romaji → ja). Appliqué partout : bibliothèque, Continue Watching, Coming up, fiche titre, Stats (activité récente, fun stats)

### Ajouté
- Intégration TheTVDB : enrichissement parallèle TMDB+TVDB à chaque import/refresh — note TVDB affichée en badge sur la fiche titre, lien externe TVDB dans l'ActionDrawer, champ TVDB ID dans Fix match → Manual IDs, et prise en charge des URLs `thetvdb.com/series/<slug>` et `thetvdb.com/movies/<slug>` dans le champ Add (et partage PWA)
- Bibliothèque : badge de statut (Watching, Completed, Dropped, Plan) sur chaque entrée de la grille et de la liste pour identifier le statut d'un coup d'œil
- Fiche titre : affichage du temps de visionnage total (ex : "2h 30m") dans l'onglet Détails, cumulé sur tous les visionnages y compris les rewatches
- Colonne `total_watch_minutes` sur les titres (migration 015), maintenue automatiquement lors de chaque ajout ou suppression d'événement de visionnage (manuel, Plex, Simkl)
- Le temps de visionnage est recalculé automatiquement lors de l'enrichissement TMDB quand le runtime change
- Filtrage par genre dans la bibliothèque et la recherche (ANY ou ALL) via un panneau de genres déroulant avec recherche textuelle
- Bouton TMDB dans la page de recherche pour afficher des résultats TMDB en complément de la bibliothèque locale
- Endpoint `GET /api/genres` renvoyant les genres de la bibliothèque triés par occurrence
- Migration 016 : les genres sont maintenant stockés dans une table de jointure `title_genres` (normalisée) au lieu d'une colonne JSON
- Stats : carte "Temps regardé" dans la vue d'ensemble
- Stats : histogramme des top genres
- Stats : cartes "Série en cours" et "Meilleure série" de jours consécutifs
- Stats : flux d'activité récente paginé (`GET /api/stats/activity`) groupé par date
- TitleDetail : bouton « Historique » ouvrant un panneau de visionnage par épisode avec compteur de rewatches (`GET /api/titles/{id}/history`)
- Bibliothèque : strip "Coming up" — liste horizontale collapsible des prochains épisodes à diffuser (date issue de TMDB `next_episode_to_air`)
- Bibliothèque : strip "Continue Watching" — liste horizontale collapsible des séries en cours avec barre de progression
- Bibliothèque : mode sélection multiple — sélectionner des titres pour changer leur statut ou les supprimer en lot
- API : `GET /api/titles/continue-watching` et `GET /api/titles/upcoming` — endpoints dédiés lazy-loadés
- API : `DELETE /api/titles/{id}`, `POST /api/titles/batch-delete`, `POST /api/titles/batch-status` — suppression et mise à jour de statut par lot
- DB : migration 017 — colonnes `next_air_date` et `next_air_episode` sur la table `titles`, peuplées lors du refresh TMDB

### Corrigé
- Bibliothèque : les strips "Continue Watching" et "Coming up" chargeaient leurs données avec un préfixe `/api` dupliqué (404 silencieux) — les sections restaient désespérément vides à l'expansion
- Stats → Activité récente : le type de chaque entrée est désormais déterminé à partir du type du titre au lieu de la présence d'un nom d'épisode, les épisodes de séries sans titre renseigné ne sont plus étiquetés « Film »
- ErrorBoundary : `componentDidCatch` logue désormais l'erreur de render avec le préfixe `[ErrorBoundary]` et envoie un POST non-bloquant vers `/api/client-errors` (stub — endpoint à implémenter côté backend)
- Push notifications : les échecs d'enregistrement du service worker exposent désormais un état `pushError` retourné par `usePush`, exploitable par l'UI pour informer l'utilisateur
- JWT : expiry calculé en UTC explicite (`time.Now().UTC()`) dans les deux chemins d'authentification (Google OAuth et dev login)
- `parseSQLiteTime` : commentaire ajouté documentant que SQLite stocke les datetimes en UTC sans marqueur de fuseau — `time.Parse` retourne bien du temps UTC, comportement attendu
- Accessibilité frontend : `PosterCard` et `TitleCard` utilisent désormais `<a href>` (navigables au clavier) ; poignée `ActionDrawer` et boutons retour (`AdminTasks`, `AdminNotifications`, `Validate`) remplacés par `<button>` avec `aria-label` et `aria-expanded` ; image miniature du poster en `role="presentation"` pour éviter la redondance lecteur d'écran — Lighthouse Library 74→76, TitleDetail 98/100
- Frontend lifecycle : timer post-refresh (`setTimeout`) stocké dans un ref et annulé à l'unmount ; toutes les mises à jour d'état dans `ActionDrawer` gardées derrière un ref `mounted` — élimine les warnings de state update sur composant démonté
- AdminTasks : stale closure sur `page` dans l'effect de pagination corrigée via `pageRef` ; `eslint-disable` supprimé
- TitleDetail : clé stable (`${name}-${role}`) dans la liste des crédits au lieu d'un index numérique
- Erreurs silencieuses dans les flux enrichissement/sync : `UpdateTotalEpisodes` (backfill), `MarkWatched` et `UpdateLastWatchedAt` (import Simkl) loggués en cas d'échec ; rollback SQLite échoué signalé en log si différent de `ErrTxDone` ; limit des tâches admin clampée à 500 pour éviter les valeurs arbitraires en query param
- Goroutine cleanup `RateLimit` : la goroutine de nettoyage des IPs s'arrête proprement au shutdown de l'app (context annulé) au lieu de fuir
- Goroutine enrichissement async Plex : bornée à 30 secondes via `context.WithTimeout` pour éviter qu'un appel API suspendu ne bloque indéfiniment
- HTTP matchers : erreurs `io.ReadAll` sur les corps de réponse d'erreur (TMDB, AniList, Gemini) vérifiées et remontées explicitement au lieu d'être ignorées
- Covers TMDB et AniList : fichier partiel supprimé (`os.Remove`) si `io.Copy` échoue pendant le téléchargement (même comportement que TVDB corrigé en session 2)
- Refresh de fond : les échecs d'écriture SQLite (`titles.Update`) ne sont plus ignorés silencieusement — chaque site loggue maintenant `background: update <kind> for title <id>: <err>`
- Covers TVDB : un fichier partiel n'est plus laissé sur disque si `io.Copy` échoue pendant le téléchargement
- TVDB client : erreur `json.Marshal` dans `Login` remontée explicitement au lieu d'être ignorée
- TVDB : les IDs distants malformés (TMDB remote ID non-numérique) sont loggués au lieu d'être ignorés silencieusement
- Pipeline matching : en cas de conflit IMDB (TMDB vs TVDB), l'ID TMDB est explicitement rétabli comme canonique avant le downgrade en `pending_review`
- Refresh de fond : overview et genres TVDB persistés lorsque le titre n'a pas de TMDB ID (évite des métadonnées vides pour les titres TVDB-only)

### Amélioré
- Bundle frontend : `MatchReview`, `Stats` et `FilterDrawer` extraits dans des chunks séparés (en plus d'`Admin` déjà splitté) — améliore la mise en cache navigateur pour les sections rarement modifiées
- Détail titre : `width`/`height` explicites sur l'image miniature (80×120 px) pour éviter le décalage de mise en page (CLS) avant chargement du CSS
- Match Review : chargement initial réduit de 500 à 50 éléments par section (pending/unconfirmed), avec bouton « Load more » ; confirmation en lot parallélisée (requests simultanées au lieu de séquentielles)
- Admin Tasks : pagination réelle par offset (`limit=50&offset=N`) au lieu d'un limit croissant qui re-téléchargeait tout à chaque page
- Admin Notifications : annulation automatique du fetch lors de la navigation (remplacement de `apiFetch` brut par le hook `useApi`)
- Refresh de fond : la détection « tous les épisodes vus » utilise désormais une requête SQL EXISTS au lieu de recharger tout le titre (seasons + episodes), réduisant les requêtes SQLite lors du passage en `completed`
- Refresh de fond : suppression de la double récupération TMDB dans `refreshFromTVDB` — le TVDB ID est désormais extrait en ligne dans `refreshMovieFromTMDB` / `refreshSeriesFromTMDB` (là où `details` est déjà en mémoire), réduisant la consommation de quota TMDB d'environ 50 % sur le chemin TVDB
- Refresh de fond : goroutine `RefreshOne` limitée à 2 minutes via `context.WithTimeout` pour éviter les fuites en cas de timeout TMDB/TVDB
- Matching : récupération TMDB+TVDB parallèle limitée à 20 secondes via `context.WithTimeout` — les goroutines d'enrichissement ne peuvent plus bloquer indéfiniment sur un timeout API
- Smoke E2E (11 pages) via Chrome DevTools MCP : 0 erreur console, ActionDrawer mid-refresh sans warning, Lighthouse Library 76 / TitleDetail 93 — aucune régression introduite par les sessions 1–10
- Matching : fusion des genres TMDB+TVDB sans aller-retour JSON — les genres TMDB sont conservés en `[]string` jusqu'à la sérialisation finale
- Enrichissement TVDB : le TMDB ID est désormais récupéré depuis les `remoteIds` TVDB si aucun TMDB ID n'est encore connu (back-fill silencieux)
- Enrichissement TVDB : détection de conflits de cross-référence — si TMDB et TVDB fournissent des IDs IMDB ou TMDB différents, le titre passe automatiquement en `pending_review` pour examen humain
- Détail titre : la bande hero de fond aligne désormais le haut de l'affiche en haut de l'écran, pour garder visibles les éléments graphiques importants (titres, visages) au lieu de couper au centre

### Corrigé
- Détail titre : le champ « Titre original » affiche désormais le nom Plex d'importation après un fix match, et non plus l'ancien titre matché
- Fusion de titres : les fusions impliquant un `season_offset` ne bloquent plus quand la saison cible existe déjà — les épisodes non-conflictuels sont migrés et les doublons supprimés proprement
- Bibliothèque : correction de la date `last_watched_at` des titres (historique Simkl et webhooks Plex uniquement) en supprimant le trigger automatique qui polluait le tri "Recently Watched"
- Webhook Plex : les scrobbles d'épisodes n'écrasent plus l'ID TMDB de la série par celui de l'épisode, ce qui empêchait l'enrichissement et l'auto-complétion
- Bibliothèque : détection des doublons lors de la création d'un titre via Plex si celui-ci existe déjà via ses identifiants externes (ex: import Simkl)

## [v0.12.4] — 2026-04-10

### Corrigé
- Performance : erreurs 504 sur les pages de liste causées par le blocage de la connexion SQLite unique par le refresh background — séparation en connexions lecture/écriture distinctes (SQLite WAL)
- Sécurité : autorisation du chargement de Google Identity Service dans la politique CSP

### Amélioré
- UI : mise en évidence du statut du titre dans la fiche détail

## [v0.12.3] — 2026-04-09

### Corrigé
- Frontend : build de production cassé avec Vite 8 / rolldown (`manualChunks` doit être une fonction)

### Amélioré
- CI : `make test-front` inclut désormais le build de production pour détecter les erreurs de bundling en local

## [v0.12.2] — 2026-04-09

### Amélioré
- Sécurité : durcissement des en-têtes HTTP, limiteurs de débit et journalisation
- Performance : réduction des re-rendus inutiles dans le frontend et factorisation du store
- Performance : réduction des aller-retours en base dans les dépôts et sécurisation de `ReplaceNames`
- Performance : ajout des index manquants et optimisation des requêtes de stats
- Performance : élimination des requêtes N+1 dans les dépôts

### Corrigé
- Import Simkl : le mode dry-run ne sautait pas les écritures en base — corrigé, seuls les doublons sont maintenant comptés
- Dépôt : curseurs SQL fermés explicitement dans `mergeInTx` et `Search` pour éviter les blocages (SQLite MaxOpenConns=1)
- Accessibilité : éléments interactifs rendus accessibles au clavier
- Frontend : erreurs silencieuses éliminées et types renforcés
- Service : propagation du contexte HTTP corrigée et erreurs silencieuses supprimées
- Service : cinq race conditions éliminées
- Service : arrêt propre des goroutines sur SIGTERM
- Dépôt : trois bugs de correctness corrigés

## [v0.12.1] — 2026-04-09

### Amélioré
- Performance : résolution des erreurs 504 par l'optimisation des requêtes SQL (remplacement du pattern N+1 par des chargements en vrac et ajout d'index critiques)
- UI : mise à jour du logo de l'application (branding PlexTracker)
- Fusion : permet de fusionner des titres directement depuis leur fiche

### Corrigé
- Recherche : application correcte du filtre anime dans la recherche floue


## [v0.12.0] — 2026-04-09

### Ajouté
- Fusion de titres : support de la fusion manuelle de titres en doublon avec identification intelligente de la saison (via Gemini pour les anime)
- Résolution par URL : support de l'ajout direct via URLs TMDB (movie/tv) et AniList
- Résolution IMDb : support de la résolution d'URL IMDb via l'API TMDB Find
- Configuration : ajout de `DISABLE_BACKGROUND_TASKS` pour désactiver les tâches de fond et le worker en environnement de développement ou de test

### Modifié
- Architecture Backend : refonte majeure centralisant la logique métier dans `TitleService`, `LibraryService` et `BackfillService` pour une meilleure maintenabilité
- Performance & Stabilité : résolution des interblocages (deadlocks) SQLite par une orchestration optimisée des services et des transactions
- Pipeline de Matching : amélioration de la détection des types et de la résolution croisée des IDs (TMDB Movie vs TV)
- Interface de Recherche : le filtre de statut est désormais sur "All" par défaut et les types sont réordonnés (Anime en priorité)
- UX de Recherche : préservation des résultats et de la position de défilement lors de la navigation arrière vers la page de recherche
- UI Edition : ajout d'un bouton Annuler sur la modal d'édition pour une meilleure fluidité

### Corrigé
- Frontend : correction de typages TypeScript dans les utilitaires et amélioration de la gestion des placeholders pour les titres inconnus

## [v0.11.1] — 2026-04-07

### Corrigé
- File d'attente des tâches : reprise automatique des tâches bloquées en statut « running » après un redémarrage ou un crash
- File d'attente des tâches : ajout de protections contre les panics pour éviter l'arrêt définitif du worker en cas d'erreur inattendue sur une tâche
- File d'attente des tâches : réduction de la cadence (0.5 req/s) et ajout d'un backoff global de 5 minutes en cas de détection d'un dépassement de quota (429) sur les API externes
- File d'attente des tâches : extension du cycle de vie des tâches en erreur de 2 à 7 jours pour une meilleure résilience face aux blocages prolongés


## [v0.11.0] — 2026-04-07

### Ajouté
- Ajout par URL : support de l'ajout direct de titres via leurs URLs (TMDB, AniList)
- PWA : support de la cible de partage (Share Target) pour ajouter des titres depuis d'autres applications

### Corrigé
- Add : correction des erreurs de lint dans la résolution d'URL

## [v0.10.0] — 2026-04-07

### Ajouté
- Covers : nettoyage quotidien automatique des couvertures orphelines (segmenté par préfixe pour économiser les ressources)

## [v0.9.1] — 2026-04-07

### Corrigé
- Worker : correction de l'initialisation du pipeline d'enrichissement (erreur « pipeline not configured »)

## [v0.9.0] — 2026-04-07

### Ajouté
- Anime : nouveau flag `is_anime` sur les séries (remplace l'ancien type anime), cohabitant avec le type série
- Anime : fusion automatique des saisons d'anime éclatées sur plusieurs titres
- Drawers et bottom sheets : fermeture par glissement (drag-to-close) sur tous les tiroirs
- Recherche : les filtres redondants sont remplacés par le FilterDrawer unifié
- Tri par date de dernier visionnage
- Admin : remplacement des `confirm` natifs par une modal drawer cohérente
- Admin : filtrage et suppression en lot des tâches
- Makefile : commandes `db-pull` / `db-push` pour synchroniser la base avec le NAS

### Amélioré
- Stats : nouvel effet néon cyan (#00F2FF) pour les barres (vibes Solo Leveling)
- Lisibilité : libellés gris éclaircis sur l'ensemble de l'interface

## [v0.8.0] — 2026-04-06

### Ajouté
- Tri par date de sortie (nouveau tri par défaut)
- Filtre par date de sortie : décennie, intervalle de dates, option d'inclusion des titres sans date
- Enrichissement TMDB : la date de sortie est désormais persistée depuis TMDB

## [v0.7.1] — 2026-04-05

### Corrigé
- Notifications : la notification « série terminée » est désormais conditionnée aux réglages utilisateur (nouveau toggle dans l'admin)

### Amélioré
- Notifications : tous les textes (push + labels admin) sont en anglais par défaut

## [v0.7.0] — 2026-04-05

### Ajouté
- Refonte complète de la page détail titre : hero plein écran, zone d'identité avec mini-poster, cartes de contenu (notes, synopsis, casting, détails)
- Métadonnées TMDB stockées en base : synopsis, genres, durée, note TMDB et casting (top 5 acteurs + réalisateurs)
- Score moyen AniList récupéré et affiché pour les anime
- ActionDrawer : tiroir d'actions repliable remplaçant l'ancienne barre d'actions (actions rapides + gestion)
- Bouton admin « Rafraîchir toutes les métadonnées » pour peupler les métadonnées sur les titres existants (TMDB + AniList, async)
- Tri dans le tiroir de filtres : dernière mise à jour, titre, année, note, date d'ajout — avec direction réversible et persistance localStorage
- Fix Match : recherche TMDB depuis la fiche titre pour corriger un mauvais match (poster + nom + année), avec saisie manuelle des IDs en fallback
- L'enrichissement met désormais à jour les noms multilingues en plus du cover et des IDs

## [v0.6.1] — 2026-04-05

### Corrigé
- Import Simkl : les titres avec un même ID TMDB mais de types différents (film vs série) ne sont plus ignorés à tort
- Import Simkl : les erreurs de backfill sont comptabilisées dans le total d'erreurs
- Migration : ajout de la source `backfill` dans la contrainte CHECK de `watch_events`
- Makefile : le mot de passe NAS avec caractères spéciaux (`$`, `#`) est correctement transmis via SSH
- Makefile : utilisation du chemin complet `/usr/local/bin/docker` pour les commandes sur le NAS

### Amélioré
- Makefile : séparation des commandes locales (`import`, `db-reset`) et NAS (`ssh-import`, `ssh-db-reset`) avec helper SSH factorisé

## [v0.6.0] — 2026-04-05

### Ajouté
- Import Simkl : enrichissement automatique des titres importés via la file d'attente existante
- Import Simkl : backfill des épisodes précédents (marque vus les épisodes antérieurs au plus avancé)
- Couvertures AniList : téléchargement automatique des covers depuis AniList quand TMDB n'a pas d'image (fallback pour les anime)

## [v0.5.0] — 2026-04-05

### Ajouté
- File d'attente avec retry automatique pour les tâches async (enrichissement, refresh, couvertures) — les erreurs de rate limit ou réseau ne perdent plus la tâche
- Retry en 2 niveaux : 5 tentatives jour même avec backoff exponentiel, report au lendemain si échec, puis tâche morte
- Section Admin dans la navbar avec page hub (Validations, Tâches, Notifications)
- Page de gestion des tâches échouées (relancer / supprimer)
- Préférences de notifications push activables/désactivables par type (rappel de notation, tâche échouée)
- Notification push quand une tâche meurt définitivement
- Tiroir de filtres unifié sur les pages Library et Search
- Auto-complétion des séries terminées au dernier épisode

### Corrigé
- Couleur du filtre "All" actif dans le tiroir
- Espace entre l'ActionBar et la Navbar sur la page de détail

### Amélioré
- Gestion des séries/anime à la réception des webhooks Plex
- Comportement des filtres de statut et de type

## [v0.4.0] — 2026-04-05

### Ajouté
- Backfill automatique : marquer un épisode comme vu marque aussi tous les épisodes précédents de la même saison

## [v0.3.2] — 2026-04-05

### Corrigé
- Base de données en lecture seule en prod — le conteneur démarre maintenant en root, corrige les permissions du volume `/data`, puis bascule sur l'utilisateur applicatif via `gosu`

## [v0.3.1] — 2026-04-05

### Corrigé
- Webhooks Plex systématiquement rejetés (erreur 400) — remplacement du parsing multipart manuel par la bibliothèque `plexwebhooks`, avec fallback JSON si le reverse proxy altère le Content-Type

## [v0.3.0] — 2026-04-04

### Ajouté
- Pagination serveur avec réponse allégée (compteurs saison, next_episode)
- Placeholders colorés par type pour les titres sans couverture
- Métadonnées épisodes enrichies depuis TMDB (nom, date de diffusion)
- États d'erreur avec bouton « Réessayer » et écran de secours JS
- Fondations frontend : store Zustand, utilitaires partagés, tokens de thème
- Recherche améliorée : tri par pertinence, limite à 50 résultats

### Refactorisé
- Découpage des clients TMDB et AniList par responsabilité (search, details, covers, sync)
- Interface `PushNotifier` avec implémentation no-op (suppression des nil checks)
- Interface `DBTX` et helper `WithTx` pour transactions SQLite
- Scrobbles Plex atomiques via transactions
- `BatchCreate` pour les watch events
- Gestion d'erreurs centralisée avec helpers HTTP
- Rate limiter `x/time/rate` pour les appels API externes

### Corrigé
- Crash JS causé par `episodes: null` dans le JSON
- Cohérence de langue sur l'écran de connexion
- Accessibilité des boutons retour/edit (balises `<button>` + `aria-label`)

## [v0.2.0] — 2026-04-04

### Ajouté
- Page Stats avec tableau de bord complet

### Corrigé
- Sécurité : CSP, open redirect, referrer leaks, durée JWT, rate limiting

## [v0.1.3] — 2026-04-04

### Corrigé
- Déploiement : préserve les fichiers locaux (`logs/`, `.env.local`) lors de la mise à jour
- CI : ne tourne plus que lors des releases

## [v0.1.2] — 2026-04-04

### Corrigé
- Déploiement : évite la suppression de `.env.local` lors de la mise à jour

### Ajouté
- Déclenchement manuel du déploiement (sans tag)

## [v0.1.1] — 2026-04-04

### Corrigé
- CI : build le frontend avant les jobs Go (go:embed ignorait le .gitkeep)
- CI : corrige toutes les erreurs errcheck dans le code et les tests

## [v0.1.0] — 2026-04-04

Version initiale avec CI/CD.

### Ajouté
- Pipeline CI GitHub Actions (tests Go, lint, build frontend, tests frontend)
- Création automatique de GitHub Releases à partir du changelog
- Déploiement automatique sur NAS Synology via SSH
- Scripts NAS de mise à jour (avec rollback) et diagnostics
