# Native Go Model Context Protocol (MCP) Server Guide

[← Back to Master Index](../INDEX.md) • [← Back to Codebase Map](../patterns.md)

---

## 🎯 Overview & Architecture

The **Trackarr MCP Server** (`cmd/trackarr-mcp`) is a native Go implementation of the official [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) specification, built using `github.com/mark3labs/mcp-go`.

It allows AI assistants (such as Antigravity, Claude Desktop, and Cursor) and automated agents to securely explore and manage your Trackarr media library over standard input/output (`stdio`) JSON-RPC.

```
┌─────────────────────────────────┐
│ AI Client (Claude / Antigravity)│
└────────────────┬────────────────┘
                 │ stdio (JSON-RPC)
┌────────────────▼────────────────┐
│      trackarr-mcp binary        │
│  (Token compaction & validation)│
└────────────────┬────────────────┘
                 │ HTTP (REST + Bearer API Key)
┌────────────────▼────────────────┐
│      Trackarr Web Service       │
│    (UnifiedAuth + Scopes DB)    │
└─────────────────────────────────┘
```

### Key Highlights
- **Zero Node.js Runtime**: Single compiled Go binary with a tiny memory footprint (~12–15 MB RAM).
- **Stdio Transport Hygiene**: `log.SetOutput(os.Stderr)` is strictly enforced on startup. Diagnostic and informational logs are directed to `stderr`, guaranteeing clean `stdout` framing for JSON-RPC messages.
- **Client-Side Token Compaction**: Tool outputs are pruned and formatted to minimize LLM context usage while preserving critical details (IDs, names, watch progress, external links).
- **Self-Correcting Error Reporting**: API and validation failures return `CallToolResult{IsError: true}` inside the protocol payload rather than terminating the RPC connection, enabling the LLM to inspect errors and self-correct.
- **Scoped API Key Protection**: Authenticates against Trackarr using multi-key API tokens (`trck_live_...`), honoring granular scopes (`library:read`, `library:write`, `library:delete`, `arr:write`).

---

## 🚀 Building & Running

### Build Binary
Compile the binary using the Docker-first development target:
```bash
make build-mcp
```
The compiled executable will be placed in `./tmp/trackarr-mcp`.

### Command Line Flags & Environment Variables

| Flag | Env Var | Default | Description |
|---|---|---|---|
| `-url` | `TRACKARR_URL` | `http://127.0.0.1:8080` | Base URL of the Trackarr server. |
| `-api-key` | `TRACKARR_API_KEY` | *(Required)* | Secret API key token (`trck_live_...`). |
| `-timeout` | `TRACKARR_TIMEOUT` | `30` | HTTP request timeout in seconds. |
| `-version` / `-v` | — | `false` | Print version information to stderr and exit. |

---

## 🔌 Client Configuration

### 1. Antigravity IDE (`~/.gemini/antigravity/mcp/trackarr.json`)
```json
{
  "name": "trackarr",
  "command": "/path/to/plextracker/tmp/trackarr-mcp",
  "env": {
    "TRACKARR_URL": "http://127.0.0.1:8080",
    "TRACKARR_API_KEY": "trck_live_YOUR_64_HEX_CHAR_API_KEY"
  }
}
```

### 2. Claude Desktop (`claude_desktop_config.json`)
```json
{
  "mcpServers": {
    "trackarr": {
      "command": "/path/to/plextracker/tmp/trackarr-mcp",
      "env": {
        "TRACKARR_URL": "http://127.0.0.1:8080",
        "TRACKARR_API_KEY": "trck_live_YOUR_64_HEX_CHAR_API_KEY"
      }
    }
  }
}
```

### 3. Cursor (`.cursor/mcp.json`)
```json
{
  "mcpServers": {
    "trackarr": {
      "command": "/path/to/plextracker/tmp/trackarr-mcp",
      "args": [
        "-url", "http://127.0.0.1:8080",
        "-api-key", "trck_live_YOUR_64_HEX_CHAR_API_KEY"
      ]
    }
  }
}
```

---

## 🛠️ MCP Tools Reference

### 1. Search & Discovery

#### `trackarr_search`
Search movies and series in Trackarr by title, keyword, or external URL.
- **Parameters**:
  - `query` (string, optional): Search keyword, title name, or external URL.
  - `type` (string, optional): `"movie"` or `"series"`.
  - `status` (string, optional): `"plan_to_watch"`, `"watching"`, `"completed"`, `"paused"`, `"dropped"`.
  - `limit` (integer, optional, default: 20, max: 100): Results limit.
- **Scope**: `library:read`.

#### `trackarr_get_title`
Fetch complete details for a title by its Trackarr ID, including aliases, overview, genres, seasons, and episodes.
- **Parameters**:
  - `id` (integer, required): Title ID.
- **Scope**: `library:read`.

#### `trackarr_get_continue_watching`
List in-progress series with their next unwatched episode.
- **Parameters**: None.
- **Scope**: `library:read`.

#### `trackarr_resolve_url`
Resolve an external URL (IMDb, TMDb, TVDB, AniList) to check if the title exists in the local library, or extract metadata via the online enrichment pipeline.
- **Parameters**:
  - `url` (string, required): Full URL (e.g. `https://www.imdb.com/title/tt0137523/`).
- **Scope**: `library:read`.

#### `trackarr_get_stats`
Retrieve library statistics (movie count, series count, total watch minutes, episodes watched).
- **Parameters**: None.
- **Scope**: `library:read`.

---

### 2. Library Modifications

#### `trackarr_add_title`
Add a single movie or TV series to Trackarr. Automatically deduplicates against existing titles and enqueues background metadata enrichment tasks.
- **Parameters**:
  - `title` (string, optional): Title name.
  - `type` (string, optional): `"movie"` or `"series"`.
  - `year` (integer, optional): Release year.
  - `url` (string, optional): External URL for auto-extraction.
  - `tmdb_id` (integer, optional): TMDb ID.
  - `tvdb_id` (integer, optional): TVDB ID.
  - `imdb_id` (string, optional): IMDb ID.
  - `anilist_id` (integer, optional): AniList ID.
  - `is_anime` (boolean, optional): Whether title is anime.
  - `status` (string, optional): Default `"plan_to_watch"`.
- **Scope**: `library:write`.

#### `trackarr_batch_add`
Ingest up to 100 titles in a single atomic transaction.
- **Parameters**:
  - `items` (array of objects, required): Up to 100 title entries with the fields described in `trackarr_add_title`.
- **Scope**: `library:write`.

#### `trackarr_update_title`
Update properties of an existing title (watch status, rating 0–10, notes, anime flag).
- **Parameters**:
  - `id` (integer, required): Title ID.
  - `status` (string, optional): Watch status.
  - `my_rating` (integer, optional): Rating from 0 to 10.
  - `personal_notes` (string, optional): User review / notes.
  - `is_anime` (boolean, optional): Anime flag.
  - `arr_ignored` (boolean, optional): Sonarr/Radarr sync exclusion.
- **Scope**: `library:write`.

#### `trackarr_set_episode_watched`
Set or toggle the watched state of a TV series episode idempotently.
- **Parameters**:
  - `title_id` (integer, required): Series ID.
  - `episode_id` (integer, required): Episode ID.
  - `watched` (boolean, required): `true` to mark watched, `false` to mark unwatched.
- **Scope**: `library:write`.

---

### 3. Deletion & External Integrations

#### `trackarr_delete_title`
Delete a title and its associated seasons, episodes, and watch history from Trackarr. Marked as destructive.
- **Parameters**:
  - `id` (integer, required): Title ID.
- **Scope**: `library:delete`.

#### `trackarr_push_to_arr`
Push a movie or series to Radarr or Sonarr for automated downloading.
- **Parameters**:
  - `id` (integer, required): Title ID.
- **Scope**: `arr:write`.

---

## 📦 MCP Resources Reference

Trackarr exposes live read-only MCP resources formatted as JSON:

| URI | Name | Description |
|---|---|---|
| `trackarr://library/summary` | Trackarr Library Summary | Live library stats, media counts, and watch time. |
| `trackarr://continue-watching` | Continue Watching | Active in-progress series and next episodes to watch. |
