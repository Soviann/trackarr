# Remplacement des libs infrastructure — Plan d'implémentation

> **For agentic workers:** REQUIRED SUB-SKILL: `superpowers:subagent-driven-development` pour orchestrer les sous-agents. Suivre la section **Kickoff procedure** ci-dessous à l'identique quand le user dit « lance le plan ».

**Goal:** Remplacer le code maison d'infrastructure (headers sécurité, parsing des variables d'environnement, gestion d'erreurs API) par des librairies Go stables, sans modifier le comportement observable du backend.

**Architecture:** 3 tâches indépendantes réparties en 2 vagues. La Vague 1 contient 2 tâches totalement découplées, exécutées en parallèle dans 2 git worktrees par 2 sous-agents Sonnet. La Vague 2 contient 1 tâche sensible exécutée après merge de la Vague 1, dans un worktree dédié, toujours sur Sonnet.

**Tech Stack:** Go 1.24, `github.com/unrolled/secure`, `github.com/caarlos0/env/v11`, `github.com/samber/oops`, chi v5, testify.

**Base branch:** `main` (pas la branche courante si elle n'est pas `main`).

**Feature branch à créer:** `refactor/infra-libs`

---

## Kickoff procedure (ce que Claude fait quand le user dit « lance le plan »)

Étapes strictes, dans l'ordre. Ne pas improviser.

1. **Vérifier l'état git**
   - `git status` : working tree doit être clean. Sinon demander au user s'il veut stash/commit.
   - `git branch --show-current` : noter la branche courante.
   - `git fetch origin main` puis `git log --oneline origin/main -5` pour voir l'état de main.

2. **Créer la branche d'intégration**
   - Depuis `main` à jour : `git checkout main && git pull --ff-only origin main`
   - `git checkout -b refactor/infra-libs`
   - Ne pas push cette branche tant que les vagues ne sont pas intégrées.

3. **Créer 2 worktrees parallèles pour la Vague 1**
   - Worktree A pour Task 1 (`unrolled/secure`) : branche `refactor/infra-libs-secure`
   - Worktree B pour Task 2 (`caarlos0/env`) : branche `refactor/infra-libs-env`
   - Les deux partent de `refactor/infra-libs`.

4. **Dispatcher 2 sous-agents Sonnet en parallèle**
   - Utiliser `Agent` avec `subagent_type: general-purpose`, `model: sonnet`, `isolation: worktree`, `run_in_background: true`.
   - **Envoyer les 2 appels dans UN SEUL message** (parallélisme réel).
   - Prompt de chaque agent : le contenu complet de la Task correspondante + le bloc **Dispatch rules for subagents** ci-dessous, copié verbatim.
   - Pas de dispatch de Task 3 tant que Vague 1 n'est pas intégrée.

5. **Attendre la fin des 2 sous-agents** (notifications automatiques). Ne pas poller.

6. **Intégrer la Vague 1 dans `refactor/infra-libs`**
   - Checkout `refactor/infra-libs`
   - Merger T1 puis T2 (ordre indifférent, pas de conflits attendus car fichiers disjoints)
   - Après chaque merge : `make build && make test && make lint`
   - Si conflit ou échec : résoudre manuellement, ne pas re-dispatch.

7. **Nettoyer les worktrees Vague 1**
   - `git worktree remove <path>` pour A et B
   - `git branch -d refactor/infra-libs-secure refactor/infra-libs-env`

8. **Confirmation utilisateur avant Vague 2**
   - Annoncer au user : « Vague 1 intégrée et tests verts. Je dispatche la Vague 2 (samber/oops sur APIError) ou je m'arrête ici ? »
   - Attendre réponse explicite. La Vague 2 est optionnelle.

9. **Si feu vert Vague 2**
   - Créer worktree C : branche `refactor/infra-libs-oops` depuis `refactor/infra-libs`
   - Dispatcher 1 sous-agent Sonnet (même règles) sur Task 3
   - Attendre, merger, tester, nettoyer comme pour Vague 1.

10. **Finalisation**
    - Mettre à jour `CHANGELOG.md` sous `## [Unreleased]` → `### Changé` avec une entrée par tâche intégrée.
    - Mettre à jour `docs/patterns.md` si un pattern standard a changé (uniquement si pertinent — demander avant).
    - Committer CHANGELOG/patterns si modifiés.
    - Déplacer ce plan vers `docs/superpowers/plans/done/remplacement-libs-infrastructure.md`.
    - Ne pas push, ne pas créer de PR sans demander au user.

---

## Dispatch rules for subagents (à copier verbatim dans chaque prompt d'agent)

```
RÈGLES IMPÉRATIVES :

1. Tu travailles dans un git worktree isolé. Ta branche est déjà basée sur refactor/infra-libs. NE MERGE PAS main ni refactor/infra-libs dans ta branche. NE REBASE PAS.

2. Lis le vrai code avant de faire confiance aux snippets du plan. Les plans peuvent diverger de la réalité. Pour chaque fichier cité, fais un Read complet avant toute modification.

3. Toutes les commandes Go/test/lint/build passent par le Makefile (dans Docker). Jamais `go` directement sur l'host.
   - Tests : `make test`
   - Lint : `make lint`
   - Build : `make build`
   - Ajouter une dépendance : `make shell` puis `go get <module>` dans le conteneur, OU éditer go.mod + `make shell` + `go mod tidy`.

4. Conserve strictement les signatures exportées existantes (noms de fonctions, types, paramètres). Ton travail ne doit rien casser côté callers.

5. Après chaque modification significative : `make build && make test`. Si rouge, fix avant de continuer.

6. Ne touche PAS à d'autres fichiers que ceux listés dans ta tâche. Si tu penses qu'un autre fichier doit changer, arrête-toi et signale-le dans ton rapport final plutôt que d'élargir le scope.

7. Commits : français, 3e personne impératif, format `refactor(scope): description`. Un seul commit final par tâche (squash autorisé). Trailer obligatoire : `Co-Built-By: Claude (<quip varié>)`.

8. Rapport final attendu : liste des fichiers modifiés, résultat de `make build`, `make test`, `make lint`, et SHA du commit final.

9. Ne push jamais. Ne crée jamais de PR. Ne touche pas à CHANGELOG.md ni docs/patterns.md — l'orchestrateur s'en charge.

10. Attention au conteneur dev `plextracker-dev` : ton worktree utilise peut-être le même `container_name` via docker-compose.dev.yml. Si tu as besoin de lancer l'app, utilise `make test` qui est auto-contenu, ou renomme le conteneur localement ET REVERT avant ton commit final.
```

---

## File inventory

**Vague 1 — Task 1 (`unrolled/secure`)**
- Modify: `internal/middleware/security.go`
- Modify: `go.mod`, `go.sum`
- Le caller `internal/router/router.go:28` utilise `mw.SecurityHeaders` — la signature doit rester identique, donc router.go ne change pas.
- Test à ajouter : `internal/middleware/security_test.go` (n'existe probablement pas)

**Vague 1 — Task 2 (`caarlos0/env`)**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `go.mod`, `go.sum`
- Le caller `cmd/serve.go:27` utilise `config.Load()` — la signature `Load() (*Config, error)` doit rester identique.

**Vague 2 — Task 3 (`samber/oops`) — OPTIONNELLE**
- Modify: `internal/service/matching/apierror.go`
- Vérifier : `internal/service/matching/*.go` pour les call sites de `newAPIError`, `IsRetryableError`, `IsRateLimitError`, `ExtractRetryAfter`
- Vérifier : `internal/handler/client_errors.go` (si référence APIError)
- Modify: `go.mod`, `go.sum`

---

## Task 1 — Remplacement des headers sécurité par `unrolled/secure`

### Contexte
`internal/middleware/security.go` définit manuellement 6 headers (HSTS, X-Content-Type-Options, X-Frame-Options, Referrer-Policy, Permissions-Policy, CSP). La lib `github.com/unrolled/secure` fournit ces options de façon maintenue, avec support natif des directives CSP structurées. Gain : maintenabilité, suivi OWASP, moins de code à relire.

### Acceptance criteria
- La fonction exportée `SecurityHeaders(next http.Handler) http.Handler` existe toujours avec la même signature.
- Tous les headers actuels sont toujours émis sur toutes les routes, avec les mêmes valeurs exactes (en particulier la CSP complète incluant `accounts.google.com`, `lh3.googleusercontent.com`, `fonts.gstatic.com`, `fonts.googleapis.com`).
- Un test unitaire `TestSecurityHeaders` vérifie la présence et la valeur de chacun des 6 headers via un handler bidon enveloppé par le middleware.
- `make build && make test && make lint` passent.
- Aucune modification de `internal/router/router.go`.

### Steps

- [ ] **1.1** Lire `internal/middleware/security.go` et noter les 6 headers + la CSP complète (6 directives).
- [ ] **1.2** Lire `internal/router/router.go` autour de la ligne 28 pour confirmer que le middleware est utilisé via `r.Use(mw.SecurityHeaders)`.
- [ ] **1.3** Baseline verte : `make test` doit passer avant toute modification.
- [ ] **1.4** Ajouter la dépendance : via `make shell`, exécuter `go get github.com/unrolled/secure@latest` puis `go mod tidy`.
- [ ] **1.5** Écrire d'abord le test (TDD) dans `internal/middleware/security_test.go` :
    - Créer un handler bidon qui renvoie 200 OK.
    - L'envelopper avec `SecurityHeaders`.
    - Faire une requête via `httptest.NewRecorder()`.
    - Asserter la présence + valeur exacte de chacun des 6 headers.
    - Asserter que la CSP contient chacune des 8 sources whitelistées actuellement (script-src, style-src, font-src, img-src, connect-src, frame-src, worker-src, manifest-src).
- [ ] **1.6** Lancer `make test ./internal/middleware/...` — le test doit passer contre l'implémentation actuelle (c'est un test de caractérisation).
- [ ] **1.7** Réécrire `internal/middleware/security.go` :
    - Instancier `secure.New(secure.Options{...})` au package level (var globale) avec toutes les options correspondantes : `STSSeconds`, `STSIncludeSubdomains`, `ContentTypeNosniff`, `FrameDeny`, `ReferrerPolicy`, `PermissionsPolicy`, `ContentSecurityPolicy` (string unique, identique à l'actuelle).
    - `SecurityHeaders` devient un wrapper qui délègue à `secureMiddleware.Handler(next)`.
    - Signature exportée **inchangée** : `func SecurityHeaders(next http.Handler) http.Handler`.
- [ ] **1.8** Relancer `make test ./internal/middleware/...` — le test doit rester vert.
- [ ] **1.9** Lancer `make test` (suite complète), `make lint`, `make build`. Tous verts.
- [ ] **1.10** Commit unique :
    ```
    refactor(security): remplace les headers maison par unrolled/secure

    Co-Built-By: Claude (<quip>)
    ```
- [ ] **1.11** Rapport final à l'orchestrateur : fichiers modifiés, SHA, résultats des 3 make.

---

## Task 2 — Remplacement du parsing env par `caarlos0/env`

### Contexte
`internal/config/config.go` fait 78 lignes de `os.Getenv` + helper `envOr` + validation manuelle. `github.com/caarlos0/env/v11` utilise des struct tags pour parser, typer et valider les variables d'environnement. Gain : ~40 LOC de moins, type safety native, moins de boilerplate quand une nouvelle clé est ajoutée.

### Subtilités à préserver

1. `GEMINI_API_KEY` contient une liste **séparée par virgules** → champ `[]string`. `caarlos0/env` gère ça via `envSeparator:","`.
2. `CookieSecure` a une logique dérivée : si `COOKIE_SECURE` n'est pas défini, il vaut `!DebugLogin`. Cette logique **doit rester après le parsing env** (post-process manuel), car `caarlos0/env` ne supporte pas les defaults dépendant d'autres champs.
3. Trois vars sont obligatoires : `GOOGLE_CLIENT_ID`, `GOOGLE_ALLOWED_EMAIL`, `JWT_SECRET`. Utiliser le tag `required:"true"` ou garder la validation manuelle après parsing.
4. `JWT_SECRET == "dev-secret-change-me"` + `!DebugLogin` → erreur. Validation métier : à garder en post-process.
5. `ListenAddr` default `:8080`, `DataDir` default `/data` : utiliser le tag `envDefault`.
6. `DEBUG_LOGIN`, `DISABLE_BACKGROUND_TASKS` : actuellement comparés à `"true"`. `caarlos0/env` parse `bool` nativement.

### Acceptance criteria
- La fonction exportée `Load() (*Config, error)` existe toujours avec la même signature.
- Le struct `Config` garde les mêmes champs exportés avec les mêmes types (sauf ajout de tags).
- Toutes les subtilités ci-dessus sont préservées (comportement identique).
- `internal/config/config_test.go` passe sans modification structurelle (ajuster uniquement si des détails de validation ont bougé, mais les scenarios doivent rester les mêmes).
- `make build && make test && make lint` passent.
- Aucune modification de `cmd/serve.go`.

### Steps

- [ ] **2.1** Lire `internal/config/config.go` intégralement. Lister tous les champs avec leur clé env et leur default.
- [ ] **2.2** Lire `internal/config/config_test.go` pour comprendre les scenarios (var requises manquantes, `dev-secret-change-me`, etc.).
- [ ] **2.3** Lire `cmd/serve.go:27` pour confirmer que seul `config.Load()` est appelé.
- [ ] **2.4** Baseline verte : `make test ./internal/config/...`.
- [ ] **2.5** Ajouter la dépendance : via `make shell`, `go get github.com/caarlos0/env/v11@latest` puis `go mod tidy`.
- [ ] **2.6** Réécrire `internal/config/config.go` :
    - Annoter chaque champ du struct `Config` avec des tags `env:"NOM_VAR"`, `envDefault:"..."`, `envSeparator:","` pour `GeminiAPIKeys`, `required:"true"` pour `GoogleClientID`/`GoogleAllowedEmail`/`JWTSecret`.
    - Dans `Load()` : appeler `env.Parse(&cfg)` à la place du bloc manuel.
    - Garder APRÈS le `env.Parse` : la logique `CookieSecure` dépendant de `DebugLogin`, et la validation `JWT_SECRET == "dev-secret-change-me"`.
    - Supprimer le helper `envOr` (devenu inutile).
- [ ] **2.7** `make test ./internal/config/...` — tous les tests doivent rester verts. Si un test casse, ne modifier que ce qui concerne le changement d'erreur (ex: message d'erreur différent pour var requise manquante — dans ce cas, adapter le test pour asserter sur le type d'erreur, pas la string exacte).
- [ ] **2.8** `make test` (suite complète), `make lint`, `make build`. Tous verts.
- [ ] **2.9** Commit unique :
    ```
    refactor(config): remplace le parsing env maison par caarlos0/env

    Co-Built-By: Claude (<quip>)
    ```
- [ ] **2.10** Rapport final : fichiers modifiés, SHA, résultats des 3 make, note si le test config a dû être ajusté et pourquoi.

---

## Task 3 — (VAGUE 2, OPTIONNELLE) Remplacement de `APIError` par `samber/oops`

### Contexte
`internal/service/matching/apierror.go` définit un type `APIError` custom avec `Service`, `StatusCode`, `RetryAfter`, plus 3 helpers : `IsRetryableError`, `IsRateLimitError`, `ExtractRetryAfter`. Tout ça fonctionne via `errors.As` et inspection d'attributs. `github.com/samber/oops` offre un builder d'erreurs structuré avec attributs, codes, contexte, stack traces.

**Attention — cette tâche est plus risquée** : `APIError` est probablement utilisé dans plusieurs fichiers de `internal/service/matching/` et potentiellement dans les handlers. Un changement mal fait peut masquer des erreurs de retry dans la taskqueue. Ne lancer qu'après avoir validé que la Vague 1 est stable.

### Acceptance criteria
- Le comportement de retry reste **strictement identique** : mêmes conditions déclenchent `IsRetryableError == true`, même valeur remontée par `ExtractRetryAfter`.
- Les fonctions exportées `IsRetryableError`, `IsRateLimitError`, `ExtractRetryAfter` gardent leur signature (`func(error) bool`/`func(error) time.Duration`).
- Tous les call sites existants de `newAPIError`, `IsRetryableError`, `IsRateLimitError`, `ExtractRetryAfter` continuent de compiler sans modification (ou avec modifications triviales).
- La logique de parsing du header `Retry-After` reste en place (que ce soit dans un helper ou via un attribut oops).
- `make build && make test && make lint` passent. En particulier, les tests du matching et de la taskqueue doivent rester verts.

### Steps

- [ ] **3.1** Lire `internal/service/matching/apierror.go` intégralement.
- [ ] **3.2** Grep les call sites : `newAPIError`, `APIError`, `IsRetryableError`, `IsRateLimitError`, `ExtractRetryAfter` dans tout le repo. Lister les fichiers touchés.
- [ ] **3.3** Lire `internal/handler/client_errors.go` (si existe) pour voir s'il référence `APIError`.
- [ ] **3.4** Lire les tests existants : `internal/service/matching/*_test.go` et `internal/service/taskqueue*_test.go` ou `internal/service/background_test.go` pour identifier ce qui teste la logique de retry.
- [ ] **3.5** Baseline verte : `make test`.
- [ ] **3.6** Ajouter la dépendance : via `make shell`, `go get github.com/samber/oops@latest` puis `go mod tidy`.
- [ ] **3.7** Réécrire `internal/service/matching/apierror.go` :
    - `newAPIError` retourne désormais une erreur construite via `oops.In(service).Code(statusCode).With("retry-after", retryAfter).Errorf(...)`.
    - `IsRetryableError` inspecte l'erreur : via `oops.AsOops(err)` récupérer le code, matcher 429/5xx, sinon fallback sur l'ancienne logique (net.Error, string patterns, "rate-limited").
    - `IsRateLimitError` : même approche, check code == 429.
    - `ExtractRetryAfter` : extraire l'attribut "retry-after" via l'API oops (ou garder un type interne wrappé si plus simple).
    - **Alternative acceptable si samber/oops ne matche pas bien** : garder `APIError` comme struct, mais utiliser `oops` en complément pour la stack trace. Dans ce cas documenter le choix dans le commit body. L'important est d'évaluer honnêtement et de ne pas dégrader la clarté.
- [ ] **3.8** Si `APIError` est supprimé et que des callers faisaient `var e *APIError; errors.As(err, &e)` : adapter ces call sites pour utiliser les helpers publics (`IsRetryableError`, etc.) au lieu d'inspecter le type concret. Lister ces modifications dans le rapport final.
- [ ] **3.9** `make test` — en particulier les tests de matching et de taskqueue doivent rester verts. Si un test de retry casse, ne pas le modifier : debugger le code.
- [ ] **3.10** `make lint && make build`. Verts.
- [ ] **3.11** Commit unique :
    ```
    refactor(matching): remplace APIError par samber/oops

    Co-Built-By: Claude (<quip>)
    ```
- [ ] **3.12** Rapport final : fichiers modifiés, SHA, résultats des 3 make, liste des call sites adaptés, choix de design (suppression totale d'APIError ou wrap partiel) et justification.

---

## Points d'attention pour l'orchestrateur

- **Ordre de merge Vague 1** : T1 puis T2 (ou l'inverse). Zéro conflit attendu car `internal/middleware/security.go` et `internal/config/config.go` sont totalement disjoints. Seul point de contact possible : `go.mod` / `go.sum`. Résoudre manuellement en acceptant les deux ajouts de dépendance.
- **Si un sous-agent Vague 1 échoue** : ne pas re-dispatcher (sandbox sibling worktree bloque Bash après ~2 tool calls). `cd` dans le worktree depuis la session principale et finir manuellement.
- **Validation post-intégration Vague 1** : lancer l'app via `make up` et vérifier côté navigateur que (a) le login fonctionne, (b) les headers de sécurité sont présents sur une requête (DevTools → Network → Response Headers). Pas besoin de visual check complet puisque zéro changement UI.
- **Changelog** : une entrée sous `## [Unreleased]` → `### Changé` par tâche intégrée, ex :
  - `Remplacement des headers de sécurité maison par la librairie unrolled/secure (aucun changement visible).`
  - `Remplacement du parsing des variables d'environnement par caarlos0/env (aucun changement visible).`
- **Vague 2 — stop-loss** : si Task 3 introduit le moindre doute sur la logique de retry de la taskqueue, annuler et restaurer `APIError`. Le gain de lisibilité ne vaut pas un bug silencieux sur les retries.

---

## Hors scope — à traiter dans un plan séparé

**Remplacement des clients HTTP TMDB / TVDB / AniList par des SDKs communautaires.**

Identifié lors de l'audit comme le plus gros gain potentiel (~300 LOC de clients HTTP maison dans `internal/service/matching/tmdb.go`, `tvdb.go`, `anilist.go`), mais **volontairement exclu de ce plan** parce qu'il nécessite une phase de recherche préalable qui n'a pas sa place dans un plan d'exécution mécanique.

Ce qu'il faudra faire avant d'écrire le plan correspondant :
1. **Évaluer les SDKs candidats** un par un (`cyruzin/golang-tmdb`, SDKs TVDB Go existants, SDKs AniList GraphQL Go). Pour chacun : dernière release, activité du repo, couverture des endpoints utilisés par PlexTracker, qualité de la gestion d'erreurs, compatibilité avec le rate limiting existant, support du context.
2. **Lister les endpoints actuellement consommés** par PlexTracker dans `internal/service/matching/` et vérifier que chaque SDK les couvre.
3. **Décider SDK par SDK** : remplacer, garder le client maison, ou remplacer partiellement (ex: auth + requêtes de base via SDK, endpoints exotiques via code maison).
4. **Identifier les impacts transverses** : rate limiter par service, rotation de clés Gemini, retry via `IsRetryableError`, parsing des réponses dans les structs domaine.

Résultat attendu : une note de décision (pas un plan) listant pour chaque service « remplacer / garder / hybride » avec justification. Si au moins un service est remplaçable sans perte, écrire alors un plan d'implémentation dédié.

Déclencheur : dire « fais la recherche SDKs matching » dans une prochaine session.
