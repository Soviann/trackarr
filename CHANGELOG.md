# Changelog

## [Unreleased]

### Synchronisation AniList
- Visionnages, changements de statut et notes propagent désormais automatiquement vers AniList — chaque saison a sa propre entrée AniList, finir JJK S1 et démarrer S2 arrive proprement côté compte au lieu d'écraser S1. Les pushs passent par la task queue (nouveaux types `anilist_push_season` / `anilist_push_movie`) pour ne jamais bloquer la requête HTTP sur la latence AniList, et une note qui serait envoyée sur une saison encore en cours est filtrée côté émetteur (AniList refuse le score tant qu'il reste des épisodes à voir).
- Mapping par saison alimenté automatiquement : importer un anime à plusieurs saisons (ou fusionner une suite importée séparément) renseigne maintenant `season_external_ids` côté serveur — la saison 1 est cartographiée dès le backfill du premier épisode si le titre a un `anilist_id`, et la fusion d'un titre source vers une saison existante propage l'ID AniList du source (ou le re-recherche par nom si le source n'en a pas) sans jamais écraser un lien déjà confirmé. Conséquence visible : push AniList et UI par saison fonctionnent immédiatement après l'import, sans nécessiter de "Link entry" manuel pour les cas standards.
- Score communautaire AniList exposé par saison : la fiche détail d'un anime renvoie désormais, pour chaque saison cartographiée, l'ID AniList associé et le score communautaire (`averageScore`). Le rafraîchissement quotidien interroge AniList pour chaque mapping et persiste le score dans la nouvelle colonne `seasons.anilist_average_score` — une saison sans mapping ou sans score reste à `null`, un 401 lève le drapeau token invalide partagé avec le push (qui déclenche déjà la bannière de reconnexion) et abandonne les appels suivants, une erreur réseau sur une saison ne bloque ni les suivantes ni le reste du rafraîchissement. Plombier pour le bandeau d'information par saison à venir côté UI.

### Fiabilité
- Marquage manuel d'un épisode comme vu : le backfill des épisodes précédents n'est plus appelé à l'intérieur de la transaction d'écriture — il tournait dans sa propre transaction sur le pool writeDB (`MaxOpenConns=1`) et bloquait chaque clic de case à cocher ~3 min jusqu'à annulation par le navigateur. Un test garde-fou se met en timeout si la structure imbriquée réapparaît.

## [v0.17.1] — 2026-04-24

### Déploiement
- Build Docker NAS : le contexte de build exclut désormais `data/` (la base SQLite live), `covers/`, `.git/`, `node_modules/` et autres artefacts locaux via un `.dockerignore` explicite — la release **v0.17.0** a échoué à se déployer sur le NAS parce que `COPY . .` du stage backend dépassait les 15 minutes à transférer plusieurs centaines de Mo inutiles. v0.17.1 embarque toutes les corrections de v0.17.0 et rétablit un déploiement qui tient en quelques minutes.

## [v0.17.0] — 2026-04-24

Release de hardening issue de l'audit 2026-04-24 (22 findings clos, 11 batches). Sécurité, fiabilité, UX : rien de visible pour un scrobble normal, mais les marges qui pouvaient dégénérer sous charge, attaque, outage externe ou crash UI sont désormais bornées.

### Fiabilité
- Arrêt du serveur : les goroutines d'arrière-plan (ticker de rafraîchissement journalier, worker de la file de tâches, `/titles/{id}/refresh`) sont désormais suivies par un `sync.WaitGroup` partagé ; `Serve()` attend jusqu'à 10 s qu'elles terminent leur itération en cours **après** `http.Server.Shutdown` et **avant** de fermer la base — les transactions en vol ne rencontrent plus `database is closed` au redémarrage et les tâches ne restent plus bloquées en `status=running` à cause d'un SIGTERM mal placé
- Webhooks Plex : un scan massif d'une bibliothèque ne lance plus des centaines de goroutines d'enrichment concurrentes qui saturent TMDB / AniList / Gemini — l'enrichment est désormais enfilé dans la task queue au sein même de la transaction du webhook, et la rafale devient un backlog consommé à cadence bornée par le rate-limiter partagé du worker (dedup key `enrichment:<titleID>` pour ignorer les répétitions)
- Statistiques : la page `/stats` respecte enfin l'annulation du client — les dix sous-requêtes (aperçu, notes, répartition, fun stats, bilan annuel, genres, streaks, watchtime) propagent désormais le contexte de la requête HTTP, et un onglet fermé pendant un calcul long libère immédiatement son slot sur la lecture de la base au lieu de le tenir jusqu'à la fin de l'agrégation
- Rafraîchissement journalier : le ticker d'arrière-plan ne meurt plus silencieusement en cas de panic dans `RefreshTitles`, `FetchMissingCovers` ou `CleanupUnusedCovers` — une boucle externe redémarre la goroutine après 30 s de backoff sur le même pattern que le worker de la file de tâches, et le daemon ne peut plus se retrouver à tourner indéfiniment sans jamais réessayer la mise à jour des titres
- Notifications push : une souscription expirée (statut `404` / `410` renvoyé par FCM ou Mozilla après désinstallation du navigateur, changement de profil ou révocation) est désormais purgée automatiquement de la base — fini les dizaines de tentatives inutiles jour après jour contre un endpoint mort. Les autres erreurs (429, 5xx, VAPID mal signé) sont enfin loguées côté caller au lieu d'être silencieusement ignorées, et le `Retry-After` est parsé pour que la logique de retry (partagée avec TMDB / AniList) puisse en tenir compte
- Connexion Google : la vérification du jeton `id_token` contre `oauth2.googleapis.com/tokeninfo` utilise désormais un client HTTP plafonné à 5 s et rattaché au contexte de la requête — un outage ou un endpoint stallé côté Google ne peut plus bloquer indéfiniment la goroutine du handler de login, et un onglet fermé pendant l'authentification interrompt immédiatement l'appel upstream au lieu de le laisser consommer le pool
- Authentification : la vérification JWT tolère désormais un décalage d'horloge de 30 s entre le device de l'utilisateur et le serveur — un téléphone dont l'horloge recale après un sleep ou un client derrière un NTP à la dérive ne se retrouve plus déconnecté pour un skew de quelques secondes, sans pour autant élargir la fenêtre de validité au point d'autoriser un replay significatif
- Diagnostic : `httputil.WriteJSON` logue désormais l'erreur d'encode au lieu de la swallow — un type accidentellement non-serializable (cycle, `chan`, `MarshalJSON` qui throw) devient visible dans les logs au lieu de produire silencieusement une réponse HTTP 2xx avec body vide ou tronqué

### Sécurité
- Webhooks Plex : la branche multipart plafonne désormais le corps à 1 MiB via `http.MaxBytesReader` — un proxy défaillant ou une source hostile ne peut plus pousser une payload géante qui saturait la mémoire (le fallback non-multipart avait déjà ce cap, mais tronquait silencieusement au lieu de renvoyer 413)
- Administration : `DELETE /admin/tasks/batch` et `POST /admin/notifications/prefs` plafonnent désormais le corps JSON à 1 MiB via `httputil.ReadJSON`, et la suppression par lot refuse les requêtes de plus de 1000 IDs avec un 400 explicite — un compte admin compromis ne peut plus déclencher un OOM en envoyant un payload géant ni forcer un `WHERE id IN (…)` monstrueux contre SQLite

### Performance
- API externes : rafraîchissement journalier, récupération des couvertures manquantes et worker de la file de tâches partagent désormais un unique rate-limiter 2 rps / burst 1 contre TMDB + AniList au lieu de trois limiteurs indépendants qui pouvaient cumuler jusqu'à 6 rps en parallèle — l'intention du code (deux requêtes par seconde maximum) est enfin respectée, quel que soit le nombre de loops actifs simultanément

### Interface
- Bibliothèque : une erreur réseau pendant une action de masse (changement de statut, suppression) ne laisse plus l'UI dans un état irrécupérable — la sélection est préservée, la bannière d'erreur s'affiche, la drawer de confirmation reste ouverte pour retry, et les boutons sont désactivés pendant l'appel pour bloquer le double-submit. Le bouton « marquer cet épisode » des cartes devient également `disabled` pendant la requête au lieu de seulement changer de couleur, ce qui évite un double POST en cas de tap rapide
- Bibliothèque : un changement de filtre ou de tri pendant le chargement d'une page suivante (`Load more`, recherche) ne pollue plus la liste avec des résultats périmés — le store discarde les réponses dont la génération ne correspond plus à l'état courant, sur le même pattern que la recherche. Un 401 reçu alors que l'utilisateur est déjà sur `/login` ne redirige plus en boucle

### Sécurité
- Listing des titres : le nom de colonne du `ORDER BY` passe désormais par une whitelist interne au repository (en plus de celle du handler HTTP) — un futur appelant qui construirait un `TitleFilter` sans passer par la couche HTTP ne peut plus injecter du SQL via le champ `Sort`, la valeur non reconnue retombe simplement sur l'ordre par défaut `updated_at DESC`
- Rate limiter : quand plus de 10 000 IPs distinctes sont suivies simultanément (attaque distribuée depuis un botnet), un utilisateur légitime connecté depuis une IP fraîche n'est plus refusé d'office — la table est désormais un LRU (`hashicorp/golang-lru/v2`) qui évince l'IP la moins récemment vue au lieu de bloquer toute nouvelle entrée, et la phase de cleanup périodique n'impacte plus l'ordre de récence

### Robustesse
- BottomSheet : si le composant est démonté abruptement (erreur parent, crash d'un enfant pendant le commit, rechargement HMR), le verrou `document.body.style.overflow = 'hidden'` est désormais restauré dans tous les cas par un effet unmount-only dédié — la page ne peut plus se retrouver avec le scroll bloqué suite à un crash d'UI transitoire
- Bibliothèque : les strips « Continue Watching » et « Coming up » sont désormais rattachés à des `AbortController` par requête — quitter la page pendant le chargement annule immédiatement le fetch au lieu de laisser arriver une réponse tardive qui essaierait de `setState` sur un composant démonté (warning React) ou pollerait le prochain rendu avec des données périmées

### Fiabilité
- Arrêt du serveur : `make down` se termine désormais en ~2 s au lieu d'attendre le SIGKILL de Docker après 10 s — le rafraîchissement en arrière-plan propage l'annulation du contexte en tête de chaque boucle, les goroutines lancées par `/admin/refresh-all` et `/titles/{id}/refresh` sont rattachées au cycle de vie du serveur, et `http.Server.Shutdown` remplace `ListenAndServe` pour couper proprement les connexions en cours
- Base de données : les écritures sur les titres, saisons, épisodes, events de visionnage, genres, file de tâches et réglages (création, mise à jour, fusion, suppression, enqueue, completion, échec, save/delete de clé) ne peuvent plus être lancées hors transaction par accident — le compilateur refuse désormais ce qui produisait auparavant des deadlocks SQLite ou des écritures perdues selon le chemin pris, et toutes les écritures propagent le contexte de la requête pour que l'abandon du client interrompe immédiatement le statement en cours
- File de tâches : la finalisation d'une tâche (succès, échec, suppression d'un type inconnu) ne dépend plus du contexte de traitement — le verdict est écrit dans un contexte dédié de 5 s pour éviter qu'un client HTTP qui coupe sa connexion en plein enrichment laisse la tâche bloquée en `running` et récupérée en double par le `ResetRunning` du prochain démarrage
- Import Simkl : l'écriture des saisons, épisodes et events d'un titre se fait désormais dans une seule transaction par titre — un crash mid-import ne laisse plus d'épisodes sans event ou de `total_episodes` désynchronisé
- Refresh TMDB : l'upsert d'une saison et de ses épisodes partage désormais une même transaction — un crash entre les deux ne peut plus laisser `total_episodes` en désaccord avec le nombre réel d'épisodes

### Performance
- Enrichment : les quatre écritures de fin de tâche (métadonnées, watchtime, alias, genres) sont désormais regroupées dans une seule transaction — le verrou d'écriture SQLite n'est plus pris quatre fois par titre enrichi

### Corrigé
- Webhooks Plex : un appel réseau bloqué (TMDB, push) pendant le traitement ne gèle plus indéfiniment la connexion d'écriture SQLite ; les transactions respectent désormais le contexte de la requête HTTP (timeout 30 s), l'auto-complétion TMDB tourne hors transaction et les notifications push ont un timeout HTTP de 5 s — corrige la perte silencieuse d'événements `media.scrobble` / `media.play` observée depuis le 17 avril
- Backfill d'épisodes : l'appel TMDB de récupération des saisons précédentes se fait désormais hors transaction — plus aucun blocage de la connexion d'écriture unique en cas de lenteur TMDB
- Notifications « Noter ce titre » : le push part désormais après le commit de la transaction — un endpoint push lent ne peut plus bloquer l'écriture SQLite

## [v0.16.4] — 2026-04-17

### Corrigé
- Accueil : les noms des titres dans les bandeaux « Coming up » et « Continue Watching » sont désormais tronqués avec des points de suspension au lieu de déborder sur les cartes voisines

## [v0.16.3] — 2026-04-17

### Corrigé
- Accueil : les affiches des bandeaux « Coming up » et « Continue Watching » s'affichent désormais au lieu de l'icône d'image cassée — l'URL de couverture était utilisée brute sans le préfixe `/api/covers/`

## [v0.16.2] — 2026-04-16

### Corrigé
- Accueil : les bandeaux « Coming up » et « Continue Watching » ne tombent plus en 504 quand un refresh en arrière-plan tourne — leurs requêtes passent désormais par le pool de lectures (WAL) au lieu de la connexion d'écriture unique
- Performance : ajout d'un index partiel sur `titles(next_air_date)` pour accélérer le bandeau « Coming up » sur les bibliothèques fournies (migration 019)

## [v0.16.1] — 2026-04-16

### Corrigé
- Fusion : la fiche conservée récupère désormais les identifiants externes (IMDb, TMDB, TVDB, AniList, Plex rating key) manquants depuis la fiche supprimée — évite la recréation d'un doublon au prochain webhook Plex
- Plex : un webhook `media.play` sur un épisode jamais marqué vu déclenche désormais le marquage (rattrapage après un `media.scrobble` manqué) au lieu d'être silencieusement ignoré

## [v0.16.0] — 2026-04-15

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
