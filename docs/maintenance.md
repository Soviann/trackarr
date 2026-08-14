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
2. Il poste automatiquement un commentaire sur la PR pour alerter l'agent AI Antigravity.
3. L'agent AI prend en charge l'analyse des logs, applique la correction dans le code et pousse la résolution sur la branche de la PR.
