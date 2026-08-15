---
name: debug-prod
description: "Debug production issues (Radarr/Sonarr queue, sync errors, scrobble failures, data discrepancies) by pulling production DB and logs locally. Never inspect logs via SSH on prod."
---

# Production Debugging

Mandatory protocol for diagnosing and debugging issues that occur in production.

## Golden Rule
**Always pull files (DB, logs) locally first instead of inspecting logs or querying DB directly on prod via SSH.**
All application debugging, log grepping, and database querying must happen **locally**. Direct SSH is reserved exclusively for non-pullable host/system diagnostics (e.g. disk space, network reachability, docker daemon status) if strictly required.

## Protocol

### 1. Pull Production State
Run:
```bash
make ssh-debug-pull
```
*(Runs `make ssh-db-pull` to download `plextracker.db` + WAL/SHM and start the local app, followed by `make ssh-logs` to dump remote logs into `data/plextracker.log`)*

If you only need fresh logs without resetting the local DB:
```bash
make ssh-logs
```
*(Or with limit: `make ssh-logs LINES=1000`)*

### 2. Inspect Downloaded Logs Locally
Inspect `data/plextracker.log` using ripgrep or file viewing:
- Search for the specific domain (e.g. `radarr`, `sonarr`, `queue`, `scrobble`, `gemini`, `tmdb`):
  `data/plextracker.log`
- Identify timestamps, error traces, and payload IDs.

### 3. Query Downloaded Database Locally
Query the local SQLite database at `data/plextracker.db`:
- Inspect records in `titles`, `watch_events`, `task_queue`, `match_events`, etc.
- Verify status, foreign keys, or missing attributes.

### 4. Reproduce & Validate
- Replicate the scenario locally against the pulled dataset at `http://localhost:8080`.
- Write unit/integration tests reproducing the bug (`make test` / `make test-front`).
- Implement the fix and verify.
