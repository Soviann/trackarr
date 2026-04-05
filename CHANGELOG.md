# Changelog

## Non publié

### Ajouté
- Import Simkl : enrichissement automatique des titres importés via la file d'attente existante
- Import Simkl : backfill des épisodes précédents (marque vus les épisodes antérieurs au plus avancé)

## [v0.5.0] — 2026-04-05

### Ajouté
- File d'attente avec retry automatique pour les tâches async (enrichissement, refresh, couvertures) — les erreurs de rate limit ou réseau ne perdent plus la tâche
- Retry en 2 niveaux : 5 tentatives jour même avec backoff exponentiel, report au lendemain si échec, puis tâche morte
- Section Admin dans la navbar avec page hub (Validations, Tâches, Notifications)
- Page de gestion des tâches échouées (relancer / supprimer)
- Préférences de notifications push activables/désactivables par type (rappel de notation, tâche échouée)
- Notification push quand une tâche meurt définitivement
- Tiroir de filtres unifié sur les pages Library et Search
- Auto-complétion des séries terminées au dernier épisode

### Corrigé
- Couleur du filtre "All" actif dans le tiroir
- Espace entre l'ActionBar et la Navbar sur la page de détail

### Amélioré
- Gestion des séries/anime à la réception des webhooks Plex
- Comportement des filtres de statut et de type

## [v0.4.0] — 2026-04-05

### Ajouté
- Backfill automatique : marquer un épisode comme vu marque aussi tous les épisodes précédents de la même saison

## [v0.3.2] — 2026-04-05

### Corrigé
- Base de données en lecture seule en prod — le conteneur démarre maintenant en root, corrige les permissions du volume `/data`, puis bascule sur l'utilisateur applicatif via `gosu`

## [v0.3.1] — 2026-04-05

### Corrigé
- Webhooks Plex systématiquement rejetés (erreur 400) — remplacement du parsing multipart manuel par la bibliothèque `plexwebhooks`, avec fallback JSON si le reverse proxy altère le Content-Type

## [v0.3.0] — 2026-04-04

### Ajouté
- Pagination serveur avec réponse allégée (compteurs saison, next_episode)
- Placeholders colorés par type pour les titres sans couverture
- Métadonnées épisodes enrichies depuis TMDB (nom, date de diffusion)
- États d'erreur avec bouton « Réessayer » et écran de secours JS
- Fondations frontend : store Zustand, utilitaires partagés, tokens de thème
- Recherche améliorée : tri par pertinence, limite à 50 résultats

### Refactorisé
- Découpage des clients TMDB et AniList par responsabilité (search, details, covers, sync)
- Interface `PushNotifier` avec implémentation no-op (suppression des nil checks)
- Interface `DBTX` et helper `WithTx` pour transactions SQLite
- Scrobbles Plex atomiques via transactions
- `BatchCreate` pour les watch events
- Gestion d'erreurs centralisée avec helpers HTTP
- Rate limiter `x/time/rate` pour les appels API externes

### Corrigé
- Crash JS causé par `episodes: null` dans le JSON
- Cohérence de langue sur l'écran de connexion
- Accessibilité des boutons retour/edit (balises `<button>` + `aria-label`)

## [v0.2.0] — 2026-04-04

### Ajouté
- Page Stats avec tableau de bord complet

### Corrigé
- Sécurité : CSP, open redirect, referrer leaks, durée JWT, rate limiting

## [v0.1.3] — 2026-04-04

### Corrigé
- Déploiement : préserve les fichiers locaux (`logs/`, `.env.local`) lors de la mise à jour
- CI : ne tourne plus que lors des releases

## [v0.1.2] — 2026-04-04

### Corrigé
- Déploiement : évite la suppression de `.env.local` lors de la mise à jour

### Ajouté
- Déclenchement manuel du déploiement (sans tag)

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
