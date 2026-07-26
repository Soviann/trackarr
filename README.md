# PlexTracker

Application personnelle de suivi de visionnage. Remplace Simkl comme tracker central pour les films, séries et anime regardés sur Jellyfin.

## Fonctionnalités

- **Suivi automatique Jellyfin** via webhooks (scrobble à la fin de visionnage)
- **Bibliothèque** avec filtres par statut (en cours, terminé, abandonné, à regarder)
- **Progression** en un coup d'œil : barre de progression, prochain épisode
- **Notes** (1-10) par titre et par saison, avec liens IMDb et sync AniList
- **Ajout manuel** par lien (IMDb, TVDB, AniList) ou recherche par nom
- **Partage Android** (PWA Share Target) depuis les apps IMDb, navigateur, etc.
- **Notifications push** pour les fins de saison/film et les changements de statut
- **Import Simkl** pour la migration initiale de l'historique

## Stack technique

| Composant | Technologie |
|---|---|
| Backend | Go 1.24, chi router, SQLite (WAL) |
| Frontend | Preact 10, Vite, TypeScript |
| Auth | Google OAuth, JWT (cookie HttpOnly) |
| APIs externes | TMDB, AniList (GraphQL), Gemini AI |
| Déploiement | Docker, Synology DS920+ |

## Développement local

Prérequis : Docker (via limactl ou Docker Desktop).

```bash
# Démarrer l'environnement de dev
make up

# Logs
make logs

# Shell dans le conteneur
make shell

# Tests Go
make test

# Tests frontend
make test-front

# Linter
make lint

# Arrêter
make down
```

Toutes les commandes passent par le Makefile qui exécute dans le conteneur Docker. Ne jamais lancer `go`, `node` ou `npm` directement sur l'hôte.

## Import Simkl

```bash
# Copier le backup dans le conteneur puis :
make import-dry BACKUP_FILE=/chemin/vers/Simkl_backup.zip  # prévisualisation
make import BACKUP_FILE=/chemin/vers/Simkl_backup.zip       # import réel
```

## Déploiement

```bash
# Build de l'image
docker build -t ghcr.io/nicolasvasse/plextracker:latest .

# Production sur le NAS
docker compose pull && docker compose up -d
```

Configuration via variables d'environnement — voir `docker-compose.yml`.

## Documentation

- [Guide utilisateur](docs/user-guide.md) — fonctionnement de l'application
- [Design spec](docs/superpowers/specs/2026-04-01-plextracker-design.md) — spécifications techniques
- [UI/UX spec](docs/superpowers/specs/2026-04-02-plextracker-ui-design.md) — design visuel et interactions
- [Patterns](docs/patterns.md) — carte du codebase et conventions
