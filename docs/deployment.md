# Déploiement & Administration

[← Retour à l'index](INDEX.md)

---

## 1. Déploiement Docker (NAS Synology)

PlexTracker s'exécute dans un conteneur Docker unique sur un NAS Synology (ex: DS920+).

### Mise à jour en production

```bash
docker compose pull && docker compose up -d
```

### Variables d'environnement
Le conteneur utilise les variables d'environnement configurées dans le fichier `.env` (valeurs par défaut) et `.env.local` (secrets) :
- `GOOGLE_CLIENT_ID`, `GOOGLE_ALLOWED_EMAIL`
- `JWT_SECRET`
- `TMDB_API_KEY`
- `ANILIST_CLIENT_ID`, `ANILIST_CLIENT_SECRET`
- `GEMINI_API_KEY`
- `JELLYFIN_WEBHOOK_SECRET`
- `RADARR_URL`, `RADARR_API_KEY` *(optionnels si configurés dans l'admin)*
- `SONARR_URL`, `SONARR_API_KEY` *(optionnels si configurés dans l'admin)*
- `VAPID_PUBLIC_KEY`, `VAPID_PRIVATE_KEY`, `VAPID_SUBJECT` *(Web Push notifications)*

---

## 2. Import & Migration Simkl

L'import initial d'un fichier de sauvegarde Simkl s'effectue via le Makefile.

### Prévisualisation (Dry-run)

```bash
make import-dry BACKUP_FILE=/chemin/vers/Simkl_backup.zip
```

### Import effectif

```bash
make import BACKUP_FILE=/chemin/vers/Simkl_backup.zip
```

### Réinitialisation et réimport complet

Pour vider la base de données et effectuer un réimport propre :

```bash
# En local
make reset-import BACKUP_FILE=/chemin/vers/Simkl_backup.zip

# Sur le NAS via SSH (fichier dans /volume1/downloads/)
make ssh-reset-import BACKUP_FILE=Simkl_backup.zip
```

---

## 3. Guide des Commandes Makefile

Toutes les opérations se font via le Makefile (qui orchestre les exécutions dans le conteneur Docker).

| Commande | Description |
|---|---|
| `make up` | Démarrer l'environnement de développement local |
| `make down` | Arrêter l'environnement local |
| `make logs` | Suivre les logs du conteneur de dev |
| `make shell` | Ouvrir un terminal bash dans le conteneur |
| `make test` | Exécuter la suite de tests unitaires Go |
| `make test-front` | Exécuter les tests Vitest et le build Vite frontend |
| `make lint` | Lancer `golangci-lint` sur le code Backend |
| `make lint-front` | Lancer la vérification de types TypeScript (`tsc`) |
| `make fmt` | Formater le code Go (`gofmt`) |
| `make dev-frontend` | Démarrer le serveur de développement Vite (hot reload) |
| `make build` | Compiler le binaire de production Go |
| `make migrate` | Exécuter les migrations de base de données |
| `make backfill-accents` | Extraire et stocker les couleurs d'accent des couvertures |
| `make db-reset` | Réinitialiser la base SQLite locale |
| `make ssh-db-pull` | Télécharger la base de production (`plextracker.db` + WAL/SHM) du NAS vers le local |
| `make ssh-logs` | Télécharger les logs du conteneur de production dans `data/plextracker.log` |
| `make ssh-debug-pull` | Télécharger à la fois la BDD et les logs de prod en local pour débogage |
| `make push-secrets` | Synchroniser `.env.local` vers le NAS |
| `make pull-secrets` | Récupérer `.env.local` depuis le NAS |
