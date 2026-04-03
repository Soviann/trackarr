# PlexTracker Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a self-hosted media tracking app (Go + Preact + SQLite) that auto-tracks Plex watch progress, with manual entry and external links/sync to IMDB and AniList.

**Architecture:** Single Go binary with embedded SQLite, serving a Preact SPA. All external API calls (TMDB, AniList, Gemini) go through the Go backend. Deployed as one Docker container on Synology NAS. Local dev uses Docker Compose via limactl.

**Tech Stack:** Go 1.24, chi router, mattn/go-sqlite3, golang-migrate, Preact 10, Vite, preact-router, Web Push API (VAPID), Google OAuth + JWT.

**Specs:** `docs/superpowers/specs/2026-04-01-plextracker-design.md` (main), `docs/superpowers/specs/2026-04-02-plextracker-ui-design.md` (UI/UX). Approved mockups in `docs/mockups/*-v2.html` (and v3/v6 for some).

---

## Conventions

- **All commands run inside the Docker container** via `docker compose exec app <cmd>`. Never run `go`, `node`, `npm` on host. Only `git`, `gh`, `docker`, `docker compose` on host.
- **Makefile** wraps all common operations (`make test`, `make lint`, `make build`, etc.) — targets call `docker compose exec app ...` internally.
- **Tests**: Go tests with `testing` + `testify/assert`. Frontend tests with Vitest + Preact Testing Library.
- **Commits**: French conventional commits (`feat(scope): ajoute ...`, `fix(scope): corrige ...`).
- **Update `docs/patterns.md`** after each task that adds new files, routes, or patterns.
- **Update `docs/user-guide.md`** when adding user-facing features (new screens, new workflows, config changes).
- **Prefer Makefile targets** over raw `docker compose exec` commands. Add new targets as needed.

## File Structure

```
plextracker/
├── Makefile                         # All dev commands (wraps docker compose exec)
├── Dockerfile                       # Multi-stage: build Go + Preact, produce single binary
├── Dockerfile.dev                   # Dev image with hot reload (air + vite)
├── docker-compose.yml               # Production
├── docker-compose.dev.yml           # Local dev (hot reload, volume mounts)
├── .air.toml                        # Go hot reload config
├── go.mod / go.sum
├── main.go                          # Entrypoint: CLI (serve / import)
├── cmd/
│   ├── serve.go                     # HTTP server startup
│   └── import.go                    # Simkl import CLI command
├── internal/
│   ├── config/
│   │   └── config.go                # Env var parsing (GOOGLE_CLIENT_ID, TMDB_API_KEY, etc.)
│   ├── database/
│   │   ├── database.go              # SQLite connection, pragmas, WAL mode
│   │   └── migrations/              # SQL migration files (001_init.up.sql, etc.)
│   ├── model/
│   │   ├── title.go                 # Title, TitleName structs + enums (TitleType, TitleStatus, etc.)
│   │   ├── season.go                # Season struct
│   │   ├── episode.go               # Episode struct
│   │   ├── watch_event.go           # WatchEvent struct
│   │   └── setting.go               # Setting struct (key/value)
│   ├── repository/
│   │   ├── title.go                 # TitleRepository (CRUD, filters, search)
│   │   ├── season.go                # SeasonRepository
│   │   ├── episode.go               # EpisodeRepository
│   │   ├── watch_event.go           # WatchEventRepository
│   │   └── setting.go               # SettingRepository
│   ├── handler/
│   │   ├── auth.go                  # Google OAuth callback, logout
│   │   ├── title.go                 # Title CRUD endpoints
│   │   ├── episode.go               # Episode toggle/batch endpoints
│   │   ├── season.go                # Season rating endpoint
│   │   ├── review.go                # Match review endpoints
│   │   ├── webhook.go               # Plex webhook handler
│   │   ├── cover.go                 # Cover image serving
│   │   ├── anilist_auth.go          # AniList OAuth flow
│   │   ├── push.go                  # Push subscription endpoints
│   │   ├── settings.go              # Settings endpoint
│   │   └── spa.go                   # SPA catch-all (serves index.html for client routes)
│   ├── middleware/
│   │   └── auth.go                  # JWT cookie validation middleware
│   ├── service/
│   │   ├── matching/
│   │   │   ├── pipeline.go          # Media matching pipeline orchestrator (Steps 1-5)
│   │   │   ├── crossref.go          # anime-offline-database cross-reference (Step 2)
│   │   │   ├── tmdb.go              # TMDB API client (Step 3 + episode fetch + covers)
│   │   │   ├── anilist.go           # AniList GraphQL client (Step 4 + episode fetch + sync)
│   │   │   └── gemini.go            # Gemini AI verification/fallback (Step 5)
│   │   ├── plex.go                  # Plex webhook payload parsing + processing
│   │   ├── push.go                  # Web Push notification sending
│   │   ├── background.go            # Daily title refresh job
│   │   └── simkl.go                 # Simkl backup import logic
│   └── router/
│       └── router.go                # chi router setup, all route registration
├── frontend/
│   ├── package.json
│   ├── vite.config.ts
│   ├── tsconfig.json
│   ├── index.html                   # SPA shell
│   ├── public/
│   │   ├── manifest.json            # PWA manifest with share_target
│   │   └── sw.js                    # Service worker (push notifications)
│   └── src/
│       ├── main.tsx                 # Preact app entry
│       ├── app.tsx                  # Router setup (preact-router)
│       ├── api.ts                   # apiFetch wrapper (JWT cookie, error handling)
│       ├── types.ts                 # TypeScript types matching Go models
│       ├── theme.ts                 # Design tokens (colors from UI spec)
│       ├── components/
│       │   ├── Navbar.tsx           # Bottom navbar (4 tabs with identity colors)
│       │   ├── FilterBar.tsx        # Library filter bar (secondary bar)
│       │   ├── ActionBar.tsx        # Title detail action bar (secondary bar)
│       │   ├── BottomSheet.tsx      # Reusable bottom sheet (drag handle, backdrop)
│       │   ├── RatingPrompt.tsx     # Rating bottom sheet (10 stars + actions)
│       │   ├── TitleCard.tsx        # Horizontal card (watching/up-to-date)
│       │   ├── PosterCard.tsx       # Poster grid card (completed/dropped/plan)
│       │   ├── EpisodeRow.tsx       # Episode list row with checkmark
│       │   ├── SeasonTab.tsx        # Season pill (completed/in-progress/unwatched)
│       │   ├── MatchReviewCard.tsx  # Match review card (unconfirmed/pending)
│       │   ├── EditSheet.tsx        # Edit title bottom sheet
│       │   ├── AniListSheet.tsx     # AniList match bottom sheet
│       │   └── StatusBadge.tsx      # Colored status pill
│       ├── pages/
│       │   ├── Library.tsx          # Library (home) screen
│       │   ├── TitleDetail.tsx      # Title detail screen
│       │   ├── Search.tsx           # Global search screen
│       │   ├── Add.tsx              # Add title screen (input)
│       │   ├── Validate.tsx         # Title validation screen (shared: add/share/fix)
│       │   ├── MatchReview.tsx      # Match review screen
│       │   ├── Login.tsx            # Google OAuth login screen
│       │   └── Stats.tsx            # Stats placeholder
│       └── hooks/
│           ├── useApi.ts            # Data fetching hook (SWR-like with apiFetch)
│           └── usePush.ts           # Push notification subscription hook
└── data/                            # Runtime data (gitignored)
    ├── plextracker.db               # SQLite database
    └── covers/                      # Downloaded cover images
```

---

## Phase 1: Project Scaffold & Local Dev

### Task 1.1: Docker Dev Environment

**Files:**
- Create: `Dockerfile.dev`
- Create: `docker-compose.dev.yml`
- Create: `Makefile`
- Create: `.gitignore`
- Create: `.air.toml`

- [ ] **Step 1: Create `.gitignore`**

```gitignore
# Runtime data
data/
*.db

# Go
/plextracker

# Frontend
frontend/node_modules/
frontend/dist/

# IDE
.idea/
.vscode/
*.swp

# OS
.DS_Store

# Plans (never commit)
docs/plans/
docs/superpowers/plans/

# Dev
tmp/
```

- [ ] **Step 2: Create `Dockerfile.dev`**

Multi-tool dev image: Go 1.24 + Node 22 LTS + air (Go hot reload).

```dockerfile
FROM golang:1.24-bookworm

# Node.js 22 LTS
RUN curl -fsSL https://deb.nodesource.com/setup_22.x | bash - \
    && apt-get install -y nodejs \
    && rm -rf /var/lib/apt/lists/*

# Air for Go hot reload
RUN go install github.com/air-verse/air@latest

WORKDIR /app

# Go dependencies (cached layer)
COPY go.mod go.sum ./
RUN go mod download

# Frontend dependencies (cached layer)
COPY frontend/package.json frontend/package-lock.json ./frontend/
RUN cd frontend && npm ci

COPY . .

EXPOSE 8080 5173

CMD ["air", "-c", ".air.toml"]
```

- [ ] **Step 3: Create `docker-compose.dev.yml`**

```yaml
services:
  app:
    build:
      context: .
      dockerfile: Dockerfile.dev
    container_name: plextracker-dev
    ports:
      - "8080:8080"   # Go backend
      - "5173:5173"   # Vite dev server
    volumes:
      - .:/app
      - go-mod-cache:/go/pkg/mod
      - go-build-cache:/root/.cache/go-build
      - node-modules:/app/frontend/node_modules
    environment:
      - GOOGLE_CLIENT_ID=dev
      - GOOGLE_ALLOWED_EMAIL=dev@localhost
      - JWT_SECRET=dev-secret-change-me
      - TMDB_API_KEY=
      - ANILIST_CLIENT_ID=
      - ANILIST_CLIENT_SECRET=
      - GEMINI_API_KEY=
      - VAPID_PUBLIC_KEY=
      - VAPID_PRIVATE_KEY=
      - VAPID_SUBJECT=mailto:dev@localhost
      - DATA_DIR=/app/data
    working_dir: /app

volumes:
  go-mod-cache:
  go-build-cache:
  node-modules:
```

- [ ] **Step 4: Create `.air.toml`**

```toml
root = "."
tmp_dir = "tmp"

[build]
  cmd = "go build -o ./tmp/plextracker ."
  bin = "./tmp/plextracker serve"
  include_ext = ["go", "tpl", "tmpl", "html", "sql"]
  exclude_dir = ["tmp", "frontend", "node_modules", "data", "docs"]
  delay = 1000

[misc]
  clean_on_exit = true
```

- [ ] **Step 5: Create `Makefile`**

```makefile
DC = docker compose -f docker-compose.dev.yml
EXEC = $(DC) exec app

.PHONY: help up down build test lint fmt dev-frontend

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

up: ## Start dev environment
	$(DC) up -d --build

down: ## Stop dev environment
	$(DC) down

logs: ## Tail logs
	$(DC) logs -f

shell: ## Shell into container
	$(EXEC) bash

test: ## Run all Go tests
	$(EXEC) go test ./... -v -count=1

test-front: ## Run frontend tests
	$(EXEC) bash -c "cd frontend && npx vitest run"

lint: ## Run Go linter
	$(EXEC) golangci-lint run ./...

fmt: ## Format Go code
	$(EXEC) gofmt -w .

dev-frontend: ## Start Vite dev server (inside container)
	$(EXEC) bash -c "cd frontend && npx vite --host 0.0.0.0"

build: ## Build production binary
	$(EXEC) go build -o plextracker .

migrate: ## Run database migrations
	$(EXEC) ./tmp/plextracker migrate

import: ## Import Simkl backup (BACKUP_FILE=path)
	$(EXEC) ./tmp/plextracker import $(BACKUP_FILE)

import-dry: ## Dry-run Simkl import
	$(EXEC) ./tmp/plextracker import --dry-run $(BACKUP_FILE)
```

- [ ] **Step 6: Initialize Go module**

Create minimal `go.mod` and `main.go` so the container can build.

`go.mod`:
```
module github.com/nicolasvasse/plextracker

go 1.24
```

`main.go`:
```go
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: plextracker <serve|import|migrate>")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		fmt.Println("PlexTracker starting...")
	case "import":
		fmt.Println("Import not yet implemented")
	case "migrate":
		fmt.Println("Migrate not yet implemented")
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}
```

- [ ] **Step 7: Start containers and verify**

```bash
make up
docker compose -f docker-compose.dev.yml exec app go version
# Expected: go version go1.24.x linux/arm64
docker compose -f docker-compose.dev.yml exec app node --version
# Expected: v22.x.x
```

- [ ] **Step 8: Commit**

```bash
git init
git add .gitignore Dockerfile.dev docker-compose.dev.yml .air.toml Makefile go.mod main.go
git commit -m "$(cat <<'EOF'
chore: initialise le projet Go avec l'environnement Docker de développement

Docker Compose avec Go 1.24 + Node 22 + air (hot reload).
Makefile pour les commandes courantes.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

### Task 1.2: SQLite Database & Migrations

**Files:**
- Create: `internal/database/database.go`
- Create: `internal/database/database_test.go`
- Create: `internal/database/migrations/001_init.up.sql`
- Create: `internal/database/migrations/001_init.down.sql`
- Modify: `go.mod` (add dependencies)

- [ ] **Step 1: Add Go dependencies**

```bash
docker compose -f docker-compose.dev.yml exec app go get \
  github.com/mattn/go-sqlite3 \
  github.com/golang-migrate/migrate/v4 \
  github.com/golang-migrate/migrate/v4/database/sqlite3 \
  github.com/golang-migrate/migrate/v4/source/iofs \
  github.com/stretchr/testify
```

- [ ] **Step 2: Write migration SQL — `internal/database/migrations/001_init.up.sql`**

```sql
CREATE TABLE titles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT NOT NULL CHECK(type IN ('movie', 'series', 'anime')),
    year INTEGER NOT NULL,
    cover_url TEXT,
    imdb_id TEXT,
    anilist_id INTEGER,
    tmdb_id INTEGER,
    tvdb_id INTEGER,
    plex_rating_key TEXT,
    my_rating INTEGER CHECK(my_rating IS NULL OR (my_rating >= 1 AND my_rating <= 10)),
    status TEXT NOT NULL DEFAULT 'watching' CHECK(status IN ('watching', 'completed', 'dropped', 'plan_to_watch')),
    series_status TEXT CHECK(series_status IS NULL OR series_status IN ('returning', 'ended', 'cancelled', 'in_production')),
    match_status TEXT NOT NULL DEFAULT 'confirmed' CHECK(match_status IN ('confirmed', 'pending_review', 'unconfirmed')),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE title_names (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title_id INTEGER NOT NULL REFERENCES titles(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    language TEXT NOT NULL,
    is_primary INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE seasons (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title_id INTEGER NOT NULL REFERENCES titles(id) ON DELETE CASCADE,
    season_number INTEGER NOT NULL,
    total_episodes INTEGER,
    my_rating INTEGER CHECK(my_rating IS NULL OR (my_rating >= 1 AND my_rating <= 10)),
    UNIQUE(title_id, season_number)
);

CREATE TABLE episodes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    season_id INTEGER NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
    episode INTEGER NOT NULL,
    name TEXT,
    air_date TEXT,
    watched INTEGER NOT NULL DEFAULT 0,
    watched_at DATETIME,
    plex_rating_key TEXT,
    UNIQUE(season_id, episode)
);

CREATE TABLE watch_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title_id INTEGER NOT NULL REFERENCES titles(id) ON DELETE CASCADE,
    episode_id INTEGER REFERENCES episodes(id) ON DELETE SET NULL,
    source TEXT NOT NULL CHECK(source IN ('plex', 'manual')),
    plex_payload TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- Indexes
CREATE INDEX idx_titles_status ON titles(status);
CREATE INDEX idx_titles_type ON titles(type);
CREATE INDEX idx_titles_match_status ON titles(match_status);
CREATE INDEX idx_titles_imdb_id ON titles(imdb_id);
CREATE INDEX idx_titles_tmdb_id ON titles(tmdb_id);
CREATE INDEX idx_titles_anilist_id ON titles(anilist_id);
CREATE INDEX idx_titles_plex_rating_key ON titles(plex_rating_key);
CREATE INDEX idx_title_names_title_id ON title_names(title_id);
CREATE INDEX idx_seasons_title_id ON seasons(title_id);
CREATE INDEX idx_episodes_season_id ON episodes(season_id);
CREATE INDEX idx_watch_events_title_id ON watch_events(title_id);
```

- [ ] **Step 3: Write down migration — `internal/database/migrations/001_init.down.sql`**

```sql
DROP TABLE IF EXISTS watch_events;
DROP TABLE IF EXISTS episodes;
DROP TABLE IF EXISTS seasons;
DROP TABLE IF EXISTS title_names;
DROP TABLE IF EXISTS settings;
DROP TABLE IF EXISTS titles;
```

- [ ] **Step 4: Write the failing test — `internal/database/database_test.go`**

```go
package database_test

import (
	"testing"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpen_CreatesDatabase(t *testing.T) {
	db, err := database.Open(":memory:")
	require.NoError(t, err)
	defer db.Close()
	assert.NotNil(t, db)
}

func TestMigrate_CreatesAllTables(t *testing.T) {
	db, err := database.Open(":memory:")
	require.NoError(t, err)
	defer db.Close()

	err = database.Migrate(db)
	require.NoError(t, err)

	// Verify all tables exist
	tables := []string{"titles", "title_names", "seasons", "episodes", "watch_events", "settings"}
	for _, table := range tables {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		assert.NoError(t, err, "table %s should exist", table)
	}
}
```

- [ ] **Step 5: Run test to verify it fails**

```bash
make test
# Expected: FAIL — package database not found
```

- [ ] **Step 6: Implement `internal/database/database.go`**

```go
package database

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/mattn/go-sqlite3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Open creates a new SQLite connection with WAL mode and foreign keys enabled.
func Open(dsn string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dsn+"?_journal_mode=WAL&_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	db.SetMaxOpenConns(1) // SQLite single-writer

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return db, nil
}

// Migrate runs all pending migrations.
func Migrate(db *sql.DB) error {
	source, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("migration source: %w", err)
	}

	driver, err := sqlite3.WithInstance(db, &sqlite3.Config{})
	if err != nil {
		return fmt.Errorf("migration driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", source, "sqlite3", driver)
	if err != nil {
		return fmt.Errorf("migration instance: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("run migrations: %w", err)
	}

	return nil
}
```

- [ ] **Step 7: Run test to verify it passes**

```bash
make test
# Expected: PASS
```

- [ ] **Step 8: Commit**

```bash
git add internal/database/ go.mod go.sum
git commit -m "$(cat <<'EOF'
feat(db): ajoute la base SQLite avec les migrations initiales

Tables: titles, title_names, seasons, episodes, watch_events, settings.
WAL mode, foreign keys, embedded migrations via golang-migrate.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

### Task 1.3: Configuration

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing test**

```go
package config_test

import (
	"testing"

	"github.com/nicolasvasse/plextracker/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "test-client-id")
	t.Setenv("GOOGLE_ALLOWED_EMAIL", "test@example.com")
	t.Setenv("JWT_SECRET", "test-secret")

	cfg, err := config.Load()
	require.NoError(t, err)

	assert.Equal(t, "test-client-id", cfg.GoogleClientID)
	assert.Equal(t, "test@example.com", cfg.GoogleAllowedEmail)
	assert.Equal(t, ":8080", cfg.ListenAddr)
	assert.Equal(t, "/data", cfg.DataDir)
}

func TestLoad_MissingRequired(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "")
	t.Setenv("GOOGLE_ALLOWED_EMAIL", "")
	t.Setenv("JWT_SECRET", "")

	_, err := config.Load()
	assert.Error(t, err)
}
```

- [ ] **Step 2: Run test, verify fail**

- [ ] **Step 3: Implement `internal/config/config.go`**

```go
package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	ListenAddr        string
	DataDir           string
	GoogleClientID    string
	GoogleAllowedEmail string
	JWTSecret         string
	TMDBAPIKey        string
	AniListClientID   string
	AniListClientSecret string
	GeminiAPIKeys     []string // Rotation pool
	VAPIDPublicKey    string
	VAPIDPrivateKey   string
	VAPIDSubject      string
}

func Load() (*Config, error) {
	cfg := &Config{
		ListenAddr:         envOr("LISTEN_ADDR", ":8080"),
		DataDir:            envOr("DATA_DIR", "/data"),
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleAllowedEmail: os.Getenv("GOOGLE_ALLOWED_EMAIL"),
		JWTSecret:          os.Getenv("JWT_SECRET"),
		TMDBAPIKey:         os.Getenv("TMDB_API_KEY"),
		AniListClientID:    os.Getenv("ANILIST_CLIENT_ID"),
		AniListClientSecret: os.Getenv("ANILIST_CLIENT_SECRET"),
		VAPIDPublicKey:     os.Getenv("VAPID_PUBLIC_KEY"),
		VAPIDPrivateKey:    os.Getenv("VAPID_PRIVATE_KEY"),
		VAPIDSubject:       os.Getenv("VAPID_SUBJECT"),
	}

	if keys := os.Getenv("GEMINI_API_KEY"); keys != "" {
		cfg.GeminiAPIKeys = strings.Split(keys, ",")
	}

	if cfg.GoogleClientID == "" || cfg.GoogleAllowedEmail == "" || cfg.JWTSecret == "" {
		return nil, fmt.Errorf("required env vars: GOOGLE_CLIENT_ID, GOOGLE_ALLOWED_EMAIL, JWT_SECRET")
	}

	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
```

- [ ] **Step 4: Run test, verify pass**

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "$(cat <<'EOF'
feat(config): ajoute le chargement de la configuration depuis les variables d'environnement

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

### Task 1.4: Models & Enums

**Files:**
- Create: `internal/model/title.go`
- Create: `internal/model/season.go`
- Create: `internal/model/episode.go`
- Create: `internal/model/watch_event.go`
- Create: `internal/model/setting.go`

- [ ] **Step 1: Create all model files**

`internal/model/title.go`:
```go
package model

import "time"

type TitleType string

const (
	TitleTypeMovie  TitleType = "movie"
	TitleTypeSeries TitleType = "series"
	TitleTypeAnime  TitleType = "anime"
)

type TitleStatus string

const (
	TitleStatusWatching    TitleStatus = "watching"
	TitleStatusCompleted   TitleStatus = "completed"
	TitleStatusDropped     TitleStatus = "dropped"
	TitleStatusPlanToWatch TitleStatus = "plan_to_watch"
)

type SeriesStatus string

const (
	SeriesStatusReturning    SeriesStatus = "returning"
	SeriesStatusEnded        SeriesStatus = "ended"
	SeriesStatusCancelled    SeriesStatus = "cancelled"
	SeriesStatusInProduction SeriesStatus = "in_production"
)

type MatchStatus string

const (
	MatchStatusConfirmed     MatchStatus = "confirmed"
	MatchStatusPendingReview MatchStatus = "pending_review"
	MatchStatusUnconfirmed   MatchStatus = "unconfirmed"
)

type Title struct {
	ID             int64         `json:"id"`
	Type           TitleType     `json:"type"`
	Year           int           `json:"year"`
	CoverURL       *string       `json:"cover_url"`
	IMDBID         *string       `json:"imdb_id"`
	AniListID      *int64        `json:"anilist_id"`
	TMDBID         *int64        `json:"tmdb_id"`
	TVDBID         *int64        `json:"tvdb_id"`
	PlexRatingKey  *string       `json:"plex_rating_key"`
	MyRating       *int          `json:"my_rating"`
	Status         TitleStatus   `json:"status"`
	SeriesStatus   *SeriesStatus `json:"series_status"`
	MatchStatus    MatchStatus   `json:"match_status"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`

	// Loaded relations
	Names    []TitleName `json:"names,omitempty"`
	Seasons  []Season    `json:"seasons,omitempty"`
}

// PrimaryName returns the primary display name, or the first name if none is primary.
func (t *Title) PrimaryName() string {
	for _, n := range t.Names {
		if n.IsPrimary {
			return n.Name
		}
	}
	if len(t.Names) > 0 {
		return t.Names[0].Name
	}
	return ""
}

type TitleName struct {
	ID        int64  `json:"id"`
	TitleID   int64  `json:"title_id"`
	Name      string `json:"name"`
	Language  string `json:"language"`
	IsPrimary bool   `json:"is_primary"`
}
```

`internal/model/season.go`:
```go
package model

type Season struct {
	ID            int64    `json:"id"`
	TitleID       int64    `json:"title_id"`
	SeasonNumber  int      `json:"season_number"`
	TotalEpisodes *int     `json:"total_episodes"`
	MyRating      *int     `json:"my_rating"`

	// Loaded relations
	Episodes []Episode `json:"episodes,omitempty"`
}
```

`internal/model/episode.go`:
```go
package model

import "time"

type Episode struct {
	ID            int64      `json:"id"`
	SeasonID      int64      `json:"season_id"`
	Episode       int        `json:"episode"`
	Name          *string    `json:"name"`
	AirDate       *string    `json:"air_date"`
	Watched       bool       `json:"watched"`
	WatchedAt     *time.Time `json:"watched_at"`
	PlexRatingKey *string    `json:"plex_rating_key"`
}
```

`internal/model/watch_event.go`:
```go
package model

import "time"

type WatchEventSource string

const (
	WatchEventSourcePlex   WatchEventSource = "plex"
	WatchEventSourceManual WatchEventSource = "manual"
)

type WatchEvent struct {
	ID          int64            `json:"id"`
	TitleID     int64            `json:"title_id"`
	EpisodeID   *int64           `json:"episode_id"`
	Source      WatchEventSource `json:"source"`
	PlexPayload *string          `json:"plex_payload"`
	CreatedAt   time.Time        `json:"created_at"`
}
```

`internal/model/setting.go`:
```go
package model

type Setting struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}
```

- [ ] **Step 2: Verify compilation**

```bash
make test
# Expected: PASS (models compile, existing tests still pass)
```

- [ ] **Step 3: Commit**

```bash
git add internal/model/
git commit -m "$(cat <<'EOF'
feat(model): ajoute les modèles Go et enums (Title, Season, Episode, WatchEvent, Setting)

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

### Task 1.5: Preact Frontend Scaffold

**Files:**
- Create: `frontend/package.json`
- Create: `frontend/vite.config.ts`
- Create: `frontend/tsconfig.json`
- Create: `frontend/index.html`
- Create: `frontend/src/main.tsx`
- Create: `frontend/src/app.tsx`
- Create: `frontend/src/theme.ts`
- Create: `frontend/src/types.ts`
- Create: `frontend/src/api.ts`

- [ ] **Step 1: Initialize frontend inside container**

```bash
docker compose -f docker-compose.dev.yml exec app bash -c "
  cd frontend && npm init -y && \
  npm install preact preact-router && \
  npm install -D typescript vite @preact/preset-vite vitest @testing-library/preact jsdom
"
```

- [ ] **Step 2: Create `frontend/vite.config.ts`**

```typescript
import { defineConfig } from 'vite'
import preact from '@preact/preset-vite'

export default defineConfig({
  plugins: [preact()],
  server: {
    host: '0.0.0.0',
    port: 5173,
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
})
```

- [ ] **Step 3: Create `frontend/tsconfig.json`**

```json
{
  "compilerOptions": {
    "target": "ES2020",
    "module": "ESNext",
    "moduleResolution": "bundler",
    "jsx": "react-jsx",
    "jsxImportSource": "preact",
    "strict": true,
    "noEmit": true,
    "skipLibCheck": true,
    "paths": {
      "react": ["./node_modules/preact/compat/"],
      "react-dom": ["./node_modules/preact/compat/"]
    }
  },
  "include": ["src"]
}
```

- [ ] **Step 4: Create `frontend/index.html`**

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0, viewport-fit=cover" />
  <meta name="theme-color" content="#0D0D0D" />
  <title>PlexTracker</title>
  <link rel="preconnect" href="https://fonts.googleapis.com" />
  <link href="https://fonts.googleapis.com/css2?family=DM+Sans:wght@400;500;700&display=swap" rel="stylesheet" />
  <style>
    *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      font-family: 'DM Sans', sans-serif;
      background: #0D0D0D;
      color: #F0F0F0;
      -webkit-font-smoothing: antialiased;
    }
  </style>
</head>
<body>
  <div id="app"></div>
  <script type="module" src="/src/main.tsx"></script>
</body>
</html>
```

- [ ] **Step 5: Create `frontend/src/theme.ts`**

Design tokens from the UI spec.

```typescript
export const colors = {
  bgPrimary: '#0D0D0D',
  bgCard: '#161616',
  bgSurface: '#1E1E1E',
  borderSubtle: '#1A1A1A',
  borderCard: '#222222',
  textPrimary: '#F0F0F0',
  textSecondary: '#666666',
  textMuted: '#555555',
  textDimmed: '#444444',

  accentAmber: '#E8A925',
  accentTeal: '#38BDB0',
  accentGreen: '#4CAF50',
  accentLavender: '#9575CD',
  accentCoral: '#EB5757',
  accentBlue: '#5B9CF6',
  accentImdb: '#F5C518',
  accentAnilist: '#02A9FF',
} as const

export const accentWash = (hex: string) => `${hex}1F` // ~12% opacity
```

- [ ] **Step 6: Create `frontend/src/types.ts`**

```typescript
export type TitleType = 'movie' | 'series' | 'anime'
export type TitleStatus = 'watching' | 'completed' | 'dropped' | 'plan_to_watch'
export type SeriesStatus = 'returning' | 'ended' | 'cancelled' | 'in_production'
export type MatchStatus = 'confirmed' | 'pending_review' | 'unconfirmed'

export interface Title {
  id: number
  type: TitleType
  year: number
  cover_url: string | null
  imdb_id: string | null
  anilist_id: number | null
  tmdb_id: number | null
  tvdb_id: number | null
  my_rating: number | null
  status: TitleStatus
  series_status: SeriesStatus | null
  match_status: MatchStatus
  names: TitleName[]
  seasons: Season[]
}

export interface TitleName {
  id: number
  title_id: number
  name: string
  language: string
  is_primary: boolean
}

export interface Season {
  id: number
  title_id: number
  season_number: number
  total_episodes: number | null
  my_rating: number | null
  episodes: Episode[]
}

export interface Episode {
  id: number
  season_id: number
  episode: number
  name: string | null
  air_date: string | null
  watched: boolean
  watched_at: string | null
}
```

- [ ] **Step 7: Create `frontend/src/api.ts`**

```typescript
const BASE = '/api'

export class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message)
  }
}

export async function apiFetch<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json', ...options?.headers },
    ...options,
  })

  if (res.status === 401) {
    window.location.href = '/login'
    throw new ApiError(401, 'Unauthorized')
  }

  if (!res.ok) {
    const text = await res.text()
    throw new ApiError(res.status, text)
  }

  if (res.status === 204) return undefined as T

  return res.json()
}
```

- [ ] **Step 8: Create `frontend/src/app.tsx` and `frontend/src/main.tsx`**

`frontend/src/app.tsx`:
```tsx
import Router from 'preact-router'

export function App() {
  return (
    <div style={{ minHeight: '100vh' }}>
      <Router>
        <Home path="/" />
      </Router>
    </div>
  )
}

function Home() {
  return (
    <div style={{ padding: '20px' }}>
      <h1 style={{ fontSize: '20px', fontWeight: 700 }}>PlexTracker</h1>
      <p style={{ color: '#666', marginTop: '8px' }}>Coming soon.</p>
    </div>
  )
}
```

`frontend/src/main.tsx`:
```tsx
import { render } from 'preact'
import { App } from './app'

render(<App />, document.getElementById('app')!)
```

- [ ] **Step 9: Verify frontend builds**

```bash
docker compose -f docker-compose.dev.yml exec app bash -c "cd frontend && npx vite build"
# Expected: build succeeds, outputs to frontend/dist/
```

- [ ] **Step 10: Commit**

```bash
git add frontend/
git commit -m "$(cat <<'EOF'
feat(frontend): initialise le projet Preact avec Vite, thème et types

Preact 10, preact-router, TypeScript, tokens couleurs du design system.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase 2: HTTP Server & Auth

### Task 2.1: Chi Router & SPA Serving

**Files:**
- Create: `internal/router/router.go`
- Create: `internal/handler/spa.go`
- Modify: `cmd/serve.go` (new file)
- Modify: `main.go`

- [ ] **Step 1: Add chi dependency**

```bash
docker compose -f docker-compose.dev.yml exec app go get github.com/go-chi/chi/v5
```

- [ ] **Step 2: Create `internal/handler/spa.go`**

Serves the embedded Preact SPA. In dev mode, proxies to Vite. In production, serves from embedded `frontend/dist/`.

```go
package handler

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

// SPAHandler serves the embedded Preact SPA.
// For any path not starting with /api, it serves static files or falls back to index.html.
func SPAHandler(distFS embed.FS) http.Handler {
	sub, _ := fs.Sub(distFS, "frontend/dist")
	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")

		// Try serving the file directly
		if f, err := sub.Open(path); err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// Fallback to index.html for client-side routing
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
```

- [ ] **Step 3: Create `internal/router/router.go`**

```go
package router

import (
	"database/sql"
	"embed"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/nicolasvasse/plextracker/internal/config"
	"github.com/nicolasvasse/plextracker/internal/handler"
)

func New(cfg *config.Config, db *sql.DB, distFS embed.FS) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))

	// API routes
	r.Route("/api", func(r chi.Router) {
		r.Get("/health", handler.Health)
	})

	// SPA catch-all
	r.Handle("/*", handler.SPAHandler(distFS))

	return r
}
```

Add `handler.Health`:
```go
// In internal/handler/spa.go or a new health.go
func Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}
```

- [ ] **Step 4: Create `cmd/serve.go`**

```go
package cmd

import (
	"embed"
	"fmt"
	"log"
	"net/http"

	"github.com/nicolasvasse/plextracker/internal/config"
	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/router"
)

func Serve(distFS embed.FS) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	dbPath := cfg.DataDir + "/plextracker.db"
	db, err := database.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	if err := database.Migrate(db); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	r := router.New(cfg, db, distFS)

	log.Printf("PlexTracker listening on %s", cfg.ListenAddr)
	return http.ListenAndServe(cfg.ListenAddr, r)
}
```

- [ ] **Step 5: Update `main.go`**

```go
package main

import (
	"embed"
	"fmt"
	"log"
	"os"

	"github.com/nicolasvasse/plextracker/cmd"
)

//go:embed frontend/dist
var distFS embed.FS

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: plextracker <serve|import|migrate>")
		os.Exit(1)
	}

	var err error
	switch os.Args[1] {
	case "serve":
		err = cmd.Serve(distFS)
	case "import":
		fmt.Println("Import not yet implemented")
	case "migrate":
		fmt.Println("Migrate not yet implemented")
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}

	if err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 6: Verify it builds and health endpoint works**

```bash
# Build frontend first so embed works
docker compose -f docker-compose.dev.yml exec app bash -c "cd frontend && npx vite build"
docker compose -f docker-compose.dev.yml exec app go build -o ./tmp/plextracker .
# Test health (start in background, curl, stop)
docker compose -f docker-compose.dev.yml exec -e DATA_DIR=/tmp app bash -c "./tmp/plextracker serve &
  sleep 1 && curl -s http://localhost:8080/api/health && kill %1"
# Expected: {"status":"ok"}
```

- [ ] **Step 7: Commit**

```bash
git add cmd/ internal/router/ internal/handler/ main.go go.mod go.sum
git commit -m "$(cat <<'EOF'
feat(server): ajoute le serveur HTTP chi avec le health check et le service du SPA

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

### Task 2.2: Google OAuth & JWT Middleware

**Files:**
- Create: `internal/handler/auth.go`
- Create: `internal/handler/auth_test.go`
- Create: `internal/middleware/auth.go`
- Create: `internal/middleware/auth_test.go`
- Modify: `internal/router/router.go`

- [ ] **Step 1: Add JWT dependency**

```bash
docker compose -f docker-compose.dev.yml exec app go get github.com/golang-jwt/jwt/v5
```

- [ ] **Step 2: Write failing test for JWT middleware**

`internal/middleware/auth_test.go`:
```go
package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	mw "github.com/nicolasvasse/plextracker/internal/middleware"
	"github.com/stretchr/testify/assert"
)

func TestJWTAuth_ValidToken(t *testing.T) {
	secret := "test-secret"
	token := createTestToken(t, secret, "user@test.com", time.Now().Add(time.Hour))

	handler := mw.JWTAuth(secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestJWTAuth_MissingToken(t *testing.T) {
	handler := mw.JWTAuth("secret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestJWTAuth_ExpiredToken(t *testing.T) {
	secret := "test-secret"
	token := createTestToken(t, secret, "user@test.com", time.Now().Add(-time.Hour))

	handler := mw.JWTAuth(secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func createTestToken(t *testing.T, secret, email string, exp time.Time) string {
	t.Helper()
	claims := jwt.MapClaims{"email": email, "exp": jwt.NewNumericDate(exp)}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return s
}
```

- [ ] **Step 3: Run test, verify fail**

- [ ] **Step 4: Implement `internal/middleware/auth.go`**

```go
package middleware

import (
	"context"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const EmailKey contextKey = "email"

// JWTAuth validates the JWT token from the "token" cookie.
func JWTAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("token")
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			token, err := jwt.Parse(cookie.Value, func(t *jwt.Token) (interface{}, error) {
				return []byte(secret), nil
			}, jwt.WithValidMethods([]string{"HS256"}))

			if err != nil || !token.Valid {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			email, _ := claims["email"].(string)
			ctx := context.WithValue(r.Context(), EmailKey, email)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
```

- [ ] **Step 5: Run test, verify pass**

- [ ] **Step 6: Write failing test for auth handler**

`internal/handler/auth_test.go` — test the Google OAuth callback that verifies the ID token, checks the email, and issues a JWT cookie.

```go
package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nicolasvasse/plextracker/internal/handler"
	"github.com/stretchr/testify/assert"
)

func TestLogout_ClearsCookie(t *testing.T) {
	h := handler.NewAuthHandler("secret", "test@example.com", "client-id")
	req := httptest.NewRequest("POST", "/api/auth/logout", nil)
	rr := httptest.NewRecorder()

	h.Logout(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	cookies := rr.Result().Cookies()
	assert.Len(t, cookies, 1)
	assert.Equal(t, "token", cookies[0].Name)
	assert.Equal(t, "", cookies[0].Value)
	assert.True(t, cookies[0].MaxAge < 0)
}
```

- [ ] **Step 7: Implement `internal/handler/auth.go`**

```go
package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type AuthHandler struct {
	jwtSecret    string
	allowedEmail string
	clientID     string
}

func NewAuthHandler(jwtSecret, allowedEmail, clientID string) *AuthHandler {
	return &AuthHandler{
		jwtSecret:    jwtSecret,
		allowedEmail: allowedEmail,
		clientID:     clientID,
	}
}

// GoogleCallback verifies the Google ID token and issues a JWT cookie.
func (h *AuthHandler) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Credential string `json:"credential"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Verify Google ID token via Google's tokeninfo endpoint
	resp, err := http.Get(fmt.Sprintf("https://oauth2.googleapis.com/tokeninfo?id_token=%s", body.Credential))
	if err != nil || resp.StatusCode != 200 {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}
	defer resp.Body.Close()

	var tokenInfo struct {
		Email string `json:"email"`
		Aud   string `json:"aud"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenInfo); err != nil {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	if tokenInfo.Email != h.allowedEmail || tokenInfo.Aud != h.clientID {
		http.Error(w, "Unauthorized", http.StatusForbidden)
		return
	}

	// Issue JWT
	claims := jwt.MapClaims{
		"email": tokenInfo.Email,
		"exp":   jwt.NewNumericDate(time.Now().Add(365 * 24 * time.Hour)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(h.jwtSecret))
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    signed,
		Path:     "/",
		MaxAge:   365 * 24 * 3600,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
	w.WriteHeader(http.StatusOK)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
	w.WriteHeader(http.StatusOK)
}
```

- [ ] **Step 8: Run tests, verify pass**

- [ ] **Step 9: Wire auth into router**

Update `internal/router/router.go` to add auth routes and protect API routes:

```go
// In the /api route group:
r.Route("/api", func(r chi.Router) {
    r.Get("/health", handler.Health)

    // Auth (unauthenticated)
    auth := handler.NewAuthHandler(cfg.JWTSecret, cfg.GoogleAllowedEmail, cfg.GoogleClientID)
    r.Post("/auth/google", auth.GoogleCallback)
    r.Post("/auth/logout", auth.Logout)

    // Plex webhook (unauthenticated)
    // r.Post("/webhook/plex", webhookHandler.Handle)

    // Authenticated routes
    r.Group(func(r chi.Router) {
        r.Use(mw.JWTAuth(cfg.JWTSecret))
        // Title routes will go here
    })
})
```

- [ ] **Step 10: Commit**

```bash
git add internal/handler/auth*.go internal/middleware/ internal/router/ go.mod go.sum
git commit -m "$(cat <<'EOF'
feat(auth): ajoute l'authentification Google OAuth et le middleware JWT

Login via Google ID token, JWT en cookie HttpOnly Secure SameSite=Strict.
Email unique autorisé via GOOGLE_ALLOWED_EMAIL.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase 3: Core CRUD API

### Task 3.1: Title Repository

**Files:**
- Create: `internal/repository/title.go`
- Create: `internal/repository/title_test.go`

- [ ] **Step 1: Write failing tests**

Test: `Create`, `GetByID` (with names, seasons, episodes), `List` (with status/type/search filters), `Update`.

```go
package repository_test

import (
	"testing"

	"github.com/nicolasvasse/plextracker/internal/database"
	"github.com/nicolasvasse/plextracker/internal/model"
	"github.com/nicolasvasse/plextracker/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *repository.TitleRepository {
	t.Helper()
	db, err := database.Open(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.Migrate(db))
	t.Cleanup(func() { db.Close() })
	return repository.NewTitleRepository(db)
}

func TestTitleRepository_CreateAndGet(t *testing.T) {
	repo := setupTestDB(t)

	title := &model.Title{
		Type:        model.TitleTypeMovie,
		Year:        2024,
		Status:      model.TitleStatusWatching,
		MatchStatus: model.MatchStatusConfirmed,
	}
	names := []model.TitleName{{Name: "Dune", Language: "en", IsPrimary: true}}

	id, err := repo.Create(title, names)
	require.NoError(t, err)
	assert.Greater(t, id, int64(0))

	got, err := repo.GetByID(id)
	require.NoError(t, err)
	assert.Equal(t, "Dune", got.PrimaryName())
	assert.Equal(t, model.TitleTypeMovie, got.Type)
	assert.Equal(t, 2024, got.Year)
}

func TestTitleRepository_ListByStatus(t *testing.T) {
	repo := setupTestDB(t)

	repo.Create(&model.Title{Type: model.TitleTypeMovie, Year: 2024, Status: model.TitleStatusWatching, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "A", Language: "en", IsPrimary: true}})
	repo.Create(&model.Title{Type: model.TitleTypeSeries, Year: 2023, Status: model.TitleStatusCompleted, MatchStatus: model.MatchStatusConfirmed}, []model.TitleName{{Name: "B", Language: "en", IsPrimary: true}})

	titles, err := repo.List(repository.TitleFilter{Status: ptr(model.TitleStatusWatching)})
	require.NoError(t, err)
	assert.Len(t, titles, 1)
	assert.Equal(t, "A", titles[0].PrimaryName())
}

func ptr[T any](v T) *T { return &v }
```

- [ ] **Step 2: Run test, verify fail**

- [ ] **Step 3: Implement `internal/repository/title.go`**

Full repository with `Create`, `GetByID` (loads names + seasons + episodes), `List` (filters: status, type, search, match_status), `Update` (PATCH semantics).

Key patterns:
- `TitleFilter` struct with optional fields for query building
- Search = `LIKE` on `title_names.name`
- `GetByID` does 3 queries: title + names, seasons, episodes (avoids N+1 while staying simple)

- [ ] **Step 4: Run test, verify pass**

- [ ] **Step 5: Commit**

### Task 3.2: Season & Episode Repositories

**Files:**
- Create: `internal/repository/season.go`
- Create: `internal/repository/episode.go`
- Create: `internal/repository/episode_test.go`

Same pattern as TitleRepository. Key operations:
- `SeasonRepository`: `GetOrCreate(titleID, seasonNumber)`, `UpdateRating(id, rating)`
- `EpisodeRepository`: `ToggleWatched(id)`, `BatchMarkWatched(ids, watchedAt)`, `GetBySeasonID(seasonID)`

- [ ] **Step 1-5**: TDD cycle (write test → fail → implement → pass → commit)

### Task 3.3: WatchEvent & Setting Repositories

**Files:**
- Create: `internal/repository/watch_event.go`
- Create: `internal/repository/setting.go`

- `WatchEventRepository`: `Create(event)`, `ListByTitle(titleID)`
- `SettingRepository`: `Get(key)`, `Set(key, value)`, `Delete(key)`

- [ ] **Step 1-5**: TDD cycle → commit

### Task 3.4: Title API Handlers

**Files:**
- Create: `internal/handler/title.go`
- Create: `internal/handler/title_test.go`
- Modify: `internal/router/router.go`

Endpoints:
- `GET /api/titles` — list with filters (query params: `status`, `type`, `search`, `match_status`)
- `GET /api/titles/:id` — detail with seasons, episodes, names
- `POST /api/titles` — manual add (used by import and add screen)
- `PATCH /api/titles/:id` — update status, type, rating, match_status

Handler struct holds repositories (dependency injection):

```go
type TitleHandler struct {
	titles   *repository.TitleRepository
	seasons  *repository.SeasonRepository
	episodes *repository.EpisodeRepository
	events   *repository.WatchEventRepository
}
```

- [ ] **Step 1: Write failing test for GET /api/titles**

```go
func TestListTitles_FilterByStatus(t *testing.T) {
	db := setupTestDB(t)
	h := handler.NewTitleHandler(
		repository.NewTitleRepository(db),
		repository.NewSeasonRepository(db),
		repository.NewEpisodeRepository(db),
		repository.NewWatchEventRepository(db),
	)

	// Seed data...
	req := httptest.NewRequest("GET", "/api/titles?status=watching", nil)
	rr := httptest.NewRecorder()
	h.List(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var titles []model.Title
	json.NewDecoder(rr.Body).Decode(&titles)
	assert.Len(t, titles, 1)
}
```

- [ ] **Step 2-8**: Implement all 4 endpoints following TDD cycle

- [ ] **Step 9: Wire routes**

```go
// In authenticated group:
titles := handler.NewTitleHandler(titleRepo, seasonRepo, episodeRepo, eventRepo)
r.Get("/titles", titles.List)
r.Get("/titles/{id}", titles.GetByID)
r.Post("/titles", titles.Create)
r.Patch("/titles/{id}", titles.Update)
```

- [ ] **Step 10: Commit**

### Task 3.5: Episode & Season API Handlers

**Files:**
- Create: `internal/handler/episode.go`
- Create: `internal/handler/season.go`
- Modify: `internal/router/router.go`

Endpoints:
- `PATCH /api/titles/:id/episodes/:id` — toggle watched
- `POST /api/titles/:id/episodes/batch-watch` — batch mark watched
- `PATCH /api/titles/:id/seasons/:id` — update season rating

On episode toggle/batch-watch:
1. Mark episode(s) as watched with `watched_at = now()`
2. Log `watch_event` with `source = manual`
3. Auto-update title status: all eps watched + ended/cancelled → `completed`
4. Return updated title status in response (frontend needs this)

- [ ] **Step 1-8**: TDD cycle for all 3 endpoints → commit

### Task 3.6: Cover Serving

**Files:**
- Create: `internal/handler/cover.go`

Simple: serve files from `{DATA_DIR}/covers/{filename}`. Validate filename (no path traversal). Set `Cache-Control: public, max-age=604800`.

- [ ] **Step 1-4**: TDD cycle → commit

---

## Phase 4: Simkl Import

### Task 4.1: Simkl Import Service

**Files:**
- Create: `internal/service/simkl.go`
- Create: `internal/service/simkl_test.go`
- Modify: `cmd/import.go`

- [ ] **Step 1: Write failing test with sample Simkl JSON**

Create a test fixture with a minimal `SimklBackup.json` containing one movie, one show, one anime. Test:
- Type mapping (movies → movie, shows → series, anime → anime, anime_type=movie → movie)
- Status mapping (completed, watching, plantowatch, hold → watching, notinteresting → dropped)
- External IDs stored
- Rating imported
- Watched episodes created with correct `watched_at`
- Duplicate skip (re-import same data → no new rows)

```go
func TestSimklImport_Movie(t *testing.T) {
	db := setupTestDB(t)
	importer := service.NewSimklImporter(
		repository.NewTitleRepository(db),
		repository.NewSeasonRepository(db),
		repository.NewEpisodeRepository(db),
		repository.NewWatchEventRepository(db),
	)

	backup := &service.SimklBackup{
		Movies: []service.SimklItem{{
			Title:          "Dune: Part Two",
			Year:           2024,
			Status:         "completed",
			UserRating:     intPtr(9),
			LastWatchedAt:  "2024-03-15T20:00:00Z",
			IDs:            service.SimklIDs{IMDB: "tt15239678", TMDB: 693134},
		}},
	}

	result, err := importer.Import(backup, false)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Created)
	assert.Equal(t, 0, result.Skipped)
}
```

- [ ] **Step 2: Run test, verify fail**

- [ ] **Step 3: Implement `internal/service/simkl.go`**

Key logic:
- Parse the backup JSON structure (3 arrays: movies, anime, shows)
- For each item: create title + names + seasons/episodes (watched only) + watch_event
- Skip if imdb_id or tmdb_id already exists in DB
- `--dry-run` mode: wrap everything in a transaction and rollback

- [ ] **Step 4: Run test, verify pass**

- [ ] **Step 5: Implement `cmd/import.go`**

CLI command that:
1. Accepts path to zip file (or extracted JSON)
2. Unzips to temp dir, reads `SimklBackup.json`
3. Parses and calls `SimklImporter.Import()`
4. Prints summary: created, skipped, errors

```go
func Import(args []string) error {
	dryRun := false
	path := ""
	for _, arg := range args {
		if arg == "--dry-run" {
			dryRun = true
		} else {
			path = arg
		}
	}
	// ... open DB, parse zip, run import
}
```

- [ ] **Step 6: Test with the actual Simkl backup**

```bash
# Copy the zip into the container
docker cp Simkl_backup_03.05.2026_22.40.zip plextracker-dev:/tmp/
# Dry run first
make import BACKUP_FILE="--dry-run /tmp/Simkl_backup_03.05.2026_22.40.zip"
# Then real import
make import BACKUP_FILE="/tmp/Simkl_backup_03.05.2026_22.40.zip"
```

- [ ] **Step 7: Commit**

```bash
git add internal/service/simkl*.go cmd/import.go
git commit -m "$(cat <<'EOF'
feat(import): ajoute l'import Simkl (films, séries, anime)

Mapping des statuts et types, import des épisodes regardés,
support --dry-run, skip des doublons par imdb_id/tmdb_id.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase 5: Plex Webhook

### Task 5.1: Plex Webhook Handler

**Files:**
- Create: `internal/service/plex.go`
- Create: `internal/service/plex_test.go`
- Create: `internal/handler/webhook.go`
- Create: `internal/handler/webhook_test.go`
- Modify: `internal/router/router.go`

- [ ] **Step 1: Write failing test for Plex payload parsing**

Test with a real-ish `media.scrobble` multipart payload. Extract: title, year, type, season/episode numbers, ratingKey, IMDB/TMDB/TVDB IDs from Plex's nested Metadata structure.

- [ ] **Step 2: Run test, verify fail**

- [ ] **Step 3: Implement `internal/service/plex.go`**

```go
type PlexPayload struct {
	Event    string       `json:"event"`
	Metadata PlexMetadata `json:"Metadata"`
}

type PlexMetadata struct {
	Title           string `json:"title"`
	GrandparentTitle string `json:"grandparentTitle"` // Series name for episodes
	Year            int    `json:"year"`
	Type            string `json:"type"`              // "movie", "episode"
	ParentIndex     int    `json:"parentIndex"`       // Season number
	Index           int    `json:"index"`             // Episode number
	RatingKey       string `json:"ratingKey"`
	GrandparentRatingKey string `json:"grandparentRatingKey"` // Series ratingKey
	GUID            []PlexGUID `json:"Guid"`
}

type PlexGUID struct {
	ID string `json:"id"` // "imdb://tt1234567", "tmdb://12345", "tvdb://12345"
}
```

Parse multipart form, extract JSON payload from `payload` field. Parse GUIDs to extract external IDs.

- [ ] **Step 4: Run test, verify pass**

- [ ] **Step 5: Write failing test for webhook processing logic**

Test the full flow: receive scrobble → find/create title → mark episode watched → log watch_event → auto-update status.

- [ ] **Step 6: Implement webhook handler**

Processing flow (from spec):
1. Parse multipart payload, check `event == "media.scrobble"`
2. Extract IDs from GUIDs
3. Lookup title by `plex_rating_key` or external IDs
4. If new → create title with available metadata, set `match_status = confirmed` if Plex IDs found (full matching pipeline deferred to Phase 6)
5. Mark episode/movie as watched, log watch_event
6. Re-watches: log event but don't update episode
7. Auto-update status: all watched + ended → completed

- [ ] **Step 7: Wire route (unauthenticated)**

```go
r.Post("/webhook/plex", webhookHandler.Handle)
```

- [ ] **Step 8: Commit**

---

## Phase 6: Media Matching Pipeline

### Task 6.1: TMDB API Client

**Files:**
- Create: `internal/service/matching/tmdb.go`
- Create: `internal/service/matching/tmdb_test.go`

- [ ] **Step 1: Write failing test**

Test: search by title+year, parse response, extract TMDB ID + IMDB ID. Use `httptest.NewServer` to mock the TMDB API.

- [ ] **Step 2-4: Implement + verify**

Key operations:
- `SearchMovie(title, year)` → TMDB result with IDs
- `SearchTV(title, year)` → TMDB result with IDs
- `GetMovieDetails(tmdbID)` → full details with IMDB ID
- `GetTVDetails(tmdbID)` → full details with IMDB ID, seasons, episodes
- `GetTVSeasonEpisodes(tmdbID, seasonNumber)` → episode list with names + air dates
- `GetTitleNames(tmdbID, type)` → multilingual names (en, fr)
- `DownloadCover(posterPath, destDir)` → saves to `/data/covers/`

- [ ] **Step 5: Commit**

### Task 6.2: AniList GraphQL Client

**Files:**
- Create: `internal/service/matching/anilist.go`
- Create: `internal/service/matching/anilist_test.go`

Key operations:
- `SearchAnime(title)` → AniList result with IDs, names (romaji, english)
- `GetAnimeDetails(anilistID)` → full details with episodes
- `SyncRating(anilistID, rating, accessToken)` → GraphQL mutation
- `GetNames(anilistID)` → romaji, english names

- [ ] **Step 1-5: TDD cycle → commit**

### Task 6.3: Cross-Reference Database

**Files:**
- Create: `internal/service/matching/crossref.go`
- Create: `internal/service/matching/crossref_test.go`

Uses [anime-offline-database](https://github.com/manami-project/anime-offline-database) JSON. Downloaded to `/data/anime-offline-database.json`.

Key operation: `Lookup(ids ExternalIDs) → ExternalIDs` — given any subset of IDs, returns all known cross-references.

- [ ] **Step 1-5: TDD cycle → commit**

### Task 6.4: Gemini AI Verification

**Files:**
- Create: `internal/service/matching/gemini.go`
- Create: `internal/service/matching/gemini_test.go`

Key operations:
- `VerifyMatch(source PlexInfo, candidate MatchCandidate)` → `{confirmed bool, confidence string, reason string}`
- `FuzzyResolve(source PlexInfo)` → `{candidateTitle, candidateYear, confidence, reason}`
- API key rotation: cycle through `GeminiAPIKeys` on 429 responses

- [ ] **Step 1-5: TDD cycle → commit**

### Task 6.5: Pipeline Orchestrator

**Files:**
- Create: `internal/service/matching/pipeline.go`
- Create: `internal/service/matching/pipeline_test.go`

Orchestrates Steps 1-5 from the spec:
1. Check Plex metadata for IDs → if found, `confirmed`
2. Cross-reference database lookup → if found, `confirmed`
3. TMDB search by title+year → if found, go to Step 5
4. AniList search (anime only) → if found, go to Step 5
5. Gemini verification → `pending_review` (high) or `unconfirmed` (low)

At every step: store newly found IDs, use them for the next step.

After matching: fetch full episode list + multilingual names + cover.

- [ ] **Step 1: Write failing test for the full pipeline**

Test with a mock Plex payload that has no external IDs, forcing the pipeline through all steps.

- [ ] **Step 2-4: Implement + verify**

- [ ] **Step 5: Integrate pipeline into webhook handler**

Replace the simple "create title" in Task 5.1 with the full pipeline.

- [ ] **Step 6: Commit**

---

## Phase 7: Frontend — Core Screens

### Task 7.1: Navbar & App Shell

**Files:**
- Create: `frontend/src/components/Navbar.tsx`
- Modify: `frontend/src/app.tsx`

Implement the 4-tab bottom navbar with identity colors (amber/teal/green/lavender). Active state: 2px top border + wash + colored icon + colored label. Reference: `docs/mockups/navbar-v3.html`.

Use Lucide-style inline SVGs (monitor, search, circle-plus, bar-chart).

- [ ] **Step 1: Implement Navbar component**
- [ ] **Step 2: Update app.tsx with all routes and Navbar**
- [ ] **Step 3: Verify visually in browser**
- [ ] **Step 4: Commit**

### Task 7.2: Library Screen

**Files:**
- Create: `frontend/src/pages/Library.tsx`
- Create: `frontend/src/components/FilterBar.tsx`
- Create: `frontend/src/components/TitleCard.tsx`
- Create: `frontend/src/components/PosterCard.tsx`
- Create: `frontend/src/hooks/useApi.ts`

Reference: `docs/mockups/library-v2.html`

**"All" tab layout**: Sections ordered top-to-bottom: Completed → Plan to watch → Dropped (poster grids, 3 columns) → Watching → Up to date (horizontal list cards with progress bar + episode badge).

**FilterBar**: Secondary bar above navbar. Tabs: All, Watching, Up to date, Completed, Dropped, Plan. Active = `accent-blue` with full-height highlight.

**TitleCard** (watching/up-to-date): Cover thumb 48×68 + title + metadata + progress bar (amber on `#2A2A2A`) + circular episode badge (34px amber circle with episode number).

**PosterCard** (completed/dropped/plan): Cover fills cell, aspect-ratio 2:3, 8px radius, bottom gradient overlay with title name.

**Quick mark**: Tapping episode badge → `POST /api/titles/:id/episodes/:id` toggle → update badge to next episode → if finale → show rating prompt.

**useApi hook**: Simple fetch wrapper using `apiFetch`. Returns `{data, error, loading, mutate}`.

- [ ] **Step 1: Create useApi hook**
- [ ] **Step 2: Create FilterBar component**
- [ ] **Step 3: Create PosterCard component**
- [ ] **Step 4: Create TitleCard component with quick mark**
- [ ] **Step 5: Create Library page assembling all components**
- [ ] **Step 6: Verify visually against mockup**
- [ ] **Step 7: Commit**

### Task 7.3: Title Detail Screen

**Files:**
- Create: `frontend/src/pages/TitleDetail.tsx`
- Create: `frontend/src/components/ActionBar.tsx`
- Create: `frontend/src/components/SeasonTab.tsx`
- Create: `frontend/src/components/EpisodeRow.tsx`

Reference: `docs/mockups/title-detail-v6.html`

**Hero cover**: 160px gradient fade. Back button + edit button + AniList badge (anime only) + title name + metadata.

**Progress bar**: Below hero. Amber on dark track + text "S2 · 7 of 10 episodes watched".

**Season tabs**: Horizontal pills. Completed = green check + rating. In-progress = amber progress + fraction. Unwatched = default surface.

**Episode list**: Rows with episode number + name, right-aligned checkmark (amber filled = watched, bordered square = unwatched). Tap to toggle.

**Action bar**: Above main navbar. Items: S02E06 (next unwatched, coral highlight), IMDb (link), AniList (sync), Rate.

- [ ] **Step 1: Create SeasonTab component**
- [ ] **Step 2: Create EpisodeRow component**
- [ ] **Step 3: Create ActionBar component**
- [ ] **Step 4: Create TitleDetail page**
- [ ] **Step 5: Verify visually against mockup**
- [ ] **Step 6: Commit**

### Task 7.4: Bottom Sheets (Rating, Edit, AniList)

**Files:**
- Create: `frontend/src/components/BottomSheet.tsx`
- Create: `frontend/src/components/RatingPrompt.tsx`
- Create: `frontend/src/components/EditSheet.tsx`
- Create: `frontend/src/components/AniListSheet.tsx`

Reference: `docs/mockups/rating-prompt-v3.html`, `docs/mockups/edit-sheet-v2.html`, `docs/mockups/anilist-sheet-v2.html`

**BottomSheet**: Reusable wrapper. 4px drag handle, `bg-card` background, 16px top radius, backdrop at 40% opacity, slide-up animation.

**RatingPrompt**: 10 stars horizontal row (amber filled, `#333` empty), generous spacing for mobile. Large rating display "8/10". Buttons: Save / Save & IMDb / Save & AniList / Skip.

**EditSheet**: Type selector (Movie/Series/Anime), status selector (Watching/Completed/Dropped/Plan), display name radio list (multilingual names). Changing to completed/dropped → triggers rating prompt after save.

**AniListSheet**: Match card (cover + romaji + english + year + link). Confidence box. Actions: Confirm & Sync / Wrong match. States: not connected, pending, confirmed.

- [ ] **Step 1: Create BottomSheet base component**
- [ ] **Step 2: Create RatingPrompt**
- [ ] **Step 3: Create EditSheet**
- [ ] **Step 4: Create AniListSheet**
- [ ] **Step 5: Wire trigger logic (episode toggle, action bar, edit button, AniList badge)**
- [ ] **Step 6: Commit**

### Task 7.5: Search Screen

**Files:**
- Create: `frontend/src/pages/Search.tsx`
- Create: `frontend/src/components/StatusBadge.tsx`

Reference: `docs/mockups/search-screen-v2.html`

Search input at bottom (thumb zone), auto-focuses on tab switch. Results as compact rows: cover thumb (42×60) + title + status badge (colored pill) + metadata + chevron. Tapping → Title Detail.

- [ ] **Step 1-4: Implement + verify + commit**

### Task 7.6: Add & Validate Screens

**Files:**
- Create: `frontend/src/pages/Add.tsx`
- Create: `frontend/src/pages/Validate.tsx`

Reference: `docs/mockups/add-screen-v2.html`, `docs/mockups/title-validation-v2.html`

**Add**: Input at bottom. Empty state with plus icon + brand hints. URL detection (IMDB/TVDB/AniList). On submit → `/validate?q=...`.

**Validate**: Shared screen for add/share target/match fix. Loading spinner during matching. Cover + title + year + type + resolved IDs as clickable chips. States: new title (status picker + Add button), already in library (status badge + view link), match fix (confirm + manual ID fields).

- [ ] **Step 1-4: Implement + verify + commit**

### Task 7.7: Match Review Screen

**Files:**
- Create: `frontend/src/pages/MatchReview.tsx`
- Create: `frontend/src/components/MatchReviewCard.tsx`

Reference: `docs/mockups/match-review-v2.html`

**Match Review Banner**: On Library screen, inline alert card (red background + count badge). Only visible when pending/unconfirmed titles exist.

**Match Review Screen**: Header with count badge. Batch confirm button. Sections: unconfirmed (red) then pending (amber). Cards with cover + Plex title + confidence box + external ID chips + Confirm/Fix actions.

- [ ] **Step 1-4: Implement + verify + commit**

### Task 7.8: Login Screen

**Files:**
- Create: `frontend/src/pages/Login.tsx`

Google Sign-In button. On success → `POST /api/auth/google` with credential → cookie set → redirect to Library.

Use Google's GSI JavaScript library (`accounts.google.com/gsi/client`).

- [ ] **Step 1-3: Implement + verify + commit**

---

## Phase 8: Push Notifications & Background Job

### Task 8.1: Web Push Service

**Files:**
- Create: `internal/service/push.go`
- Create: `internal/service/push_test.go`
- Create: `internal/handler/push.go`
- Modify: `internal/router/router.go`

Go side: VAPID push using `github.com/SherClockHolmes/webpush-go`.

Endpoints:
- `POST /api/push/subscribe` — stores push subscription JSON in settings
- `DELETE /api/push/subscribe` — removes subscription

Service: `SendNotification(title, body, url)` — sends to stored subscription.

- [ ] **Step 1-5: TDD cycle → commit**

### Task 8.2: Service Worker & Frontend Push

**Files:**
- Create: `frontend/public/sw.js`
- Create: `frontend/src/hooks/usePush.ts`
- Modify: `frontend/public/manifest.json`

Service worker: listen for push events, show notification with title/body/icon, handle notification click (open URL).

`usePush` hook: request notification permission, subscribe with VAPID public key, send subscription to backend.

PWA manifest with `share_target` for Android share sheet.

- [ ] **Step 1: Create manifest.json with share_target**
- [ ] **Step 2: Create service worker**
- [ ] **Step 3: Create usePush hook**
- [ ] **Step 4: Wire subscription on login**
- [ ] **Step 5: Commit**

### Task 8.3: Background Title Refresh Job

**Files:**
- Create: `internal/service/background.go`
- Create: `internal/service/background_test.go`
- Modify: `cmd/serve.go`

Daily job running inside the Go process (simple `time.Ticker` + goroutine).

Logic (from spec):
1. Iterate non-completed titles + titles with missing data
2. Fetch series status from TMDB/AniList
3. Status changed → push notification
4. Ended + all watched → mark completed
5. Fetch new episodes not yet in DB
6. Fetch/update cover if missing
7. Fetch multilingual names if missing
8. Refresh anime-offline-database JSON

Rate limiting: sequential processing with delays, respect per-API limits.

- [ ] **Step 1: Write test for refresh logic (mock external APIs)**
- [ ] **Step 2: Implement**
- [ ] **Step 3: Wire into serve.go (start goroutine)**
- [ ] **Step 4: Commit**

### Task 8.4: Push Notification Triggers

**Files:**
- Modify: `internal/handler/webhook.go`
- Modify: `internal/handler/episode.go`
- Modify: `internal/service/background.go`

Wire push notifications for:
- Movie or season finale marked as watched (auto via Plex or manual) → "Rate [Title]!"
- Series status change detected by background job → "[Title] has ended"

- [ ] **Step 1-3: Implement triggers + commit**

---

## Phase 9: AniList OAuth & Sync

### Task 9.1: AniList OAuth Flow

**Files:**
- Create: `internal/handler/anilist_auth.go`
- Modify: `internal/router/router.go`

Endpoints:
- `GET /api/anilist/auth` → redirect to AniList OAuth page (implicit grant)
- `GET /api/anilist/callback` → extract token from URL fragment (client-side), store in settings

- [ ] **Step 1-4: Implement + commit**

### Task 9.2: Settings API

**Files:**
- Create: `internal/handler/settings.go`
- Modify: `internal/router/router.go`

`GET /api/settings` — returns AniList connected status, push subscription status.

- [ ] **Step 1-3: Implement + commit**

---

## Phase 10: Production Docker & CI

### Task 10.1: Production Dockerfile

**Files:**
- Create: `Dockerfile`
- Create: `docker-compose.yml`

Multi-stage build:
1. Stage 1 (frontend): Node image → `npm ci && npx vite build`
2. Stage 2 (backend): Go image → copy frontend dist → `go build` with embedded SPA
3. Stage 3 (runtime): Minimal image (debian-slim or distroless) → copy binary + `/data` volume

```dockerfile
# Frontend build
FROM node:22-bookworm AS frontend
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ .
RUN npx vite build

# Backend build
FROM golang:1.24-bookworm AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/frontend/dist ./frontend/dist
RUN CGO_ENABLED=1 go build -o plextracker .

# Runtime
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=backend /app/plextracker /usr/local/bin/plextracker
VOLUME /data
EXPOSE 8080
CMD ["plextracker", "serve"]
```

Production `docker-compose.yml` matches the spec's deployment section.

- [ ] **Step 1: Create Dockerfile**
- [ ] **Step 2: Create docker-compose.yml**
- [ ] **Step 3: Build and test locally**

```bash
docker build -t plextracker:latest .
docker run --rm -e GOOGLE_CLIENT_ID=test -e GOOGLE_ALLOWED_EMAIL=test@test.com -e JWT_SECRET=test -v $(pwd)/data:/data -p 8080:8080 plextracker:latest
```

- [ ] **Step 4: Commit**

### Task 10.2: GitHub Actions CI

**Files:**
- Create: `.github/workflows/ci.yml`
- Create: `.github/workflows/release.yml`

CI: on push/PR → run Go tests + frontend build + lint.
Release: on tag push → build Docker image → push to `ghcr.io/nicolasvasse/plextracker:latest` and `:vX.Y.Z`.

- [ ] **Step 1: Create CI workflow**
- [ ] **Step 2: Create release workflow**
- [ ] **Step 3: Commit**

---

## Phase 11: Stats Screen (TBD)

Placeholder — to be designed in a dedicated session per the spec.

---

## Dependency Map

```
Phase 1 (Scaffold) ─────────────────────────────────┐
    │                                                 │
Phase 2 (Auth) ──────────────────────────┐            │
    │                                     │            │
Phase 3 (CRUD API) ──────────────────────┤            │
    │                    │                │            │
Phase 4 (Import)    Phase 5 (Webhook)    │     Phase 7 (Frontend)
                         │                │            │
                    Phase 6 (Matching)    │            │
                                          │            │
                              Phase 8 (Push + Background)
                                          │
                              Phase 9 (AniList OAuth)
                                          │
                              Phase 10 (Docker + CI)
```

Phases 4 and 5 can run in parallel. Phase 7 (frontend) can start after Phase 3 is done. Phase 6 extends Phase 5. Phase 8 depends on Phase 7 (frontend push) and Phase 5 (webhook triggers).
