# Changelog

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
