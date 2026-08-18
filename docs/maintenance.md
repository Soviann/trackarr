# Maintenance & Intégration Continue (CI/CD)

[← Retour à l'index](INDEX.md)

---

## 1. Automation des Dépendances avec Dependabot

Le projet utilise GitHub Dependabot pour maintenir à jour les dépendances Go, npm, GitHub Actions et Docker.

### Configuration (`.github/dependabot.yml`)

- **Mises à jour de version régulières** : Vérifiées chaque semaine et regroupées en PRs hebdomadaires via `applies-to: version-updates`.
- **Mises à jour de sécurité immédiates** : Les vulnérabilités signalées (CVE/GHSA) contournent le regroupement hebdomadaire et déclenchent immédiatement une Pull Request autonome dédiée dès leur publication.

```yaml
version: 2
updates:
  - package-ecosystem: "gomod"
    directory: "/"
    schedule:
      interval: "weekly"
    groups:
      go-dependencies:
        applies-to: version-updates
        patterns: ["*"]

  - package-ecosystem: "npm"
    directory: "/frontend"
    schedule:
      interval: "weekly"
    groups:
      frontend-dependencies:
        applies-to: version-updates
        patterns: ["*"]
```

---

## 2. Workflows CI/CD GitHub Actions

### Integration Continue (`.github/workflows/ci.yml`)
Chaque commit et Pull Request déclenche automatiquement :
- Les lints Backend (`golangci-lint`) et Frontend (`tsc`).
- La suite de tests Backend Go (`make test`) et Frontend Vitest (`make test-front`).
- Le build de production Go et Vite.

### Auto-Merge Dependabot (`.github/workflows/dependabot-auto-merge.yml`)
Lorsqu'une PR est ouverte par Dependabot, ce workflow active la fusion automatique (`gh pr merge --auto --squash`). Dès que la CI passe au vert, la PR est mergée sans intervention humaine.

### Auto-Fix AI (`.github/workflows/dependabot-antigravity-fix.yml`)
Si la CI échoue sur une PR Dependabot (par exemple à cause d'un *breaking change* dans une mise à jour) :
1. Le workflow détecte l'échec de la CI.
2. Il poste automatiquement un commentaire `/antigravity` sur la PR pour alerter le démon AI.
3. Le démon analyse le problème, prépare un plan d'implémentation et une proposition de résolution.

### Démon Webhook Antigravity NAS (`scripts/github-pr-daemon/`)
Le conteneur `plextracker-antigravity` tourne en tâche de fond sur le NAS (port 8191) :
- **Rôle** : Réceptionner les webhooks GitHub (commandes `/antigravity`, `/plextracker`), synchroniser les branches Git, analyser le code et les logs locaux de production (`/data/plextracker.log`), et générer via Gemini 3.6 Flash un diagnostic et un plan d'action détaillé.
- **Périmètre d'exécution** : Le conteneur du démon est minimaliste (Python + Git) et ne dispose ni de Docker, ni de Make, ni des toolchains Go/Node. Toutes les commandes de compilation, de test (`make test`, `make test-front`, `make lint`) et de rapatriement de données (`make ssh-debug-pull`) sont exécutées par le développeur sur son poste local ou par la CI GitHub Actions.

---

## 3. Diagnostic & Débogage de Production (Local-First)

Pour diagnostiquer un dysfonctionnement survenu en production (erreur de file d'attente Radarr/Sonarr, désynchronisation de scrobble, problème de matching, incohérence de BDD) :

### Règle Local-First
**Systématiquement rapatrier les fichiers (BDD, logs) en local d'abord.** L'inspection des logs, les requêtes BDD et le débogage applicatif se font en local. L'utilisation directe de SSH reste réservée aux diagnostics système qui ne peuvent pas être extraits sous forme de fichiers (espace disque hôte, connectivité réseau, statut du démon Docker).

### Workflow
1. **Extraction de l'état de production** :
   ```bash
   make ssh-debug-pull
   ```
   *Télécharge la BDD de production (`data/plextracker.db` + WAL/SHM) et les logs du conteneur (`data/plextracker.log`), puis démarre l'application locale.*
2. **Analyse des logs en local** :
   Inspection et recherche textuelle / regex directement dans `data/plextracker.log`.
3. **Analyse de la BDD en local** :
   Requêtes SQL ou inspection via l'interface locale (`http://localhost:8080`) connectée aux données de prod.
4. **Reproduction & Fixation** :
   Reproduction du bug en local, rédaction de tests unitaires/intégration (`make test`), et validation du fix.
