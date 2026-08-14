# Documentation PlexTracker — Index Général

Bienvenue dans la documentation officielle de PlexTracker. Chaque aspect de l'application est documenté séparément dans les guides ci-dessous.

---

## 📖 Sommaire

### 1. [Aperçu & Accès](overview.md)
- Présentation générale et objectifs du projet
- Périmètre (ce que fait / ne fait pas PlexTracker)
- Accès PWA sur mobile (installation Chrome, raccourcis, badge d'icône)

### 2. [Guide de l'Interface Utilisateur](interface.md)
- **Bibliothèque (Accueil)** : Filtres (statut, type, pays, note), tri, mode sélection, pull-to-refresh, Continue Watching & Coming Up.
- **Détail d'un Titre** : Notes, synopsis, cast & crew, raccourcis Arr, historique, épisodes, bandeaux AniList par saison.
- **Recherche & Ajout** : Recherche par nom, collage d'URLs (IMDb/TMDB/AniList/TVDB), partage natif Android/iOS.
- **Match Review** : Validation des correspondances d'IDs externes, swipe actions, batch confirm.
- **Notation & Édition** : Prompt de notation contextuel, modification du statut/type/nom affiché.
- **Statistiques** : Insights de visionnage, distribution des notes, records et flux d'activité récente.

### 3. [Intégrations & Webhooks](integrations.md)
- **Jellyfin Webhook** : Configuration pas-à-pas du plugin Webhook, template JSON et déduplication.
- **AniList** : Connexion OAuth, synchronisation bidirectionnelle automatique (notes, statuts, progression), saisons découpées (*parts*), et rattachement des prequels.

### 4. [Tâches de Fond & Automation](background-jobs.md)
- **Rafraîchissement Quotidien** : Mise à jour automatique des métadonnées, nouveaux épisodes, statut de séries et couvertures.
- **Pipeline de Matching Média** : Résolution des IDs externes (cross-référence, TMDB, AniList, Gemini AI).
- **Season Audit (Admin)** : Outil de détection et de fusion guidée des saisons éclatées.

### 5. [Déploiement & Administration](deployment.md)
- **Déploiement Synology NAS** : Docker Compose, variables d'environnement, mises à jour.
- **Import & Réimport Simkl** : Import initial de l'historique, dry-run, réinitialisation de base de données.
- **Guide des Commandes Makefile** : Commandes de développement, tests, linting et synchronisation NAS.

### 6. [Maintenance & CI](maintenance.md)
- **Dependabot** : Configuration des mises à jour hebdo groupées et des alertes de sécurité immédiates.
- **Workflows CI/CD** : Auto-merge des dépendances validées et correction automatique par l'agent AI (Antigravity).

---

## 🛠 Guides Développeurs (Architecture & Standards)

Pour contribuer au code ou comprendre les conventions techniques internes :
- [**Architecture & Patterns**](patterns.md) — Architecture du code (Go / Preact), conventions de nommage, gestion des erreurs et base de données.
