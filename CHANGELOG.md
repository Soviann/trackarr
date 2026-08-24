# Journal des modifications (Changelog)

Toutes les modifications notables apportées à ce projet sont consignées dans ce fichier.

Le format est basé sur [Keep a Changelog](https://keepachangelog.com/fr/1.1.0/) et ce projet adhère à la gestion sémantique de version ([Semantic Versioning](https://semver.org/lang/fr/)).

## [Unreleased]

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
