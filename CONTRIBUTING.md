# Contributing to Trackarr

Thank you for your interest in contributing to Trackarr! We welcome contributions, bug reports, and discussions.

---

## 💡 Project Scope & Philosophy

Trackarr is primarily developed as a personal homelab media tracker and watchlist manager. It focuses on a clean, single-user experience with a deliberate set of integrations (Plex, Jellyfin, AniList, Radarr, Sonarr, Prowlarr).

While the project is open-source and community contributions are very welcome, new features are prioritized around keeping the codebase fast, minimal, and aligned with this core vision. Before proposing large architectural changes or new service integrations, please open an issue first to discuss the idea.

---

## 🛠️ Development Workflow

All development commands run inside Docker to ensure environment parity. **Never run `go`, `npm`, or `vite` directly on the host machine.**

### Prerequisites
- Docker & Docker Compose
- `make`
- `git`

### Quickstart
1. Fork the repository and clone your fork:
   ```bash
   git clone https://github.com/YOUR_USERNAME/trackarr.git
   cd trackarr
   ```
2. Start the development environment:
   ```bash
   make up
   ```
3. In a separate terminal, start the frontend development server:
   ```bash
   make dev-frontend
   ```
4. Access the web app at `http://localhost:8080`.

---

## 🧪 Testing & Linting

Before opening a pull request, ensure all validation checks pass:

```bash
make test          # Run all Go backend unit tests
make test-front    # Run Vitest frontend tests + production build check
make lint          # Run Go linter (golangci-lint)
make lint-front    # Run TypeScript strict type-check
```

---

## 📐 Architecture & Standards

- **Backend (Go 1.24)**:
  - Clean architecture with dependency injection in handlers (`internal/handler`).
  - Database queries belong strictly in `internal/repository`.
  - Errors must be wrapped with contextual information: `fmt.Errorf("context: %w", err)`.
  - No magic strings: use domain constants or enums.
- **Frontend (Preact 10 + TypeScript)**:
  - Functional components with hooks.
  - Strict TypeScript mode.
  - HSL/hex CSS tokens defined in `frontend/src/tokens.css`.
  - Android-first PWA responsiveness and performance.

---

## 🔀 Submitting Pull Requests

1. Create a feature branch: `git checkout -b feat/my-new-feature`
2. Commit your changes with clear, descriptive commit messages.
3. Push to your fork and submit a Pull Request to `main`.
4. Continuous Integration (CI) will automatically validate the frontend build, backend tests, and linters.

---

## 📜 Code of Conduct

Please note that this project is released with a [Contributor Code of Conduct](CODE_OF_CONDUCT.md). By participating in this project you agree to abide by its terms.
