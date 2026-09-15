# Journal des modifications (Changelog)

Toutes les modifications notables apportées à ce projet sont consignées dans ce fichier.

Le format est basé sur [Keep a Changelog](https://keepachangelog.com/fr/1.1.0/) et ce projet adhère à la gestion sémantique de version ([Semantic Versioning](https://semver.org/lang/fr/)).

## [Unreleased]

### Ajouté
- **Affichage de la version de l'application sur le tableau de bord Admin** :
  - Exposition du champ `app_version` dans la réponse de l'endpoint `GET /api/admin/system-settings`.
  - Affichage d'un badge de version épuré (`vX.Y.Z`) en haut à droite du tableau de bord d'administration (`Admin.tsx`).

### Supprimé
- **Nettoyage de l'en-tête d'administration** :
  - Suppression du badge redondant « Online » et du sous-titre « Personal instance » dans l'en-tête du tableau de bord d'administration.

### Corrigé
- **Alignement du type de titre lors du Rematch et fiabilisation de l'enrichissement d'arrière-plan** :
  - `POST /api/titles/{id}/rematch` et le composant `RematchSheet` transmettent et mettent désormais à jour le type du titre (`type: 'series' | 'movie'`) en fonction de l'onglet de recherche sélectionné, évitant qu'une série remariée ne reste figée en film en base de données.
  - Correction de l'erreur d'envoi aux applications Arr (`radarr add returned 400: Root folder does not exist`) provoquée par le routage erroné d'une série vers Radarr au lieu de Sonarr lorsque le type en base était corrompu.
  - Correction de `CreateAndEnrich` qui programmait une tâche d'enrichissement avec un payload vide (`{"title_id": id}`) écrasant le titre nouvellement créé en film non confirmé lors de son exécution.
  - Ajout d'une protection dans `handleEnrichment` récupérant les données en base si le payload reçu est incomplet, et protection dans `buildEnrichmentUpdate` empêchant tout changement de type de titre lors d'un matching non confirmé.

## [v1.21.2] — 2026-09-14

### Modifié
- **Audit hebdomadaire de la documentation et des règles LLM** :
  - Synchronisation de `docs/patterns.md`, `docs/llm.md` et `docs/user-guide.md`.
  - Harmonisation et nettoyage de l'inventaire des routes API (suppression des doublons d'administration) et des composants frontend (`PrimeBadge`).
  - Documentation approfondie de la persistance de session des filtres, du tiroir de filtres compact, du Hub Bar unifié et des protections par `ConfirmationDrawer`.

### Sécurité & Dépendances
- Mise à jour des dépendances Go (`go-dependencies`) et développement frontend (`vitest`).

## [v1.21.1] — 2026-09-11

### Corrigé
- **Tiroir de filtres (`FilterDrawer`) en bas de page** :
  - Suppression de la première ligne de poignée redondante (`FILTERS (count)`) lorsque le tiroir est ouvert, l'en-tête du tiroir assurant le rôle d'en-tête unique avec chevron de fermeture et bouton Réinitialiser.
  - Ajout d'un espacement vertical (`bottomPad`) entre la dernière rangée de contrôles de filtres et la barre de navigation (`Navbar`).
  - Correction du rognage des filtres en bas de tiroir (notamment le champ de pays dans l'onglet Genres & Origine) grâce à l'augmentation de la hauteur maximale (`max-height: min(70vh, 460px)` pour le tiroir et `min(55vh, 380px)` pour le contenu).

## [v1.21.0] — 2026-09-11

### Modifié
- **Refonte des blocs Univers & Franchise et Historique de visionnage (`TitleDetail`)** :
  - Remplacement de l'ancien bloc double empilé (~720px de hauteur) par une barre de raccourcis compacte unifiée (Hub Bar ~100px) regroupant Univers/Franchise et Historique de visionnage (Proposition 3C).
  - Réduction de plus de 600px de défilement vertical, rendant la progression et la liste des saisons et épisodes immédiatement visibles sans défilement sur mobile.
  - Affichage de l'aperçu synthétique dans chaque rangée (titre, nombre d'éléments, pourcentage de complétion, prochain titre chronologique ou dernier visionnage).
  - Intégration de tiroirs coulissants modernes (`BottomSheet`) avec fermeture au glisser, support du bouton retour Android et a11y pour explorer l'univers étendu ou l'historique détaillé.

## [v1.20.1] — 2026-09-11

### Corrigé
- **Résolution et affichage des affiches sur la page « À venir » (`/coming-up`)** :
  - Correction de l'affichage des jaquettes dans les vues mensuelle (`CalendarMonthGrid`) et hebdomadaire (`CalendarWeekTimeline`).
  - Utilisation de `getCoverUrl()` pour les vignettes miniatures d'événements et intégration du composant `CoverImage` pour les cartes de sorties du jour sélectionné et la frise de la semaine, évitant la résolution erronée en chemin relatif par rapport à la route courante (`/coming-up/<cover>`).
  - Ajout d'un repli propre avec icône de remplacement (`CoverPlaceholder`) en cas d'erreur de chargement ou d'absence d'affiche.
  - Couverture de test unitaire enrichie avec des noms de fichiers de jaquettes relatifs pour prévenir toute régression.

## [v1.20.0] — 2026-09-11

### Ajouté
- **Case à cocher de marquage en masse de saison avec annulation universelle (`TitleDetail`)** :
  - Case à cocher d'en-tête de saison (`18x18px`) parfaitement alignée avec la colonne des cases à cocher d'épisodes (`padding: 0 12px` dans le bloc de progression).
  - Permet de marquer l'intégralité des épisodes de la saison active comme vus ou non vus en un seul tap avec mise à jour optimiste immédiate.
  - Intégration complète avec le snackbar d'annulation universel (`UndoSnackbar`) permettant d'annuler immédiatement l'opération.
  - Support backend complet dans `POST /api/titles/{titleID}/episodes/batch-watch` pour le marquage inverse (`watched: false`), avec ajustement du temps de visionnage cumulé, rétrogradation automatique du statut du titre (`Completed` vers `Watching`), et synchronisation AniList.
- **Découverte et ajout en direct sur la page d'ajout (`/add`)** :
  - Barre de recherche supérieure avec détection de saisie, effacement rapide (`✕`) et interrogation simultanée décalée (debounced 300ms) de TMDB (`type=all`), AniList et de la bibliothèque locale.
  - Détection automatique des médias déjà présents dans la collection avec badge cliquable *« Dans la bibliothèque ↗ »* redirigeant directement vers la fiche détaillée (`/title/:id`).
  - Boutons de suivi rapide en 1 tap `[+ À voir]` et `[+ En cours]` créant instantanément le média enrichi et affichant le toast d'annulation universel avec suppression de rollback en cas d'annulation.
- **Rafraîchissement synchrone immédiat des métadonnées** :
  - Prise en charge du paramètre de requête `?sync=true` sur la route `POST /api/titles/{id}/refresh`.
  - Exécution synchrone de la mise à jour des métadonnées (affiches, résumés, saisons, épisodes) et rechargement direct dans `TitleDetail` sans nécessiter de rafraîchir manuellement la page.

### Modifié
- **Zéro chaîne codée en dur & stricte parité i18n** :
  - Remplacement de l'ensemble des textes statiques en anglais dans `ActionDrawer.tsx`, `Login.tsx`, `Setup.tsx`, `Add.tsx` et `TitleDetail.tsx` par des clés de traduction typées.
  - Parité stricte 1-pour-1 entre les dictionnaires `en.ts` et `fr.ts` validée par l'audit automatisé de linter i18n (`scripts/lint-i18n.mjs`).

## [v1.19.0] — 2026-09-11

### Ajouté
- **Notification universelle d'annulation avec compte à rebours circulaire périmétrique (`UndoSnackbar`)** :
  - Composant toast snackbar flottant centré au-dessus de la barre de navigation avec minuterie circulaire SVG radiale animée (compte à rebours de 5 secondes avec anneau périmétrique s'épuisant en sens horaire et affichage des secondes restantes).
  - Déclenchement automatique sur les actions modifiant l'état : marquage rapide « +1 », marquage d'épisode vu, marquage de film vu, suppression de titre.
  - Bouton d'annulation (« Annuler » / « Undo ») permettant d'inverser immédiatement l'action effectuée sans friction.
  - Exécution différée sécurisée pour la suppression de titre (`onExpire`) garantissant un véritable retour en arrière en cas d'annulation.
  - Nettoyage et vidage automatique des actions en attente lors du déchargement de la fenêtre (`beforeunload`).
- **Retour haptique et animation de ressort sur les boutons « +1 »** :
  - Animation physique de ressort (`@keyframes springPop`) avec rebond dynamique à l'appui sur le bouton `+1` des affiches (`PosterCard`, `PosterTile`).
  - Déclenchement d'une vibration haptique courte (`haptic([15, 30, 15])`) via l'API Vibration des terminaux mobiles.
- **Vagues de chargement shimmer sur les squelettes (`shimmerWave`)** :
  - Remplacement des pulsations d'opacité statiques par une vague de brillance animée directionnelle (`background-position` sur dégradé linéaire multi-stops) pour l'ensemble des squelettes de l'application (`Library`, `Releases`, `ComingUp`, `PresetLibrary`, `SectionCards`, `SectionRow`).
  - Intégration harmonieuse avec les 4 thèmes via le token dynamique `--skeleton-bg` et `@keyframes shimmerWave`.

## [v1.18.9] — 2026-09-11

### Ajouté
- **Poignée fermée unifiée à ligne unique avec puces dismissibles défilantes (`FilterDrawer`)** :
  - Barre horizontale de 42 px au-dessus de la barre de navigation avec déclencheur `[ FILTRES (compte) ⌄ ]` à gauche et conteneur à défilement horizontal fluide de puces de filtres actifs à droite.
  - Chaque filtre actif (statut, type, anime, genre, pays, dates, notes, tri) est affiché sous forme de puce cliquable avec croix de suppression `×` permettant de retirer individuellement le critère sans rouvrir le tiroir.
  - Prévention de la propagation du clic sur les puces et sur la zone de défilement pour garantir un usage tactile sans ouverture involontaire du tiroir.

### Modifié
- **Persistance des filtres de session** :
  - Conservation des filtres actifs en mémoire Zustand lors de la navigation entre les vues de détail de titre et la bibliothèque ou la barre de navigation.
  - Réinitialisation des filtres strictement restreinte au clic explicite de l'utilisateur sur le bouton de réinitialisation (« Réinitialiser »), à la suppression des puces, ou au rechargement de la page.
- **Protection anti-débordement des onglets sur écrans étroits (320 px)** :
  - Prise en charge du défilement horizontal (`overflow-x: auto`) sans barre visible sur la barre d'onglets du tiroir de filtres.
  - Ajustement responsive par média-requête (`@media (max-width: 360px)`) des paddings, espacements et taille de police pour les viewports très compacts (320 px type iPhone SE).
  - Internationalisation complète des libellés de sous-onglets (`tabBasics`, `tabGenres`, `tabDates`) et labels d'accessibilité de retrait de filtres.

## [v1.18.8] — 2026-09-11

### Ajouté
- **Système de tokens de contraste et d'accessibilité WCAG AAA (`--accent-fg`)** :
  - Introduction du token dynamique `--accent-fg` calibré sur l'ensemble des 4 thèmes (Vault: 8.55:1 AAA, Cyber: 8.00:1 AAA, Sunset: 3.67:1 AA bold / contraste élevé, Emerald: 7.40:1 AAA).
  - Élimination des tokens fantômes dans `tokens.css` (`--bg-surface`, `--surface`, `--surface-hover`, `--text-dim`, `--text-muted`, `--radius-xs`, `--radius-sm`, `--radius-lg`, `--shadow-modal`, `--wash-anilist`).
  - Unification des tokens de marque Radarr (`--brand-radarr: #ffc230`) et Sonarr (`--brand-sonarr: #00c0ff`).
  - Carte d'intégration Jellyfin dédiée dans le panneau d'administration (`Admin.tsx`) sous la section *Intégrations & Services Externes*.
- **Tiroirs de confirmation pour les actions destructives et de masse (`ConfirmationDrawer`)** :
  - Sécurisation de la validation en masse des correspondances dans `MatchReview` (« Tout confirmer »).
  - Sécurisation de la fusion globale des saisons dans `AdminSeasonAudit` (« Tout fusionner »).
  - Sécurisation de la déconnexion du compte OAuth dans `AdminAniList` (« Se déconnecter »).

### Modifié
- **Thématisation dynamique des icônes SVG** :
  - Remplacement des imports de couleurs statiques `colors.*` par `stroke="currentColor"` et variables CSS dans `Navbar`, `EpisodeRow`, `SearchBar`, `typeIcons` et `Admin`, adaptant immédiatement les glyphes lors d'un changement de thème.
  - Nettoyage des paramètres d'URL (`cleanPath.split('?')[0]`) pour l'activation des onglets de la barre de navigation.
  - Remplacement du fond d'alerte `ErrorBanner` par un voile sémantique rouge `color-mix(in srgb, var(--status-crit) 12%, var(--bg-elev))` au lieu d'un voile de couleur d'accent.

## [v1.18.7] — 2026-09-11

### Modifié
- **Refonte compacte UX/UI des filtres et du tri (`FilterDrawer`)** :
  - Remplacement des 20+ boutons puces (chips) encombrants par des contrôles compacts : sélecteur de tri stylisé avec bouton toggle d'ordre rapide `[ ↑ / ↓ ]`, sélecteur de statut de visionnage et contrôle segmenté pour le type (`Tout`, `Film`, `Série`) avec pastille Anime.
  - Réduction de plus de 60 % de la hauteur du tiroir déplié (passant de 480 px à ~170 px), laissant plus de 80 % de l'écran visible pour la grille de jaquettes.
  - Élimination complète du vide sous le tiroir grâce à un ajustement automatique au contenu sans padding mort au-dessus de la barre de navigation.
  - Ajout d'un sélecteur contextuel de statut de série s'affichant uniquement lorsque le type `Série` est actif.



### Modifié
- **Refonte UX/UI du bouton rapide « +1 » sur les cartes de la bibliothèque et des strips** :
  - Remplacement du disque cyan opaque par un bouton circulaire en verre sombre translucide (*dark glassmorphism* avec `backdrop-filter: blur(8px)` et fine bordure).
  - Typographie nette avec symbole `+` en couleur d'accent et chiffre `1` en blanc fixe (`#ffffff`) sur tous les thèmes.
  - Troncature automatique du titre de la carte avec points de suspension (`...`) juste avant le badge (`padding-right: 38px`), évitant toute collision ou superposition de texte.
- **Déplacement des badges de plateformes de streaming sur la fiche du titre** :
  - Retrait des badges de la zone d'en-tête/identité du titre pour alléger la présentation.
  - Intégration dans la carte *Détails* immédiatement sous la ligne « Sources », avec libellé i18n dédié (*Plateformes* / *Platforms*).

### Corrigé
- **Élimination des superpositions de badges en haut des cartes de la liste** :
  - Retrait de l'indicateur de plateforme de streaming sur `PosterCard` et `TitleCard`. La présence sur une plateforme restant consultable sur la page dédiée du titre, les badges de type (gauche) et de statut (droite) ne risquent plus de se chevaucher sur les affiches compactes.
- **Masquage du badge de disponibilité Arr et du bouton rapide +1 pour les épisodes non diffusés** :
  - Harmonisation de la détection des épisodes non disponibles : un épisode est désormais considéré non disponible s'il possède une date de diffusion future (`air_date > aujourd'hui`), s'il est un placeholder indicatif (`TBA`, `TBD`) ou si la série est déjà à jour (`caught_up`).
  - Résolution de l'anomalie sur *The Pitt* (et les séries avec saisons futures déjà référencées dans TMDB) où S03E01 s'affichait `DISPO` avec un bouton `+1` actif alors que sa diffusion n'intervient qu'en 2027.
  - Masquage du bandeau `NextEpisodeHero` et du bouton d'incrémentation `TitleCard` lorsqu'aucun épisode diffusé n'est en attente de visionnage.
  - Exclusion des épisodes futurs non diffusés du décompte d'épisodes restants et de l'estimation de binge (`unwatchedEpisodesCount`).
- **Exclusion des épisodes non diffusés lors du marquage complet et de l'évaluation de fin de série** :
  - `MarkAllWatchedForTitle` ignore désormais strictement les épisodes dont la date de diffusion est dans le futur (`air_date > aujourd'hui`) ainsi que les placeholders `TBA`/`TBD`, évitant de marquer artificiellement comme vus des épisodes futurs lors de l'auto-complétion ou du backfill.
  - `HasUnwatchedEpisodes` filtre les épisodes non diffusés dans le futur et les placeholders `TBA`/`TBD`, permettant aux séries terminées ou annulées de basculer correctement en statut `completed` dès lors que tous les épisodes diffusés ont été visionnés.
  - Couverture complète par des tests unitaires dédiés validant la préservation des épisodes futurs et des placeholders lors d'un marquage global.

## [v1.18.5] — 2026-09-10

### Ajouté
- **Indicateur de progression et contrôle de l'actualisation complète de la bibliothèque (`/admin`)** :
  - Affichage d'une barre de progression en temps réel avec pourcentage, titre en cours de traitement et compteur (`X / Total`).
  - Nouveaux contrôles permettant d'interrompre/suspendre (`Arrêter`), reprendre (`Reprendre`) ou réinitialiser (`Recommencer`) le balayage des métadonnées.
  - Nouveaux endpoints d'API `GET /api/admin/refresh-all/status` et `POST /api/admin/refresh-all/cancel` (avec support de `?restart=true` sur `POST /api/admin/refresh-all`).
- **Persistance du curseur d'actualisation de la médiathèque** :
  - Sauvegarde automatique de l'état et du curseur dans la table `settings` (`refresh_job_progress`), permettant la reprise automatique ou manuelle sans perte de progression après un redémarrage du conteneur.

### Modifié
- **Priorisation des titres jamais actualisés** :
  - `TitleRepository.ListAllForRefresh` ordonne désormais par `last_refreshed_at ASC NULLS FIRST, id ASC` au lieu de `updated_at DESC`. Les titres n'ayant jamais été enrichis sont traités en priorité absolue, empêchant les relances répétées de bloquer indéfiniment sur les mêmes titres récents.

### Corrigé
- **Relations TheTVDB pour les films** :
  - `refreshTVDBRelations` filtre désormais les films (`TitleTypeMovie`) et les identifiants TheTVDB invalides avant d'appeler `/series/{id}/extended`, éliminant les requêtes en erreur HTTP 404 et 400 dans les journaux d'arrière-plan.

## [v1.18.4] — 2026-09-09

### Corrigé
- **Génération et affichage des rétrospectives Wrapped** :
  - Suppression de la rétroactivité sur Wrapped : le `Scheduler` ne balaie plus les années passées et n'enclenche la génération de snapshot qu'« à date » pour l'année écoulée (`time.Now().Year() - 1`), sous condition d'activité réelle (`total_titles > 0`).
  - Correction de `StatsRepository.AvailableYears` pour renvoyer uniquement les années d'activité de visionnage (`watch_events`) plutôt que l'année de sortie minimale des médias en bibliothèque (`titles.year`).
  - Suppression de la sauvegarde automatique de snapshots lors de la simple consultation `GET` d'une année passée.
  - Le worker `generate_wrapped` ignore désormais les années à 0 titres (aucun appel IA Gemini inutile, aucun snapshot vide enregistré, aucune notification push).
  - Migration de base de données 046 nettoyant automatiquement tous les instantanés vides (`total_titles = 0`) dans `wrapped_snapshots`.
  - `WrappedRepository.ListArchives` et le frontend ignorent désormais tout snapshot vide éventuel.

### Ajouté
- **Contrôles d'affichage et réduction des Wrapped sur la page Stats** :
  - Bouton de masquage (✕) sur la bannière Wrapped de l'année en cours, mémorisé dans `localStorage` pour l'année.
  - Section des bilans passés réductible en une barre compacte à une seule ligne avec pastilles d'accès rapide par année (`[ 2025 ]`) et bouton Développer/Réduire, mémorisé dans `localStorage`.
  - Masquage complet de la section des archives lorsqu'aucun bilan contenant des titres n'est disponible.

## [v1.18.3] — 2026-09-05

### Modifié
- **Statut « À jour » (Caught Up) pour les séries en cours sans épisode restant (ex. Dan Da Dan)** :
  - `TitleRepository.GetByID` calcule et hydrate désormais le champ dérivé `caught_up` (via la condition SQL unifiée `caughtUpCond`).
  - Les pages `TitleDetail`, `Search`, et les composants `TitleCard` et `PosterCard` transmettent désormais systématiquement `caughtUp` à `StatusBadge`, affichant le badge cyan `CAUGHT UP` lorsque tous les épisodes diffusés ont été visionnés.
  - Le bandeau héros de prochain épisode (`NextEpisodeHero`) est masqué lorsque la série est à jour (`caught_up`) ou lorsque l'épisode suivant est un placeholder "TBA" / "TBD".
  - `HasUnwatchedEpisodes` et `MarkAllWatchedForTitle` ignorent les épisodes indicatifs non diffusés "TBA" / "TBD".
  - Suppression du forçage inconditionnel du statut `watching` dans la synchronisation TMDB pour les séries `returning`.
- **Affichage et actions sur les cartes et listes de titres** :
  - Masquage du bouton d'action rapide `+1` épisode et du badge de disponibilité Arr (`SxxExx DISPO`) pour les titres abandonnés (`dropped`).
  - Masquage du badge de disponibilité Arr pour les épisodes déclarés "TBA" / "TBD".
  - Exclusion des épisodes "TBA" du décompte d'épisodes restants et du temps d'estimation de binge (`unwatchedEpisodesCount`).

## [v1.18.2] — 2026-09-05

### Modifié
- **Modularisation des services d'arrière-plan (`Scheduler` & `MetadataSyncService`)** :
  - Décomposition de l'orchestration des crons récurrents (`internal/service/scheduler.go`) et de la synchronisation de métadonnées (`internal/service/background.go`).
  - Migration de base de données 045 ajoutant `generate_wrapped` à la contrainte CHECK de `task_queue.task_type`.
  - Nettoyage des dépendances non utilisées injectées dans `CalendarHandler` (`writeDB`, `settingRepo`).
- **Refactorisation frontend & Gestion des gestes tactiles** :
  - Extraction du hook partagé `useSwipeDownToClose` (`frontend/src/hooks/useSwipeDownToClose.ts`) avec écouteurs non-passifs `{ passive: false }` prévenant le défilement parasite lors du glisser vers le bas dans `FilterDrawer` et `ActionDrawer`.
  - Décomposition modulaire de `FilterDrawer` en sous-composants par onglet (`FilterBasicsTab`, `FilterGenresTab`, `FilterDatesTab`).
  - Consolidation de la signature des 32 props individuelles de `FilterDrawer` en structures typées `FilterState` et `FilterActions`.

## [v1.18.1] — 2026-09-05

### Sécurité & Infrastructure
- **Protection contre le listage de répertoires et traversal** : Sécurisation du endpoint de covers (`/api/covers/`) empêchant le parcours de répertoires et l'accès arbitraire aux dossiers parents ou dotfiles.
- **Protection DoS sur les requêtes entrantes** : Limitation de la taille des réponses distantes à 15 Mo pour les posters AniList et limitation stricte à 50 Mo pour les archives ZIP de sauvegarde décompressées.
- **Limitation de débit sur le flux calendrier** : Application du middleware de rate limiting (60 requêtes/minute) sur `/api/calendar.ics`.
- **Qualité de code & Linters** : Mise à niveau de `golangci-lint` en version `v2.13.2` compatible Go 1.24+ avec 0 erreur.

### Modifié
- **Architecture de persistance & Modèle Writer** :
  - Isolation compile-time stricte des mutations SQL via `WrappedWriter`, `SeasonExternalIDsWriter` et `TitleRelationWriter` exigeant une transaction `*sql.Tx`.
  - Migration de l'orchestration des transactions SQL depuis les handlers HTTP (`internal/handler/`) vers les services métier (`TitleService`, `LibraryService`).
  - Découplage de la file de tâches asynchrones (`TaskQueueWorker`) avec dispatch de handlers enregistrables (`TaskHandlerFunc`).
  - Déduplication de l'extraction et du scan de lignes de titres SQLite via `scanTitleRow`.
  - Amélioration de la concurrence : suppression des verrous prolongés lors de l'authentification externe TVDB dans `DynamicConfigService`.

## [v1.18.0] — 2026-09-05

### Ajouté
- **Module Sauvegarde & Exportation 1-Clic et Restauration d'Archive ([#58](https://github.com/Soviann/trackarr/issues/58))** :
  - Boutons d'exportation directe 1-clic : `JSON Complet` (`/api/admin/export/json`), `CSV Tableur` (`/api/admin/export/csv`), et `Trakt.tv Sync` (`/api/admin/export/trakt`).
  - Zone de glisser-déposer pour importer des archives et fichiers de sauvegarde (.zip, .json, .csv).
  - Mode prévisualisation sécurisé (*dry-run*) affichant les statistiques de sauvegarde (titres détectés, nouveaux titres à importer, doublons déjà présents ignorés, erreurs) avant confirmation des écritures.
  - Exécution transactionnelle de l'import avec mise en file d'attente automatique des tâches d'enrichissement asynchrones.
- **Harmonisation UI de System Settings & API Keys ([#58](https://github.com/Soviann/trackarr/issues/58))** :
  - En-tête standardisé avec bouton circulaire `←` (retour arrière) et alignement visuel identique aux pages Tâches et Audit.
  - Aération des cartes de sections avec un espacement uniforme de 20px (`gap: 20px`).

## [v1.17.0] — 2026-09-05

### Ajouté
- **Décomposition naturelle du temps de visionnage et filtres avancés sur la page Stats ([#57](https://github.com/Soviann/trackarr/issues/57))** :
  - Décomposition humaine du watch time cumulé en années, jours, heures et minutes (ex: `3 ans 87 j 16 h` en FR / `3 yrs 87 d 16 h` en EN) dans une carte Hero dédiée, avec sous-titre détaillé (`Équivalent à 28 392 heures • 50 251 épisodes`).
  - Ligne de filtres temporels avec bouton `Tout l'historique`, sélecteur compact en pilule `<select>` pour filtrer par année spécifique (généré dynamiquement de la première année en base à l'année courante), et bouton `30 derniers jours`.
  - Ligne de filtres par type de média : `Tous Médias`, `🎬 Films`, `📺 Séries`, `⛩️ Anime`.
  - Filtrage complet backend (`GET /api/stats?timeframe=...&year=...&media_type=...`) sur l'ensemble des métriques, genres, acteurs, réalisateurs, notes et rétrospectives.


## [v1.16.0] — 2026-09-05

### Ajouté
- **Barre de recherche et tiroir de filtres fusionnés en bas de page (`SearchBar` / `FilterDrawer`)** :
  - Champ de recherche docké en bas de l'écran (au-dessus de la barre de navigation) avec bouton d'accès aux filtres et badge de compteur actif (`[ N ]`) directement intégré dans la barre.
  - Tiroir de filtres flottant s'ouvrant au-dessus de la barre de recherche lors de l'activation des filtres.
  - Séparation claire des réinitialisations : bouton `✕` pour effacer uniquement la saisie de texte dans le champ de recherche, et bouton dédié `✕ Réinitialiser` dans l'en-tête du tiroir avec libellé `FILTRES (N ACTIFS)` pour remettre à zéro les critères de tri, de statut et de filtrage sans toucher au texte saisi.
  - Clamping fluide des titres longs sur 2 lignes propres (`-webkit-line-clamp: 2`) sur les cartes de résultats et de la bibliothèque pour éviter les troncatures abruptes.

## [v1.15.0] — 2026-09-05

### Ajouté
- **Notation directe conditionnelle sur la fiche titre (`TitleDetail`)** :
  - Affichage d'une bande tactile de notation 1-tap directe (`1` à `10`) sur la carte de note lorsque le titre n'est pas encore noté, avec enregistrement instantané et synchronisation AniList/IMDb.
  - Affichage épuré de la note (`8/10`) avec bouton d'action *« Modifier »* ouvrant le tiroir de modification/suppression de note lorsque le titre est déjà noté.
- **Module de suivi de Sagas / Univers par titres (`FranchiseRelationsSection`)** :
  - Carte de suivi de franchise enrichie affichant la progression globale basée sur les titres (*« X / Y Titres vus »*) avec barre de progression graduée.
  - Puce encadrée indiquant le prochain titre chronologique à visionner (*« Suivant : ... »*) ou message de complétion intégrale.
  - Bandeau horizontal fluide des opus de la saga avec scrollbar masquée (`scrollbar-width: none`), puces stylisées (vu `✓`, prochain `▶`, à venir) et navigation directe au clic.

## [v1.14.0] — 2026-09-05

### Ajouté
- **Bouton rapide +1 épisode sur Continue Watching & Bibliothèque (`PosterCard` / `PosterTile`)** :
  - Ajout du chargement groupé de l'épisode suivant (`NextEpisode`) dans la requête `ListContinueWatching` du backend Go (`GET /api/titles/continue-watching`).
  - Bouton circulaire d'action rapide `+1` superposé avec espacement symétrique (`bottom: 10px; right: 10px;`) sur les affiches de reprise de lecture et de la grille bibliothèque, conforme à la maquette (`v1.9-ux-ui-improvements.html`).
  - Mise à jour immédiate et optimiste de la barre de progression et du compteur d'épisodes en place avec appel API asynchrone (`PATCH /titles/{id}/episodes/{epId}`).
  - Affichage sous le titre du code de l'épisode à regarder (`S{saison} E{épisode}`).
- **Badge de disponibilité Arr Stack (`DISPO`) & Streaming** :
  - Badge discret `S{saison}E{épisode} DISPO` en haut à droite des affiches si le titre est suivi dans Sonarr/Radarr et possède un épisode disponible.
  - Intégration du composant `WatchProviderBadges` sur les affiches de la bibliothèque en vue grille (`PosterCard`) et en vue liste (`TitleCard`) pour visualiser instantanément la disponibilité des plateformes de streaming configurées (Netflix, Prime Video, Disney+, Apple TV+, Max, Canal+, Crunchyroll, Paramount+, ADN).

## [v1.13.0] — 2026-09-04

### Ajouté
- **Recherche AniList intégrée et raccourcis de recherche pour les saisons d'anime** :
  - Endpoint API `GET /api/anilist/search?query=...` interrogeant l'API GraphQL publique d'AniList pour renvoyer les correspondances d'animes avec jaquettes, titres (anglais et romaji), formats (TV, ONA, Movie, OVA...), années et nombre d'épisodes.
  - Recherche AniList interactive dans la feuille de liaison de saison (`RematchSheet`) : pré-remplissage et recherche instantanée dès l'ouverture, sélection en 1 clic pour lier une saison, et lien rapide *« Search on AniList.co ↗ »* ouvrant la recherche dans un nouvel onglet de navigateur.
  - Conservation de la gestion des parties multiples (ordre et suppression) et de la saisie manuelle d'ID ou d'URL.



## [v1.12.1] — 2026-09-03

### Ajouté
- **Sélecteur d'indexeur et badges sur la page Releases (`/releases`)** :
  - Extraction dynamique des indexeurs Prowlarr disponibles depuis les résultats et ajout d'un menu déroulant interactif pour filtrer la liste par indexeur.
  - Affichage du badge de l'indexeur sur chaque carte de release et dans le panneau de détails (`ReleaseDetailSheet`).

### Corrigé
- **Scintillement rapide (*flickering*) de la liste des releases (`/releases`)** :
  - Mémoïsation de la fonction de traduction `t` dans `useTranslation()` via `useCallback` pour éviter l'instanciation de nouvelles références à chaque rendu, éliminant la boucle infinie de re-fetch et d'affichage des skeletons.
  - Génération de clés composites déterministes `${indexerId}-${guid}` pour empêcher les collisions de clés Preact lorsque plusieurs indexeurs sont activés dans Prowlarr.
  - Internationalisation complète des labels et boutons du composant `ReleaseDetailSheet`.


## [v1.12.0] — 2026-09-01
  - **Expérience interactive multi-diapositives (Story Player)** : Lecteur immersif à 6 diapositives avec défilement automatique, barre de progression dynamique, mise en pause par appui prolongé ou bouton dédié, navigation tactile/clavier et sélecteur d'année.
  - **Diapositive 1 (Vue d'ensemble)** : Métriques annuelles clés (temps total de visionnage avec équivalents parlants en jours/mois/années, titres découverts, épisodes scrobblés, note moyenne, taux de complétion précis).
  - **Diapositive 2 (Coups de Cœur)** : Top 3 par catégorie (Films, Séries TV, Anime) avec médailles, affiches et notes personnelles.
  - **Diapositive 3 (Meilleures Sorties)** : Top des nouveautés parues et visionnées au cours de l'année.
  - **Diapositive 4 (Champion du Revisionnage)** : Calcul rigoureux des véritables cycles de revisionnage (différenciation des scrobbles automatiques Plex/Jellyfin/Emby comptabilisés en intégralité et des marquages manuels en lot/fusions plafonnés à 1, avec détail des épisodes distincts).
  - **Diapositive 5 (Cast, Équipe & Genres)** : Répartition des genres favoris et classement des acteurs et réalisateurs les plus vus de l'année.
  - **Diapositive 6 (Persona IA Gemini)** : Profil cinéphile sur mesure généré par Gemini (titre d'archétype concis, badges, citation marquante, rétrospective synthétique d'une phrase et anecdotes/faits insolites percutants) avec repli déterministe hors-ligne.
  - **Instantanés Immuables en Base & Compilation Automatique au 1er Janvier** : Table SQLite `wrapped_snapshots` figeant les rétrospectives des années passées (aucun recalcul entre consultations). Tâche de fond `generate_wrapped` s'exécutant au 1er janvier pour compiler automatiquement l'année écoulée et envoyer une notification Web Push (`notif_wrapped_ready`).
  - **Galerie d'Archives des Bilans Passés sur `/stats`** : Section dédiée affichant les cartes d'archives de toutes les années précédentes (année, affiche principale, archétype IA, badges de persona, temps total de visionnage et lien direct vers l'histoire).
  - **Bannière d'accès direct sur `/stats`** et support bilingue complet (anglais & français).


## [v1.11.1] — 2026-08-30

### Ajouté
- **Audit et Linter i18n anti-français hardcodé (`make test-front` & `make lint-front`)** :
  - Mise en place d'un moteur d'analyse statique (`frontend/src/i18n/checker.ts`) détectant automatiquement les chaînes de caractères, textes JSX, attributs UI et commentaires en français hors du dictionnaire `locales/fr.ts`.
  - Intégration d'un test Vitest (`src/i18n/i18n-audit.test.ts`) et d'un script CLI (`npm run lint:i18n`) vérifiant à la fois l'absence de texte hardcodé et la parité stricte des clés et paramètres de traduction entre `en.ts` et `fr.ts`.
  - Prise en charge des annotations d'exception inline `// i18n-ignore`.
  - Internationalisation complète de la page des releases (`Releases.tsx`).

## [v1.11.0] — 2026-08-30

### Ajouté
- **Statistiques des Acteurs et Réalisateurs les plus vus (`/stats`)** :
  - Extraction et classement dynamique des 10 acteurs et 10 réalisateurs les plus vus basés sur les crédits de la bibliothèque (`titles.credits`) et l'état de visionnage réel (`last_watched_at` ou statuts `completed` / `watching`).
  - Barres de progression proportionnelles interactives sur la page Statistiques.
  - **Tiroir de filmographie interactif (`PersonFilmographyDrawer`)** : Clic sur un acteur ou un réalisateur pour ouvrir un volet déroulant répertoriant tous les titres associés présents dans la médiathèque locale, avec accès direct aux fiches et lien vers la filmographie complète (`/person/:name`).
- **Internationalisation complète de la page Statistiques (`Stats.tsx`)** :
  - Support multilingue rigoureux (Français & Anglais) via `useTranslation()` couvrant l'intégralité des libellés de cartes, insights, barres de statistiques et dates d'activité récente.


## [v1.10.1] — 2026-08-30

### Modifié
- **Internationalisation Frontend & Uniformisation i18n (`en.ts` / `fr.ts`)** :
  - Extraction de l'ensemble des textes statiques résiduels dans le dictionnaire i18n (`en.ts` pour l'anglais par défaut et `fr.ts` pour le français).
  - Internationalisation complète des composants : `NextEpisodeHero`, `PersonalNotesCard`, `ComingUp`, `CalendarMonthGrid`, `CalendarWeekTimeline`, `CalendarIcalModal`, `FranchiseRelationsSection`, et `SeasonSideStories`.
  - Formatage dynamique des dates et jours de la semaine selon la locale active (`fr-FR` ou `en-US`).
  - Ajout d'une règle d'architecture stricte dans `AGENTS.md` interdisant les chaînes de caractères localisées en dur dans les composants JSX/TSX.

## [v1.10.0] — 2026-08-30

### Ajouté
- **Calendrier In-App Multi-Vues (`/coming-up`)** : Refonte complète de la page des sorties à venir avec un sélecteur de mode de vue à 3 options :
  - **Grille Mensuelle (`CalendarMonthGrid`)** : Calendrier 7 colonnes (Lundi–Dimanche) avec navigation temporelle fluide, retour « Aujourd'hui », pastilles interactives par statut et miniatures de sorties du jour sélectionné.
  - **Frise Hebdomadaire (`CalendarWeekTimeline`)** : Colonnes détaillées pour chaque jour de la semaine avec jaquettes, badge épisode (`S02E08`), titre d'épisode et badges de plateformes de streaming.
  - **Vue Liste Classique** : Préservation du mode grille d'affiches avec badges relatifs (`Today`, `in 5d`) et restauration automatique de la position de défilement (`useScrollRestoration`).
  - **Filtres par Catégorie** : Filtrage instantané par *Tous*, *🎬 Films*, *📺 Séries* et *⛩️ Anime*.
  - **Persistance des préférences** : Sauvegarde automatique du mode de vue favori dans le `localStorage`.
- **Flux d'Abonnement iCal Synchronisé (RFC 5545)** :
  - Endpoint public tokenisé `GET /api/calendar.ics?token=...` servant un flux d'agenda iCalendar conforme (all-day events, `VEVENT`, UIDs déterministes, repliement de lignes à 75 octets et échappement des caractères spéciaux).
  - Modal d'abonnement `CalendarIcalModal` avec copie d'URL 1-clic, boutons directs d'abonnement Apple Calendar (`webcal://`) et Google Calendar, guides d'utilisation pas-à-pas et rotation de token secret (`POST /api/calendar/token/regenerate`).
- **Nouveaux Endpoints Backend** :
  - `GET /api/calendar.ics` : Flux calendrier RFC 5545.
  - `GET /api/calendar/events` : Données d'événements calendaires enrichies pour l'interface utilisateur.
  - `GET /api/calendar/token` & `POST /api/calendar/token/regenerate` : Gestion et rotation du token de calendrier dans SQLite `settings`.

## [v1.9.0] — 2026-08-30

### Ajouté
- **Watch Providers Configurables (`WatchProviderBadges`)** : Remplacement de l'ancien badge statique Prime par un composant multi-plateformes dynamique prenant en charge 9 services de streaming reconnus (Netflix, Amazon Prime Video, Disney+, Apple TV+, Max, Canal+, Crunchyroll, Paramount+, Animation Digital Network).
- **Gestion des plateformes actives dans les Paramètres (`/admin/settings`)** : Nouvelle section permettant d'activer ou désactiver individuellement chaque plateforme de streaming avec prévisualisation des badges colorés.
- **Support complet multi-écrans** : Affichage des badges de streaming sur la fiche titre (`TitleDetail`), les cartes compactes de reprise de lecture (`ContinueWatching`), et les sorties à venir (`ComingUp`).
- **Synchronisation backend des préférences** : Clé de paramètre `enabled_watch_providers` stockée dans SQLite, exposée dans `/api/config`, `/api/settings` et `/api/admin/system-settings` avec rechargement à chaud.

## [v1.8.1] — 2026-08-30

### Modifié
- **Épuration de la fiche titre (`NextEpisodeHero`)** : Suppression du bandeau redondant de félicitations lorsque tous les épisodes sont vus, l'état étant déjà clairement indiqué par le badge et le statut de la série.

## [v1.8.0] — 2026-08-30

### Ajouté
- **Bouton Héroïque Épisode Suivant (`NextEpisodeHero`)** : Bouton d'action proéminent sur la fiche titre permettant de marquer le prochain épisode non visionné comme vu d'un seul clic (`▶ Marquer S02E06 comme vu`), sans avoir à déplier la liste des saisons.
- **Calculateur de Binge & Temps restant** : Estimation en temps réel de la durée nécessaire pour rattraper une série en cours (*« ⏱️ Reste ~3h 15m (4 ép.) »*) ou de la durée totale estimée pour les œuvres en liste à voir (*Plan to Watch*) et les films.
- **Notes Personnelles (`PersonalNotesCard`)** : Bloc de texte épuré sur chaque fiche titre avec auto-sauvegarde asynchrone débouncée (500ms) permettant d'ajouter et d'éditer des notes, mémos ou recommandations privées.
- **Migration de schéma SQLite (043)** : Ajout de la colonne `personal_notes TEXT` sur la table `titles`, intégrée aux modèles Go, à la lecture/écriture transactionnelle (`TitleWriter`), à l'endpoint `PATCH /api/titles/{id}` et à la fusion de titres (`Merge`).

### Documentation
- **Architecture des tâches d'arrière-plan et Task Queue (`docs/background-jobs.md`)** : Documentation exhaustive du service de rafraîchissement quotidien (`BackgroundService`), de la file de tâches asynchrone SQLite (`TaskQueueWorker`), du cycle de vie des tâches (`pending`, `running`, `sleeping`, `dead`), du rate limiter partagé (`APILimiter`) et du protocole d'arrêt gracieux (`shutdownWG`).

## [v1.7.0] — 2026-08-30

### Ajouté
- **Sagas de films (TMDB Collections)** : Détection et affichage de l'ensemble des films d'une même saga cinématographique (ex: *Harry Potter*, *Dune*, *Le Seigneur des Anneaux*, *Star Wars*, *Fast & Furious*) dans la section « Saga & Collection », avec correspondance automatique des films possédés et bouton `[+ Ajouter]` pour les manquants.
- **Univers de séries & Spin-offs (TheTVDB Franchises)** : Regroupement et affichage des séries préquelles, suites et spin-offs rattachés à un univers télévisé (ex: *Breaking Bad* ↔ *Better Call Saul* ↔ *El Camino*, *Game of Thrones* ↔ *House of the Dragon*).
- **Sélecteur d'ordre de visionnage (Chronologique vs Date de sortie)** : Bascule dynamique permettant de visionner une saga ou un univers soit par ordre chronologique de l'histoire (⏱️ Chronologie), soit par ordre de diffusion historique (📅 Sortie).
- **Intégration visuelle en carte standard & Repliage par défaut** : Formatage harmonieux au design de la fiche titre (`.card` avec fond élevé, bordure et typographie standard) et repliage automatique au-dessus de 3 résultats avec bouton interactif « Voir plus (+N) » / « Voir moins ».
- **Intégration au rafraîchissement unitaire et global** : Récupération et mise à jour automatique des sagas TMDB et univers TVDB lors du rafraîchissement d'une fiche (`POST /api/titles/:id/refresh`) et lors du cycle global (`POST /api/admin/refresh-all` et cron).
- **Réconciliation multi-providers unifiée** : Extension de la jointure SQL de relations pour réconcilier en simultané les identifiants AniList, TMDB et TheTVDB avec l'état de la bibliothèque locale.

## [v1.6.0] — 2026-08-30

### Ajouté
- **Side Stories & Films AniList dans la timeline des saisons** : Intégration directe des films et OAVs rattachés à la fin des épisodes de la saison parente (ex: *My Hero Academia: Two Heroes* affiché chronologiquement après la Saison 2), avec support de plusieurs hors-séries par saison, statuts de visionnage et bascule rapide.
- **Section Univers & Franchise** : Ajout d'une section dédiée sur la fiche des animes récapitulant l'ensemble des films, OAVs, résumés et spin-offs reliés via AniList, avec filtres par catégorie (*Tous*, *Films*, *OAVs*, *Spin-offs*) et indicateurs de suivi.
- **Ajout direct des titres manquants** : Bouton d'action direct `[+ Ajouter]` sur les cartes et blocs de side stories absentes de la médiathèque redirigeant automatiquement vers l'écran d'ajout/validation (`/admin/validate`) pré-rempli avec l'entrée AniList.
- **Réconciliation automatique & Push AniList** : Détection dynamique des films et hors-séries présents dans la médiathèque locale avec synchronisation croisée du statut de visionnage vers AniList.

### Corrigé
- **Content-Security-Policy pour les jaquettes distantes** : Prise en charge des domaines CDN AniList (`s4.anilist.co`, `*.anilist.co`) et TVDB (`artworks.thetvdb.com`) dans la directive `img-src` de la CSP pour l'affichage direct des affiches externes.

## [v1.5.0] — 2026-08-29

### Ajouté
- **Langue des métadonnées configurable** : Ajout d'un paramètre dédié dans l'administration (`/admin/settings`) permettant de choisir la langue source prioritaire pour les titres et métadonnées (Français, Anglais, Allemand, Espagnol, Italien, Portugais, Japonais), distinct de la langue de l'interface utilisateur.
- **Support multilingue étendu (TMDB & TVDB)** : Extraction et enrichissement des traductions de titres dans plusieurs langues lors des recherches et de la synchronisation en arrière-plan.
- **Résolution dynamique des titres & alternatives** : Priorisation automatique de la langue configurée pour le titre principal et classement adapté des titres alternatifs sur l'ensemble de l'interface.

## [v1.4.0] — 2026-08-24

### Ajouté
- **Scripts de pré-lancement & init Docker** : Prise en charge des scripts personnalisés utilisateur et root (`/docker-entrypoint-init.d/`, `/data/init.d/`) lors du démarrage du conteneur.
- **Audit de saisons & Consolidation** : Détection intelligente des faux doublons, consolidation simplifiée des saisons et tri déterministe dans l'outil d'audit (`/admin/season-audit`).
- **Documentation exhaustive & Aide intégrée** : Ajout d'une page d'aide in-app (`/help`), d'un guide d'installation pour toutes plateformes (`docs/deployment.md`) et de spécifications techniques (`docs/dev/*`).

### Sécurité & Infrastructure
- **Préparation Open-Source** : Standardisation du `docker-compose.yml` public (`ghcr.io/soviann/trackarr`), suppression de tous les résidus d'infrastructure privée et sécurisation des workflows GitHub Actions.
- **Refactoring & Qualité** : Migration du namespace de module Go vers `github.com/Soviann/trackarr`, centralisation des constantes de configuration et standardisation des parsers JSON `httputil`.

## [v1.3.4] — 2026-08-24

### Corrigé
- **Restauration de défilement & Navigation** : Préservation rigoureuse de la position de défilement dans les listes (`Library`, `ComingUp`, `ContinueWatching`, `Search`) lors du retour arrière (`Back`) tout en garantissant l'ouverture au sommet (`scrollY = 0`) lors de la navigation vers une nouvelle page ou fiche titre.

## [v1.3.3] — 2026-08-24

### Corrigé
- **Défilement lors des changements de page** : Réinitialisation systématique de la position de défilement en haut de page (`window.scrollTo(0, 0)`) lors des navigations SPA (notamment vers la page d'aide et les pages d'administration).

## [v1.3.2] — 2026-08-24

### Modifié
- **Tableau de bord Admin** : Affichage de 'Arr Stack' en titre principal et 'Radarr / Sonarr / Prowlarr' en sous-titre sur la carte d'intégration.

## [v1.3.1] — 2026-08-24

### Modifié
- **Carte Releases & Prowlarr** : Remplacement de l'intitulé 'Explore C411' par 'Explore Prowlarr' sur la carte d'accueil de la bibliothèque et neutralisation du nom d'indexeur par défaut dans la fiche de release et l'aide.

## [v1.3.0] — 2026-08-24

### Ajouté
- **Gestion des options Radarr & Sonarr** : Ajout d'un drawer interactif complet permettant de consulter et mettre à jour les options d'un média (profil de qualité, dossier racine, statut surveillé, recherche immédiate) directement depuis l'application via `GET/PUT /api/arr/title/:id`.
- **Réorganisation des sources externes** : Déplacement des boutons d'accès direct (IMDb, TMDB, TVDB, AniList) dans la section Détails de la fiche titre.

### Corrigé
- **Liens directs Radarr & Sonarr** : Résolution des erreurs 404 en construisant dynamiquement les URLs basées sur le `titleSlug` et l'instance configurée (`/movie/{slug}` et `/series/{slug}`).
- **Affichage dynamique des Releases** : Masquage automatique de la carte Releases et répartition à 50% des cartes restantes sur la bibliothèque lorsque Prowlarr n'est pas configuré.

## [v1.2.1] — 2026-08-24

### Corrigé
- **Orientation du Progress Ring Orbital** : Inversion et calage de la progression dans le sens horaire partant de 12h (midi) en trait plein (65%) et se terminant par les pointillés (35%) sur tous les assets vectoriels, favicons et icônes PWA.

## [v1.2.0] — 2026-08-24

### Corrigé
- **Authentification par mot de passe local** : Prise en charge complète des sessions JWT en mode mot de passe seul lorsque `GOOGLE_ALLOWED_EMAIL` n'est pas configuré, empêchant le rejet erroné des requêtes par le middleware `JWTAuth`.

### Modifié
- **Audit de qualité & Refactoring DRY** : Factorisation de l'émission et de la pose des cookies de session JWT (`issueAuthCookie`), centralisation des déclenchements de rafraîchissement asynchrone (`asyncRefresh`), et standardisation du parsing des paramètres et requêtes d'administration via `httputil`.

### Corrigé
- **Icônes PWA d'installation (192, 512, Maskable)** : Regénération complète de tous les fichiers PNG d'icônes en véritable 32-bit RGBA avec le monogramme TR et anneau orbital dans la zone de sécurité (safe zone 70%) sur fond `#0b0f19`, et ajout de cache-busting `?v=2` dans `manifest.json`.

## [v1.1.0] — 2026-08-23

### Ajouté
- **Abstraction Multi-IA (`AIProvider`)** : Formalisation de l'interface `AIProvider` (`VerifyMatch`, `FuzzyResolve`, `IdentifyAnimeSeason`) permettant de brancher facilement d'autres moteurs d'IA (OpenAI, Claude, Ollama) en plus de Gemini.
- **En-tête de marque Trackarr** : Ajout de la typographie de marque stylisée avec dégradé thématique dynamique dans l'en-tête de la bibliothèque (`Trackarr / Library`).
- **Guide des scripts personnalisés** : Section dédiée dans la documentation de déploiement pour exécuter des scripts de maintenance ou de backup personnalisés via `docker exec`.

### Corrigé
- **Favicons transparents pour onglets Chrome** : Suppression du fond sombre sur `favicon.svg`, `favicon.ico` et les PNGs multi-tailles pour un affichage net et contrasté quel que soit le thème du navigateur.
- **Sécurité du démon NAS Antigravity** : Autorisation explicite des bots CI vérifiés (`github-actions[bot]`, `dependabot[bot]`) pour permettre l'auto-réparation automatique des dépendances tout en conservant le verrouillage strict HMAC et dépôts.
- **Scanners de secrets & CLI** : Renommage de la variable d'environnement de réinitialisation de mot de passe en `TRACKARR_ADMIN_PASSWORD` pour éliminer les faux positifs GitGuardian.
- **Nettoyage Plextracker** : Suppression de toutes les mentions résiduelles dans les fichiers de configuration, les modèles, les services et la documentation technique.

## [v1.0.1] — 2026-08-23

### Corrigé
- **Icônes PWA Android & Favicon Desktop** : Ajout du fichier `favicon.ico` multi-résolutions et paramètres de cache-busting dans `index.html` pour Chrome Desktop.
- **Support Maskable Icon Android** : Refonte de `icon-maskable.png` avec fond plein sans transparence et centrage dans la zone de sécurité (safe zone 75%) pour garantir la génération et l'affichage de l'icône sous Android WebAPK.

## [v1.0.0] — 2026-08-23

### Ajouté
- **Rebranding officiel Trackarr** : Transition open-source complète du projet avec nouveau nom, identité visuelle et typographique.
- **Logo officiel Monogramme "TR"** : Monogramme géométrique avec lettres distinctes resserrées et anneau orbital de progression 60% plein / 40% pointillés.
- **Sélecteur de 4 thèmes in-app** : *Cyber Cyan*, *Sunset Coral*, *Emerald Teal*, et *Vault Amber* sélectionnables en temps réel dans `/admin/settings`.
- **Publication Docker Multi-Arch (GHCR)** : Workflows GitHub Actions pour compiler et publier automatiquement les images `linux/amd64` et `linux/arm64` sur GitHub Container Registry (`ghcr.io/soviann/trackarr`).
- **Support dynamique `PUID` / `PGID`** : Compatibilité totale avec les environnements Synology, Unraid, TrueNAS, Docker Compose sans conflit de permissions.
- **Commande et package Version** : CLI `trackarr version` et injection de version au build (`-ldflags`) exposée sur `/api/health`.
- **Gouvernance & Templates Open Source** : Licence MIT, `SECURITY.md`, `CODE_OF_CONDUCT.md`, templates d'issues et de pull requests GitHub.
- **Documentation complète en anglais** : Nouveau `README.md` vitrine, `CONTRIBUTING.md`, guides utilisateur et développeur précisant la conception PWA Android-first et l'architecture Go ultra-légère (~30 Mo RAM).

### Modifié
- Renommage du module Go : `github.com/Soviann/plextracker` ➔ `github.com/Soviann/trackarr`.
- Détection et rétrocompatibilité automatique de la base de données (`plextracker.db` conservé sans migration manuelle si existant, `trackarr.db` par défaut pour les nouvelles installations).

### Corrigé
- Nettoyage automatique des saisons et épisodes résiduels lors de la conversion d'une série en film, et réinitialisation de l'affiche pour forcer le rafraîchissement des métadonnées du film.
- Réinitialisation de l'affiche lors du rematch manuel pour télécharger l'affiche du nouveau titre associé.

## [v0.46.0] — 2026-08-23

### Ajouté
- Envoi direct vers Radarr et Sonarr via un tiroir coulissant (Drawer / `ArrPushSheet`) depuis la fiche d'un titre et le menu d'actions, avec sélection du dossier racine et du profil de qualité.

### Supprimé
- Suppression de la file d'attente globale Radarr / Sonarr (`Arr Queue`) et de sa page d'administration au profit de l'envoi direct synchrone.

## [v0.45.0] — 2026-08-22

### Ajouté
- Workflow GitHub Actions hebdomadaire (`weekly-docs-audit.yml`) vérifiant l'activité récente et déléguant l'audit de la documentation à Antigravity.
- Activation de la suppression automatique des branches après fusion (`delete_branch_on_merge` et `--delete-branch`).

## [v0.44.3] — 2026-08-22

### Corrigé
- Préservation des métadonnées, du statut de file d'attente Arr et de l'état de visionnage lors de la fusion de titres (`TitleWriter.Merge`).
- Harmonisation complète et audit de l'écosystème d'instructions LLM, des agents et des skills (gestion du changelog, plans sous `docs/plans/`, Chrome DevTools).

## [v0.44.2] — 2026-08-22

### Corrigé
- Correction du traitement des résolutions d'identifiants externes et fiabilisation du pipeline de matching.

## [v0.44.1] — 2026-08-15

### Amélioré
- Optimisation des requêtes de statistiques et affinage de l'interface mobile PWA.

## [v0.44.0] — 2026-08-01

### Ajouté
- Support multi-parts pour les saisons AniList et enrichissement des informations de diffusion.
