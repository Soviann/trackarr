# PlexTracker

PlexTracker est une application personnelle de suivi de visionnage de films, séries et animes. Conçue comme une Progressive Web App (PWA) optimisée pour mobile, elle remplace Simkl en s'intégrant directement avec Jellyfin, Plex, TMDB, TVDB et AniList.

---

## 🚀 Fonctionnalités clés

- **Suivi automatique** : Enregistrement automatique des visionnages via webhooks Jellyfin et Plex.
- **Gestion de bibliothèque** : Suivi des statuts (*Watching*, *Plan to watch*, *Caught up*, *Completed*, *Dropped*).
- **Synchronisation AniList** : Synchronisation bidirectionnelle automatique des notes, statuts et de la progression par saison/épisode.
- **Match Review intelligent** : Pipeline d'identification des médias assisté par Gemini AI avec file de revue manuelle pour les cas ambigus.
- **Gestion des saisons et Animes** : Fusion automatique des saisons, support des saisons découpées en plusieurs *parts*, et outil d'audit des saisons.
- **File Arr (Radarr / Sonarr)** : Intégration directe pour suivre et ajouter des titres à la file de téléchargement.
- **Statistiques & Insights** : Tableau de bord de statistiques détaillées et d'insights de visionnage.

---

## 🛠 Stack Technique

- **Backend** : Go 1.24, Chi router, SQLite (`sqlite_fts5`)
- **Frontend** : Preact 10, TypeScript, Vite, Vanilla CSS (Design system HSL)
- **Infrastructure** : Docker & Docker Compose
- **APIs & Intégrations** : TMDB, TVDB, AniList OAuth, Gemini AI, Webhooks Jellyfin/Plex

---

## 💻 Démarrage Rapide

### Prérequis
- Docker et Docker Compose
- Make

### Lancement en développement

```bash
# Démarrer le conteneur applicatif (Go backend)
make up

# Démarrer le serveur dev frontend (Vite)
make dev-frontend
```

L'application est accessible sur `http://localhost:8080`.

### Commandes Makefile principales

```bash
make test          # Lancer les tests unitaires Go
make test-front    # Lancer les tests Vitest + build Vite
make lint          # Lancer golangci-lint
make lint-front    # Lancer le type-check TypeScript
make build         # Compiler le binaire Go
```

---

## 📚 Documentation Détaillée

Toute la documentation du projet est organisée de manière modulaire dans le dossier [`docs/`](docs/INDEX.md) :

1. [**Aperçu & Accès**](docs/overview.md) — Présentation, périmètre et installation PWA.
2. [**Guide de l'Interface**](docs/interface.md) — Bibliothèque, fiches titres, recherche, fusions, notation et statistiques.
3. [**Intégrations & Webhooks**](docs/integrations.md) — Configuration Jellyfin, Plex et synchronisation AniList.
4. [**Tâches de Fond & Matching**](docs/background-jobs.md) — Rafraîchissement quotidien, pipeline de matching Gemini AI et audit des saisons.
5. [**Déploiement & Administration**](docs/deployment.md) — Déploiement NAS, import Simkl et commandes Makefile.
6. [**Maintenance & CI**](docs/maintenance.md) — Dependabot, auto-merge et correction automatique AI.

Consultez l'[Index Général de la Documentation](docs/INDEX.md) pour naviguer facilement.
