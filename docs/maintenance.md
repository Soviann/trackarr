# Maintenance, CI/CD & Operations

[← Back to Index](INDEX.md)

This document outlines ongoing maintenance, automated dependency updates, CI/CD pipelines, and local database operations.

---

## 1. Automated Dependency Management (Dependabot)

Trackarr uses GitHub Dependabot to keep Go modules, npm packages, Docker bases, and GitHub Actions up to date:
- **Weekly Version Updates**: Routine minor/patch updates are grouped weekly into consolidated PRs (`applies-to: version-updates`).
- **Immediate Security Alerts**: Security vulnerabilities (CVE/GHSA) immediately trigger dedicated standalone Pull Requests.

---

## 2. CI/CD GitHub Actions Workflows

- **Continuous Integration (`.github/workflows/ci.yml`)**:
  - Backend Go unit tests (`make test`) with in-memory SQLite and FTS5.
  - Backend linting (`golangci-lint`).
  - Frontend unit tests and production Vite compilation (`make test-front`).
  - Frontend type checks (`tsc`).
- **Multi-Arch GHCR Release (`.github/workflows/release.yml`)**:
  - Builds and publishes `linux/amd64` and `linux/arm64` Docker images to GitHub Container Registry on every Git tag push (`v*`).
- **Dependabot Auto-Merge (`.github/workflows/dependabot-auto-merge.yml`)**:
  - Automatically merges green Dependabot PRs using squash merge.

---

## 3. Database Backup & Safe Restore

### Online SQLite WAL Backup:
SQLite in WAL mode supports online backups without shutting down Trackarr:
```bash
# Inside container or host with sqlite3 CLI
sqlite3 /data/trackarr.db ".backup '/data/trackarr-backup.db'"
gzip /data/trackarr-backup.db
```

### Restoring a Backup:
```bash
# 1. Stop Trackarr container
docker compose down

# 2. Clean temporary WAL/SHM files
rm -f ./data/trackarr.db-wal ./data/trackarr.db-shm

# 3. Restore database file
gunzip -c /path/to/backup.db.gz > ./data/trackarr.db

# 4. Start Trackarr
docker compose up -d
```

---

## 4. Built-in CLI Maintenance Commands

The `trackarr` binary includes emergency maintenance subcommands (database migrations run automatically on startup):
```bash
# Emergency Password Reset
docker exec -t trackarr trackarr reset-password --password="MyNewPassword"

# Backfill Poster Accent Colors
docker exec -t trackarr trackarr backfill-accents --force

# Verify Binary Version & Build Metadata
docker exec -t trackarr trackarr version

# Historical Backup Import (Simkl)
docker exec -t trackarr trackarr import --dry-run /path/to/backup.zip
```
