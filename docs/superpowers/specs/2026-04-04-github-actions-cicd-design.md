# GitHub Actions CI/CD — Spec

## Contexte

PlexTracker n'a qu'un CI basique (job unique séquentiel) et un workflow de release qui publie sur ghcr.io. Le dépôt doit rester **privé** (pas de registry d'images). On veut un pipeline complet : CI parallélisée, releases GitHub avec changelog, et déploiement automatique sur le NAS Synology via SSH + build local.

Inspiré du setup Bibliotheque, adapté aux spécificités PlexTracker (app Go single-container, migrations auto au démarrage, pas de post-deploy commands).

---

## Workflow 1 : CI (`ci.yml`)

**Déclenchement** : PR vers main + push sur main

**4 jobs parallèles** (remplace le job unique actuel) :

| Job | Contenu |
|-----|---------|
| **test-backend** | Go 1.24 → `go test -tags sqlite_fts5 ./... -count=1` |
| **lint-backend** | Go 1.24 → golangci-lint (action officielle) |
| **build-frontend** | Node 22 → `npm ci` → `npx vite build` |
| **test-frontend** | Node 22 → `npm ci` → `npx vitest run` |

Note : le tag `sqlite_fts5` manque dans le CI actuel et est nécessaire pour la compilation.

---

## Workflow 2 : Release (`release.yml`)

**Déclenchement** : push de tag `v*`

1. Extraire la section correspondante du `CHANGELOG.md` (pattern `## [vX.Y.Z]`)
2. Créer une GitHub Release via `gh release create` avec les notes extraites

Nécessite un fichier `CHANGELOG.md` à la racine du projet.

---

## Workflow 3 : Deploy (`deploy.yml`)

**Déclenchement** : push de tag `v*`

**Job unique `deploy`** :
1. SSH sur le NAS via `appleboy/ssh-action`
2. Exécuter `/volume1/docker/plextracker/scripts/nas-update.sh`
3. En cas d'échec :
   - Exécuter `nas-diagnostics.sh` via SSH
   - Capturer la sortie dans un fichier local sur le runner
   - Uploader comme artifact GitHub (rétention 90 jours)

---

## Script NAS : `nas-update.sh`

Adapté de Bibliotheque, simplifié (single container, pas de post-deploy commands).

1. `git fetch --tags origin`
2. Trouver le dernier tag semver, comparer au tag déployé
3. Si même tag + conteneur running → skip. Si conteneur down → redéployer
4. `git checkout <tag>` → `docker compose up -d --build --wait`
5. Nettoyer les anciennes images Docker (garder uniquement la version courante)
6. En cas d'échec : rollback sur les 5 tags précédents
7. Si rollback échoue aussi : message d'erreur critique

Différences clés vs Bibliotheque :
- `docker compose up --build` au lieu de `pull` (pas de registry)
- Pas de `cache:clear`, `migrations:migrate`, `warm-thumbnails` (migrations auto au boot)
- Vérification d'1 conteneur (pas 3)
- Pas de `--env-file` séparé (utilise `.env` + `.env.local` standard)

---

## Script NAS : `nas-diagnostics.sh`

Collecte d'infos de debug en cas d'échec :

1. État des conteneurs (`docker compose ps -a`)
2. Logs Docker (100 dernières lignes)
3. Événements OOM/kill (dmesg)
4. Espace disque (`df -h /volume1`)
5. Dernier log de mise à jour

Sortie vers stdout (visible dans Actions) + fichier log sur le NAS.

---

## Artifacts en cas d'échec

Le job deploy capture la sortie du script de diagnostics et l'uploade comme artifact GitHub. Double visibilité : logs Actions immédiat + fichier téléchargeable pendant 90 jours.

---

## Secrets GitHub requis

| Secret | Usage |
|--------|-------|
| `NAS_HOST` | Hostname SSH du NAS |
| `NAS_SSH_USER` | Utilisateur SSH |
| `NAS_SSH_KEY` | Clé privée SSH |
| `NAS_SSH_PORT` | Port SSH |

`GITHUB_TOKEN` automatique pour la création de releases.

---

## Fichiers à créer/modifier

| Fichier | Action |
|---------|--------|
| `.github/workflows/ci.yml` | Remplacer — jobs parallèles, ajout tag sqlite_fts5 |
| `.github/workflows/release.yml` | Remplacer — extraction changelog + GitHub Release (supprimer le build Docker) |
| `.github/workflows/deploy.yml` | Créer — SSH deploy vers NAS |
| `scripts/nas-update.sh` | Créer |
| `scripts/nas-diagnostics.sh` | Créer |
| `CHANGELOG.md` | Créer — version initiale |

---

## CHANGELOG.md

Format keepachangelog, sections en français :

```markdown
# Changelog

## [v0.1.0] — 2026-04-04

Version initiale avec CI/CD.

### Ajouté
- Pipeline CI GitHub Actions (tests, lint, build)
- Déploiement automatique sur NAS via SSH
- Scripts de mise à jour et diagnostics NAS
```
