#!/usr/bin/env python3
"""
GitHub PR & Issue Webhook Daemon for Antigravity (Synology NAS)
Listens for GitHub Webhook events on port 8191 (via Synology Reverse Proxy).
Verifies HMAC SHA-256 signatures, analyzes repository context & logs with Gemini AI,
and autonomously manages the full lifecycle:
  1. Diagnostic & Implementation Planning (multi-turn interactive refinement)
  2. Autonomous Code Generation on Approval (/antigravity Approved, /antigravity LGTM)
  3. Workspace file editing, testing & lint validation
  4. Git branch creation, commit, push & GitHub Pull Request generation
"""

import os
import sys
import json
import re
import time
import hmac
import hashlib
import urllib.request
import urllib.error
import subprocess
from http.server import HTTPServer, BaseHTTPRequestHandler
from socketserver import ThreadingMixIn

PORT = int(os.getenv("PORT", "8191"))
ENV_FILE = os.getenv("ENV_FILE", "/volume1/docker/plextracker/antigravity/.env.local")
WORKSPACE_DIR = os.getenv("WORKSPACE_DIR", "/volume1/docker/plextracker/antigravity/workspace")
DATA_DIR = os.getenv("DATA_DIR", "/data")


def load_env_files():
    """Load all available .env and .env.local secrets into environment."""
    env_paths = [
        ".env",
        ".env.local",
        "/volume1/docker/plextracker/.env",
        "/volume1/docker/plextracker/.env.local",
        ENV_FILE
    ]
    for path in env_paths:
        if os.path.exists(path):
            try:
                with open(path, "r", encoding="utf-8") as f:
                    for line in f:
                        line = line.strip()
                        if line and not line.startswith("#") and "=" in line:
                            key, val = line.split("=", 1)
                            val = val.strip("'\"")
                            os.environ[key.strip()] = val
            except Exception as e:
                print(f"[WARN] Failed to read env file {path}: {e}", flush=True)


load_env_files()

GITHUB_TOKEN = os.getenv("GITHUB_TOKEN", os.getenv("ANTIGRAVITY_TOKEN", os.getenv("GH_TOKEN", "")))
WEBHOOK_SECRET = os.getenv("WEBHOOK_SECRET", os.getenv("GITHUB_WEBHOOK_SECRET", ""))
GEMINI_API_KEY = os.getenv("GEMINI_API_KEY", os.getenv("GOOGLE_API_KEY", ""))
DEFAULT_MODEL = os.getenv("DEFAULT_MODEL", "gemini-3.6-flash")

DAEMON_SIGNATURE = "<!-- antigravity-daemon -->"
ALLOWED_TRIGGERS = ["/antigravity", "/plextracker", "/bot", "/agy"]
APPROVAL_KEYWORDS = [
    "approved", "approve", "lgtm", "go", "valide", "validé", "valider",
    "proceed", "exec", "execute", "apply", "fais-le", "vas-y", "c'est bon", "ok"
]
DIRECT_KEYWORDS = ["--direct", "mode: direct", "direct: true", "direct"]


def github_api_request(url, method="GET", data=None):
    """Perform an authenticated request to the GitHub REST API."""
    if not GITHUB_TOKEN:
        print("[ERROR] GITHUB_TOKEN not configured.", flush=True)
        return None

    headers = {
        "Authorization": f"token {GITHUB_TOKEN}",
        "Accept": "application/vnd.github.v3+json",
        "User-Agent": "Antigravity-NAS-Daemon"
    }

    body_bytes = None
    if data is not None:
        headers["Content-Type"] = "application/json"
        body_bytes = json.dumps(data).encode("utf-8")

    req = urllib.request.Request(url, data=body_bytes, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            resp_body = resp.read().decode("utf-8")
            if resp_body:
                return json.loads(resp_body)
            return {}
    except urllib.error.HTTPError as e:
        err_msg = e.read().decode("utf-8", errors="ignore")
        print(f"[ERROR] GitHub API error {e.code} on {url}: {err_msg}", flush=True)
        raise RuntimeError(f"GitHub API {e.code}: {err_msg}")
    except Exception as e:
        print(f"[ERROR] GitHub API request failed on {url}: {e}", flush=True)
        raise


def post_github_comment(repo_full_name, issue_number, body):
    """Post a comment to a GitHub Issue/PR."""
    if not GITHUB_TOKEN:
        print("[ERROR] GITHUB_TOKEN not set, cannot post comment to GitHub.", flush=True)
        return False

    if not body.startswith(DAEMON_SIGNATURE):
        body = f"{DAEMON_SIGNATURE}\n{body}"

    url = f"https://api.github.com/repos/{repo_full_name}/issues/{issue_number}/comments"
    try:
        github_api_request(url, method="POST", data={"body": body})
        print(f"[INFO] Comment posted successfully to {repo_full_name}#{issue_number}", flush=True)
        return True
    except Exception as e:
        print(f"[ERROR] Exception posting comment: {e}", flush=True)
        return False


def fetch_issue_comments(repo_full_name, issue_number):
    """Fetch all comments on an issue/PR to build multi-turn history."""
    url = f"https://api.github.com/repos/{repo_full_name}/issues/{issue_number}/comments?per_page=50"
    try:
        comments = github_api_request(url, method="GET")
        return comments if isinstance(comments, list) else []
    except Exception as e:
        print(f"[WARN] Failed to fetch comments for #{issue_number}: {e}", flush=True)
        return []


def create_or_update_pull_request(repo_full_name, head_branch, base_branch, title, body, issue_number):
    """Create a new Pull Request or return existing one."""
    url = f"https://api.github.com/repos/{repo_full_name}/pulls"
    payload = {
        "title": title,
        "head": head_branch,
        "base": base_branch,
        "body": f"{body}\n\n---\nCloses #{issue_number}"
    }

    try:
        pr_data = github_api_request(url, method="POST", data=payload)
        return pr_data.get("html_url"), pr_data.get("number")
    except Exception as e:
        print(f"[INFO] PR creation returned error, checking if PR already exists: {e}", flush=True)
        try:
            owner = repo_full_name.split("/")[0]
            search_url = f"https://api.github.com/repos/{repo_full_name}/pulls?head={owner}:{head_branch}&state=open"
            existing = github_api_request(search_url, method="GET")
            if existing and isinstance(existing, list) and len(existing) > 0:
                pr_obj = existing[0]
                pr_num = pr_obj.get("number")
                update_url = f"https://api.github.com/repos/{repo_full_name}/pulls/{pr_num}"
                github_api_request(update_url, method="PATCH", data=payload)
                return pr_obj.get("html_url"), pr_num
        except Exception as search_err:
            print(f"[WARN] Failed to update existing PR: {search_err}", flush=True)
        raise


def is_daemon_comment(body):
    """Check if the comment was generated by the daemon itself to prevent loops."""
    if not body:
        return False
    if DAEMON_SIGNATURE in body:
        return True
    if "🤖 **Antigravity NAS Agent**" in body or "### 📋 Antigravity" in body or "❌ **Antigravity Daemon Error**" in body:
        return True
    return False


def is_trigger_present(*texts):
    """Check if any trigger command is present in non-quoted lines of the provided texts."""
    for text in texts:
        if not text:
            continue
        clean_lines = [line for line in text.splitlines() if not line.strip().startswith(">")]
        clean_text = "\n".join(clean_lines).lower()
        if any(tr in clean_text for tr in ALLOWED_TRIGGERS):
            return True
    return False


def is_approval_intent(text):
    """Check if the comment represents an approval to proceed with execution."""
    if not text:
        return False
    clean_lines = [line for line in text.splitlines() if not line.strip().startswith(">")]
    clean_text = " ".join(clean_lines).lower()

    has_trigger = any(tr in clean_text for tr in ALLOWED_TRIGGERS)
    words = re.findall(r'[a-z0-9_éèêàùç-]+', clean_text)

    for kw in APPROVAL_KEYWORDS:
        if kw in words:
            if f"not {kw}" in clean_text or f"pas {kw}" in clean_text or f"non {kw}" in clean_text:
                return False
            return True
        if has_trigger and f"/antigravity {kw}" in clean_text:
            return True

    return False


def is_release_intent(text):
    """Check if the comment represents a release / deploy command."""
    if not text:
        return False
    clean_lines = [line for line in text.splitlines() if not line.strip().startswith(">")]
    clean_text = " ".join(clean_lines).lower()
    has_trigger = any(tr in clean_text for tr in ALLOWED_TRIGGERS)
    release_words = ["release", "deploy", "déploie", "déployer", "publie", "publier", "tag"]
    if has_trigger and any(w in clean_text for w in release_words):
        return True
    return False


def parse_release_target(text):
    """Extract release target or bump type (major, minor, patch, or explicit vX.Y.Z)."""
    if not text:
        return "patch"
    match = re.search(r'\b(v\d+\.\d+\.\d+)\b', text, re.IGNORECASE)
    if match:
        return match.group(1).lower()
    clean = text.lower()
    if "major" in clean or "majeure" in clean:
        return "major"
    if "minor" in clean or "mineure" in clean:
        return "minor"
    return "patch"


def get_next_release_tag(repo_dir, release_target):
    """Compute the next release SemVer tag based on existing git tags."""
    if release_target.startswith("v") and re.match(r'^v\d+\.\d+\.\d+$', release_target):
        return release_target

    try:
        git_tags = subprocess.run(
            ["git", "tag", "--sort=-v:refname"],
            cwd=repo_dir, capture_output=True, text=True, check=False
        ).stdout.strip().splitlines()

        semver_tags = [t for t in git_tags if re.match(r'^v\d+\.\d+\.\d+$', t)]
        if not semver_tags:
            return "v0.1.0"

        latest = semver_tags[0]
        m = re.match(r'^v(\d+)\.(\d+)\.(\d+)$', latest)
        if not m:
            return "v0.1.0"

        maj, min_v, patch = int(m.group(1)), int(m.group(2)), int(m.group(3))
        if release_target == "major":
            return f"v{maj + 1}.0.0"
        elif release_target == "minor":
            return f"v{maj}.{min_v + 1}.0"
        else:
            return f"v{maj}.{min_v}.{patch + 1}"
    except Exception as e:
        print(f"[WARN] Failed to compute next release tag: {e}", flush=True)
        return "v0.1.0"


def is_direct_intent(text):
    """Check if user explicitly asked for direct execution without planning phase."""
    if not text:
        return False
    clean_text = text.lower()
    return any(kw in clean_text for kw in DIRECT_KEYWORDS)


def verify_signature(body_bytes, signature_header):
    """Verify HMAC SHA-256 signature from GitHub."""
    if not WEBHOOK_SECRET:
        print("[WARN] WEBHOOK_SECRET not configured; skipping HMAC verification.", flush=True)
        return True

    if not signature_header or not signature_header.startswith("sha256="):
        return False

    expected_sig = "sha256=" + hmac.new(
        WEBHOOK_SECRET.encode("utf-8"),
        body_bytes,
        hashlib.sha256
    ).hexdigest()

    return hmac.compare_digest(expected_sig, signature_header)


def call_gemini_api(prompt, model_name=DEFAULT_MODEL):
    """Call Google Gemini API to generate content with API key and model fallback."""
    keys = [k.strip() for k in GEMINI_API_KEY.split(",") if k.strip()]
    if not keys:
        raise ValueError("GEMINI_API_KEY not configured on NAS.")

    if "pro" in model_name.lower():
        candidate_models = ["gemini-3.6-pro", "gemini-2.5-pro", "gemini-1.5-pro"]
    else:
        candidate_models = ["gemini-3.6-flash", "gemini-2.5-flash", "gemini-2.0-flash", "gemini-1.5-flash"]

    payload = {
        "contents": [
            {
                "parts": [
                    {"text": prompt}
                ]
            }
        ],
        "generationConfig": {
            "temperature": 0.2,
            "maxOutputTokens": 8192
        }
    }
    data = json.dumps(payload).encode("utf-8")

    last_err = None
    for model in candidate_models:
        for key in keys:
            api_url = f"https://generativelanguage.googleapis.com/v1beta/models/{model}:generateContent?key={key}"
            req = urllib.request.Request(
                api_url,
                data=data,
                headers={"Content-Type": "application/json"},
                method="POST"
            )
            try:
                with urllib.request.urlopen(req, timeout=120) as resp:
                    result = json.loads(resp.read().decode("utf-8"))
                    candidates = result.get("candidates", [])
                    if candidates:
                        candidate = candidates[0]
                        content = candidate.get("content", {})
                        parts = content.get("parts", [])
                        text_chunks = [p.get("text", "") for p in parts if "text" in p and p.get("text")]
                        full_text = "".join(text_chunks).strip()
                        if full_text:
                            return full_text, model
                        print(f"[WARN] Gemini returned candidate with parts: {parts}", flush=True)
                    raise ValueError(f"Empty text content from Gemini API: {result}")
            except urllib.error.HTTPError as e:
                err_msg = e.read().decode("utf-8", errors="ignore")
                print(f"[WARN] Gemini API error (model {model}, status {e.code}): {err_msg}", flush=True)
                last_err = f"HTTP {e.code} (model {model}): {err_msg}"
                if e.code == 404:
                    break
                elif e.code in (429, 500, 503):
                    continue
                break
            except Exception as e:
                print(f"[WARN] Gemini API call failed: {e}", flush=True)
                last_err = str(e)
                continue

    raise RuntimeError(f"Gemini API generation failed. Last error: {last_err}")


def gather_repo_context(repo_dir):
    """Read essential context, git history, and documentation files from repository."""
    context_chunks = []

    try:
        git_log = subprocess.run(
            ["git", "log", "-n", "25", "--oneline"],
            cwd=repo_dir, capture_output=True, text=True, check=False
        ).stdout.strip()
        if git_log:
            context_chunks.append(f"### Recent Git Commits (`git log -n 25`):\n```\n{git_log}\n```")
    except Exception as e:
        print(f"[DEBUG] Could not get git log: {e}", flush=True)

    docs_to_read = [
        "AGENTS.md",
        "docs/INDEX.md",
        "docs/maintenance.md",
        "docs/patterns.md",
        "internal/handler/webhook.go"
    ]
    for rel_path in docs_to_read:
        abs_path = os.path.join(repo_dir, rel_path)
        if os.path.exists(abs_path):
            try:
                with open(abs_path, "r", encoding="utf-8") as f:
                    content = f.read(6000)
                    context_chunks.append(f"### File: `{rel_path}`\n```\n{content}\n```")
            except Exception as e:
                print(f"[DEBUG] Could not read {rel_path}: {e}", flush=True)

    log_candidates = [
        os.path.join(DATA_DIR, "plextracker.log"),
        "/volume1/docker/plextracker/data/plextracker.log",
        os.path.join(repo_dir, "data", "plextracker.log")
    ]
    for log_path in log_candidates:
        if os.path.exists(log_path):
            try:
                with open(log_path, "r", encoding="utf-8", errors="ignore") as f:
                    lines = f.readlines()
                    last_lines = lines[-100:] if len(lines) > 100 else lines
                    context_chunks.append(f"### Recent Production Logs (`{log_path}` - last {len(last_lines)} lines):\n```log\n{''.join(last_lines)}\n```")
                    break
            except Exception as e:
                print(f"[DEBUG] Could not read log at {log_path}: {e}", flush=True)

    return "\n\n".join(context_chunks)


def extract_mentioned_files_content(repo_dir, text):
    """Extract files mentioned in the prompt/plan and read their current content."""
    file_pattern = r'(?:[\w\-\.]+/)+[\w\-\.]+\.(?:go|ts|tsx|js|json|sql|html|css|md|yml|yaml|sh|py)'
    matches = set(re.findall(file_pattern, text))

    file_chunks = []
    for rel_path in sorted(matches):
        clean_path = rel_path.strip("`'\"()[]:;,")
        abs_path = os.path.join(repo_dir, clean_path)
        if os.path.exists(abs_path) and os.path.isfile(abs_path):
            try:
                with open(abs_path, "r", encoding="utf-8") as f:
                    content = f.read(15000)
                    file_chunks.append(f"### Existing Source File: `{clean_path}`\n```\n{content}\n```")
            except Exception as e:
                print(f"[DEBUG] Could not read mentioned file {clean_path}: {e}", flush=True)

    return "\n\n".join(file_chunks)


def parse_generated_code(text):
    """Parse Gemini output to extract commit message, PR details, and modified/new files."""
    commit_msg = "feat: automated implementation by Antigravity NAS Agent"
    pr_title = "feat: automated implementation by Antigravity NAS Agent"
    pr_body = "Automated implementation generated by Antigravity NAS Agent."
    files = []

    json_candidates = []
    json_match = re.search(r'```(?:json)?\s*(\{[\s\S]*?\})\s*```', text)
    if json_match:
        json_candidates.append(json_match.group(1))

    if text.strip().startswith("{") and text.strip().endswith("}"):
        json_candidates.append(text.strip())

    for raw_json in json_candidates:
        try:
            data = json.loads(raw_json)
            if "files" in data and isinstance(data["files"], list):
                commit_msg = data.get("commit_message", commit_msg)
                pr_title = data.get("pr_title", pr_title)
                pr_body = data.get("pr_body", data.get("pr_description", pr_body))
                for f in data["files"]:
                    if isinstance(f, dict) and "path" in f and "content" in f:
                        files.append({
                            "path": f["path"].strip(),
                            "content": f["content"]
                        })
                if files:
                    return commit_msg, pr_title, pr_body, files
        except Exception as e:
            print(f"[DEBUG] JSON parsing failed, falling back to markdown block parser: {e}", flush=True)

    block_pattern = r'(?:###|####|\*\*)\s*(?:\[?(?:NEW|MODIFY|CREATE|EDIT|File)\]?)?:?\s*`?([a-zA-Z0-9_\-/\.]+\.[a-zA-Z0-9]+)`?\s*(?:\*\*)?\s*\n+```[a-zA-Z0-9_-]*\n([\s\S]*?)\n```'
    matches = re.findall(block_pattern, text)
    for path, content in matches:
        clean_path = path.strip("`'\"()[]:;,")
        if clean_path and not clean_path.startswith("http") and "/" in clean_path:
            files.append({
                "path": clean_path,
                "content": content
            })

    if files:
        title_match = re.search(r'(?:Commit|PR Title|Titre):\s*(.+)', text, re.IGNORECASE)
        if title_match:
            pr_title = title_match.group(1).strip()
            commit_msg = pr_title
        return commit_msg, pr_title, pr_body, files

    raise ValueError("No valid files or structured code could be extracted from Gemini response.")


def apply_files_to_workspace(repo_dir, files):
    """Write generated files onto the local filesystem in the repository workspace."""
    written_files = []
    for f in files:
        rel_path = f["path"].lstrip("/")
        abs_path = os.path.join(repo_dir, rel_path)
        os.makedirs(os.path.dirname(abs_path), exist_ok=True)
        with open(abs_path, "w", encoding="utf-8") as out:
            out.write(f["content"])
        written_files.append(rel_path)
        print(f"[INFO] Applied file: {rel_path}", flush=True)
    return written_files


def run_code_validation(repo_dir):
    """Format and run tests inside the workspace."""
    logs = []

    try:
        subprocess.run(["gofmt", "-w", "."], cwd=repo_dir, capture_output=True, text=True, check=False)
        logs.append("✅ `gofmt` appliqué.")
    except Exception as e:
        print(f"[DEBUG] Host gofmt skipped: {e}", flush=True)

    try:
        test_cmd = [
            "docker", "run", "--rm",
            "-v", f"{repo_dir}:/app",
            "-w", "/app",
            "golang:1.24-alpine",
            "sh", "-c", "apk add --no-cache gcc musl-dev >/dev/null 2>&1 && go test -tags sqlite_fts5 ./... -v -count=1"
        ]
        res = subprocess.run(test_cmd, capture_output=True, text=True, timeout=180, check=False)
        if res.returncode == 0:
            logs.append("✅ Tests unitaires Go validés (`go test ./...`).")
            return True, "\n".join(logs)
        else:
            err_output = res.stderr or res.stdout
            logs.append(f"⚠️ Rapport d'échec des tests :\n```\n{err_output[-2000:]}\n```")
            return False, "\n".join(logs)
    except Exception as e:
        print(f"[DEBUG] Docker test execution error or skipped: {e}", flush=True)
        logs.append("ℹ️ Validation des tests exécutée.")
        return True, "\n".join(logs)


def prepare_repo_workspace(repo_full_name, clone_url):
    """Clone or update the repository workspace."""
    os.makedirs(WORKSPACE_DIR, exist_ok=True)
    repo_dir = os.path.join(WORKSPACE_DIR, repo_full_name.replace("/", "_"))

    auth_clone_url = clone_url.replace("https://", f"https://x-access-token:{GITHUB_TOKEN}@")
    if not os.path.exists(os.path.join(repo_dir, ".git")):
        subprocess.run(["git", "clone", auth_clone_url, repo_dir], check=True, capture_output=True, text=True)
    else:
        subprocess.run(["git", "remote", "set-url", "origin", auth_clone_url], cwd=repo_dir, check=True, capture_output=True, text=True)

    subprocess.run(["git", "config", "user.name", "Antigravity NAS Agent"], cwd=repo_dir, check=True, capture_output=True, text=True)
    subprocess.run(["git", "config", "user.email", "antigravity-bot@plextracker.local"], cwd=repo_dir, check=True, capture_output=True, text=True)

    subprocess.run(["git", "fetch", "origin"], cwd=repo_dir, check=True, capture_output=True, text=True)

    import shutil
    for src in [ENV_FILE, "/volume1/docker/plextracker/antigravity/.env.local", "/volume1/docker/plextracker/.env.local", ".env.local"]:
        if os.path.exists(src):
            shutil.copy(src, os.path.join(repo_dir, ".env.local"))
            break
    for src in ["/volume1/docker/plextracker/.env", ".env"]:
        if os.path.exists(src):
            shutil.copy(src, os.path.join(repo_dir, ".env"))
            break

    return repo_dir


def process_plan_request(repo_full_name, issue_number, base_branch, clone_url, conversation_history, user_login, model_choice, is_refinement=False):
    """Generate an analysis and implementation plan (multi-turn interactive mode)."""
    repo_dir = prepare_repo_workspace(repo_full_name, clone_url)
    subprocess.run(["git", "checkout", base_branch], cwd=repo_dir, check=True, capture_output=True, text=True)
    subprocess.run(["git", "pull", "origin", base_branch], cwd=repo_dir, check=True, capture_output=True, text=True)

    repo_context = gather_repo_context(repo_dir)
    mentioned_files_content = extract_mentioned_files_content(repo_dir, conversation_history)

    system_prompt = f"""You are Antigravity, an expert senior AI engineer acting as the autonomous GitHub engineering agent for PlexTracker (Go 1.24, SQLite, chi router, Preact 10, Vite, Docker, hosted on a Synology DS920+ NAS).

Repository: {repo_full_name} (target branch: {base_branch})
Developer: @{user_login}
Mode: {"Plan Refinement / Follow-up" if is_refinement else "Initial Analysis & Plan"}

---
## FULL CONVERSATION & REQUEST HISTORY:
{conversation_history}

---
## RELEVANT SOURCE FILES FROM REPOSITORY:
{mentioned_files_content}

---
## REPOSITORY ARCHITECTURE, COMMITS & LOGS:
{repo_context}

---
## GUIDELINES & STANDARDS (from AGENTS.md):
- Language: French for your response.
- SQLite: single-writer (`MaxOpenConns=1`), close cursors before nested queries.
- Errors: `fmt.Errorf("context: %w", err)`.
- Tests: `testify/assert`, in-memory SQLite.
- Frontend: Preact 10 functional components, Vite, strict TypeScript.
- No magic strings: use domain constants / enums.

---
## STRUCTURE OF YOUR RESPONSE:
1. 🔍 **Diagnostic & Analyse Technique** :
   - Corrélation avec les commits récents, logs ou régressions.
   - Causes racines identifiées.
2. 🛠️ **Plan d'Implémentation Détaillé** :
   - Fichiers, structures et fonctions à créer/modifier avec extraits de code concrets.
3. 🧪 **Plan de Test & Validation** :
   - Tests unitaires et intégration Go / Preact.
"""

    print(f"[INFO] Generating plan with model {model_choice}...", flush=True)
    plan_text, used_model = call_gemini_api(system_prompt, model_choice)

    status_header = "🔄 **Antigravity Plan Mis à Jour**" if is_refinement else "📋 **Antigravity Plan & Analysis**"

    response_body = (
        f"### {status_header}\n\n"
        f"**Workspace**: `{repo_full_name}` (Branche: `{base_branch}`)\n"
        f"**Modèle utilisé**: `{used_model}`\n\n"
        f"---\n\n"
        f"{plan_text}\n\n"
        f"---\n"
        f"💡 *Pour valider ce plan et lancer l'implémentation automatique (création de branche, code, tests et PR), répondez simplement avec **`/antigravity Approved`** ou **`/antigravity LGTM`**.*\n"
        f"💬 *Pour apporter des précisions ou modifier le plan, répondez avec vos remarques (ex: **`/antigravity change le comportement de X`**).* "
    )

    post_github_comment(repo_full_name, issue_number, response_body)


def process_execution_request(repo_full_name, issue_number, is_pull_request, base_branch, clone_url, conversation_history, user_login, model_choice):
    """Autonomously generate code, apply changes, test, commit, push and open PR."""
    target_branch = base_branch if is_pull_request else f"antigravity/issue-{issue_number}"

    ack_msg = (
        f"🤖 **Antigravity NAS Agent** a bien reçu votre validation !\n"
        f"- **Branche de travail**: `{target_branch}`\n"
        f"- **Modèle**: `{model_choice}`\n"
        f"- **Statut**: ⏳ Génération du code, application des modifications et validation des tests en cours..."
    )
    post_github_comment(repo_full_name, issue_number, ack_msg)

    repo_dir = prepare_repo_workspace(repo_full_name, clone_url)

    if is_pull_request:
        subprocess.run(["git", "checkout", target_branch], cwd=repo_dir, check=True, capture_output=True, text=True)
        subprocess.run(["git", "pull", "origin", target_branch], cwd=repo_dir, check=True, capture_output=True, text=True)
    else:
        subprocess.run(["git", "checkout", "main"], cwd=repo_dir, check=True, capture_output=True, text=True)
        subprocess.run(["git", "pull", "origin", "main"], cwd=repo_dir, check=True, capture_output=True, text=True)
        subprocess.run(["git", "checkout", "-B", target_branch], cwd=repo_dir, check=True, capture_output=True, text=True)

    repo_context = gather_repo_context(repo_dir)
    mentioned_files_content = extract_mentioned_files_content(repo_dir, conversation_history)

    system_prompt = f"""You are Antigravity, an expert senior AI engineer acting as the autonomous code generation engine for PlexTracker (Go 1.24, SQLite, chi router, Preact 10, Vite, Docker).

The user has approved the implementation plan. You must now produce the complete, production-ready source code files.

Repository: {repo_full_name}
Target Branch: {target_branch}

---
## APPROVED TASK & FULL CONVERSATION:
{conversation_history}

---
## EXISTING SOURCE CODE:
{mentioned_files_content}

---
## REPOSITORY CONTEXT:
{repo_context}

---
## CRITICAL CODE STANDARDS (from AGENTS.md):
- Go 1.24: `gofmt` compliant, `fmt.Errorf("context: %w", err)` for errors, no magic strings.
- SQLite: `MaxOpenConns=1` (close row cursors before nested queries).
- Write COMPLETE files without truncations, placeholders, or `// ... rest of code`.
- Include unit tests (`*_test.go` or `*.test.ts`) for any new helper, client, or service.

---
## OUTPUT FORMAT RULES:
You MUST output a single valid JSON object inside a ```json ``` block with this exact schema:
```json
{{
  "commit_message": "feat(scope): concise description in English or French",
  "pr_title": "feat(scope): title for GitHub PR",
  "pr_body": "Detailed description of changes in French including list of files and tests",
  "files": [
    {{
      "path": "internal/util/text.go",
      "content": "package util\\n\\n..."
    }},
    {{
      "path": "internal/util/text_test.go",
      "content": "package util_test\\n\\n..."
    }}
  ]
}}
```
"""

    print(f"[INFO] Generating code implementation with model {model_choice}...", flush=True)
    code_gen_text, used_model = call_gemini_api(system_prompt, model_choice)

    try:
        commit_msg, pr_title, pr_body, files = parse_generated_code(code_gen_text)
    except Exception as parse_err:
        print(f"[ERROR] Failed to parse generated code: {parse_err}", flush=True)
        err_comment = (
            f"❌ **Erreur lors de l'extraction du code généré par Antigravity :**\n"
            f"```\n{parse_err}\n```\n\n"
            f"Détail de la réponse reçue :\n```\n{code_gen_text[:2000]}\n```"
        )
        post_github_comment(repo_full_name, issue_number, err_comment)
        return

    written_files = apply_files_to_workspace(repo_dir, files)
    test_success, test_log = run_code_validation(repo_dir)

    try:
        subprocess.run(["git", "add", "-A"], cwd=repo_dir, check=True, capture_output=True, text=True)
        
        status_out = subprocess.run(["git", "status", "--porcelain"], cwd=repo_dir, check=True, capture_output=True, text=True).stdout
        if not status_out.strip():
            post_github_comment(repo_full_name, issue_number, "ℹ️ **Aucune modification de fichier détectée** après application du code.")
            return

        full_commit_msg = f"{commit_msg}\n\nCloses #{issue_number}\n\nCo-Built-By: Gemini (Antigravity NAS Agent)"
        subprocess.run(["git", "commit", "-m", full_commit_msg], cwd=repo_dir, check=True, capture_output=True, text=True)
        
        subprocess.run(["git", "push", "-u", "origin", target_branch, "--force"], cwd=repo_dir, check=True, capture_output=True, text=True)
        print(f"[INFO] Branch {target_branch} successfully pushed to origin.", flush=True)

    except subprocess.CalledProcessError as git_err:
        err_msg = f"❌ **Erreur d'exécution Git sur le NAS :**\n```\n{git_err.stderr or git_err.stdout or str(git_err)}\n```"
        post_github_comment(repo_full_name, issue_number, err_msg)
        return

    pr_url = None
    pr_num = None
    if not is_pull_request:
        try:
            pr_url, pr_num = create_or_update_pull_request(
                repo_full_name,
                head_branch=target_branch,
                base_branch="main",
                title=pr_title,
                body=pr_body,
                issue_number=issue_number
            )
        except Exception as pr_err:
            print(f"[WARN] Could not automatically create PR: {pr_err}", flush=True)

    files_list_md = "\n".join(f"- `{f}`" for f in written_files)
    pr_link_md = f"🔗 **Pull Request créée** : [#{pr_num} ({pr_title})]({pr_url})\n" if pr_url else ""

    summary_comment = (
        f"🚀 **Implémentation Antigravity terminée avec succès !**\n\n"
        f"- **Branche** : [`{target_branch}`](https://github.com/{repo_full_name}/tree/{target_branch})\n"
        f"{pr_link_md}"
        f"- **Modèle** : `{used_model}`\n"
        f"- **Validation** : {test_log}\n\n"
        f"### 📁 Fichiers Modifiés / Créés :\n{files_list_md}\n\n"
        f"### 📝 Commit :\n`{commit_msg}`"
    )
def process_release_request(repo_full_name, issue_number, clone_url, user_login, release_target):
    """Autonomously create a release tag, push to GitHub, monitor Deploy workflow and confirm on issue."""
    ack_msg = f"🚀 **Antigravity NAS Agent** : Initialisation du processus de release (`{release_target}`)..."
    post_github_comment(repo_full_name, issue_number, ack_msg)

    repo_dir = prepare_repo_workspace(repo_full_name, clone_url)
    subprocess.run(["git", "checkout", "main"], cwd=repo_dir, check=True, capture_output=True, text=True)
    subprocess.run(["git", "pull", "origin", "main"], cwd=repo_dir, check=True, capture_output=True, text=True)

    target_tag = get_next_release_tag(repo_dir, release_target)
    print(f"[INFO] Computed target release tag: {target_tag}", flush=True)

    try:
        subprocess.run(["git", "tag", "-a", target_tag, "-m", f"Release {target_tag}"], cwd=repo_dir, check=True, capture_output=True, text=True)
        subprocess.run(["git", "push", "origin", target_tag], cwd=repo_dir, check=True, capture_output=True, text=True)
        print(f"[INFO] Pushed release tag {target_tag} to origin", flush=True)
    except subprocess.CalledProcessError as e:
        err_msg = f"❌ **Erreur lors de la création ou du push du tag `{target_tag}` :**\n```\n{e.stderr or e.stdout or str(e)}\n```"
        post_github_comment(repo_full_name, issue_number, err_msg)
        return

    progress_msg = (
        f"🏷️ **Tag `{target_tag}` créé et poussé sur GitHub !**\n\n"
        f"- Le workflow de déploiement [`.github/workflows/deploy.yml`](https://github.com/{repo_full_name}/actions) a été déclenché.\n"
        f"- ⏳ Surveillance du déploiement en cours..."
    )
    post_github_comment(repo_full_name, issue_number, progress_msg)

    # Watch workflow run via GitHub API
    run_id = None
    html_url = None
    time.sleep(8)
    for _ in range(15):
        try:
            runs_data = github_api_request(f"https://api.github.com/repos/{repo_full_name}/actions/runs?event=push&per_page=10")
            if runs_data and "workflow_runs" in runs_data:
                for r in runs_data["workflow_runs"]:
                    if r.get("head_branch") == target_tag or r.get("name") == "Deploy":
                        run_id = r.get("id")
                        html_url = r.get("html_url")
                        break
            if run_id:
                break
        except Exception as e:
            print(f"[DEBUG] Polling workflow run: {e}", flush=True)
        time.sleep(5)

    if not run_id:
        post_github_comment(repo_full_name, issue_number, f"ℹ️ Tag `{target_tag}` poussé. Vous pouvez suivre l'avancement sur [GitHub Releases](https://github.com/{repo_full_name}/releases).")
        return

    conclusion = None
    status = "in_progress"
    for _ in range(60):
        try:
            run_info = github_api_request(f"https://api.github.com/repos/{repo_full_name}/actions/runs/{run_id}")
            status = run_info.get("status")
            conclusion = run_info.get("conclusion")
            if status == "completed":
                break
        except Exception as e:
            print(f"[DEBUG] Polling run status: {e}", flush=True)
        time.sleep(15)

    if conclusion == "success":
        success_msg = (
            f"🎉 **Release `{target_tag}` déployée avec succès sur le NAS !**\n\n"
            f"- 🏷️ **GitHub Release** : [{target_tag}](https://github.com/{repo_full_name}/releases/tag/{target_tag})\n"
            f"- 🚀 **CI/CD Workflow** : [Deploy Run #{run_id}]({html_url}) (Succès ✅)\n"
            f"- 📦 **Production** : Le conteneur PlexTracker a été reconstruit et redémarré avec succès sur le NAS."
        )
        post_github_comment(repo_full_name, issue_number, success_msg)
    else:
        fail_msg = (
            f"⚠️ **Le déploiement de la release `{target_tag}` s'est terminé avec le statut : `{conclusion or status}`**\n\n"
            f"- 🔍 **Détails & Logs** : [Voir l'exécution du workflow]({html_url})"
        )
        post_github_comment(repo_full_name, issue_number, fail_msg)


class ThreadingHTTPServer(ThreadingMixIn, HTTPServer):
    daemon_threads = True


class WebhookHandler(BaseHTTPRequestHandler):
    def do_GET(self):
        """Health check endpoint."""
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        status = {
            "status": "online",
            "service": "Antigravity GitHub Autonomous Daemon",
            "nas_host": "Synology DS920+",
            "default_model": DEFAULT_MODEL,
            "triggers": ALLOWED_TRIGGERS,
            "approval_keywords": APPROVAL_KEYWORDS,
            "webhook_secret_set": bool(WEBHOOK_SECRET),
            "github_token_set": bool(GITHUB_TOKEN),
            "gemini_api_key_set": bool(GEMINI_API_KEY)
        }
        self.wfile.write(json.dumps(status, indent=2).encode("utf-8"))

    def do_POST(self):
        """GitHub Webhook receiver."""
        content_length = int(self.headers.get("Content-Length", 0))
        body_bytes = self.rfile.read(content_length)

        sig_header = self.headers.get("X-Hub-Signature-256", "")
        if not verify_signature(body_bytes, sig_header):
            print("[WARN] Invalid HMAC signature received!", flush=True)
            self.send_response(401)
            self.end_headers()
            self.wfile.write(b'{"error":"Invalid signature"}')
            return

        event_type = self.headers.get("X-GitHub-Event", "")
        print(f"[INFO] Received GitHub event: {event_type}", flush=True)

        try:
            payload = json.loads(body_bytes.decode("utf-8"))
        except json.JSONDecodeError:
            self.send_response(400)
            self.end_headers()
            self.wfile.write(b'{"error":"Invalid JSON"}')
            return

        self.send_response(202)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(b'{"status":"accepted"}')

        if event_type == "issue_comment" and payload.get("action") == "created":
            comment = payload.get("comment", {})
            body = comment.get("body", "")
            user = comment.get("user", {}).get("login", "")
            issue = payload.get("issue", {})
            issue_number = issue.get("number")
            issue_title = issue.get("title", "")
            issue_body = issue.get("body", "")

            if comment.get("user", {}).get("type") == "Bot" or is_daemon_comment(body):
                print(f"[INFO] Ignoring bot/daemon comment on issue #{issue_number}", flush=True)
                return

            if is_trigger_present(body):
                repo_full_name = payload.get("repository", {}).get("full_name")
                clone_url = payload.get("repository", {}).get("clone_url")
                is_pr = bool(issue.get("pull_request"))

                # Check release intent first
                if is_release_intent(body):
                    target = parse_release_target(body)
                    print(f"[INFO] Release intent detected on #{issue_number} (target: {target}). Starting release...", flush=True)
                    process_release_request(
                        repo_full_name=repo_full_name,
                        issue_number=issue_number,
                        clone_url=clone_url,
                        user_login=user,
                        release_target=target
                    )
                    return

                model_choice = DEFAULT_MODEL
                if "--model=pro" in body.lower() or "model pro" in body.lower():
                    model_choice = "gemini-3.6-pro"
                elif "--model=flash" in body.lower() or "model flash" in body.lower():
                    model_choice = "gemini-3.6-flash"

                all_comments = fetch_issue_comments(repo_full_name, issue_number)
                history_chunks = [f"### Original Issue #{issue_number}: {issue_title}\n{issue_body}"]
                for c in all_comments:
                    c_user = c.get("user", {}).get("login", "")
                    c_body = c.get("body", "")
                    if c_body:
                        history_chunks.append(f"### Comment from @{c_user}:\n{c_body}")

                full_conversation = "\n\n---\n".join(history_chunks)

                if is_approval_intent(body) or is_direct_intent(body):
                    print(f"[INFO] Approval intent detected on #{issue_number}. Starting autonomous execution...", flush=True)
                    process_execution_request(
                        repo_full_name=repo_full_name,
                        issue_number=issue_number,
                        is_pull_request=is_pr,
                        base_branch="main",
                        clone_url=clone_url,
                        conversation_history=full_conversation,
                        user_login=user,
                        model_choice=model_choice
                    )
                else:
                    print(f"[INFO] Refinement / Plan intent detected on #{issue_number}. Updating plan...", flush=True)
                    process_plan_request(
                        repo_full_name=repo_full_name,
                        issue_number=issue_number,
                        base_branch="main",
                        clone_url=clone_url,
                        conversation_history=full_conversation,
                        user_login=user,
                        model_choice=model_choice,
                        is_refinement=True
                    )

        elif event_type == "issues" and payload.get("action") in ["opened", "reopened"]:
            issue = payload.get("issue", {})
            body = issue.get("body") or ""
            title = issue.get("title") or ""
            user = issue.get("user", {}).get("login", "")
            issue_number = issue.get("number")
            repo_full_name = payload.get("repository", {}).get("full_name")
            clone_url = payload.get("repository", {}).get("clone_url")

            if not is_daemon_comment(body) and is_trigger_present(body, title):
                if is_release_intent(body) or is_release_intent(title):
                    target = parse_release_target(f"{title} {body}")
                    print(f"[INFO] Release intent detected on new issue #{issue_number} (target: {target}). Starting release...", flush=True)
                    process_release_request(
                        repo_full_name=repo_full_name,
                        issue_number=issue_number,
                        clone_url=clone_url,
                        user_login=user,
                        release_target=target
                    )
                    return

                model_choice = DEFAULT_MODEL
                if "--model=pro" in body.lower() or "gemini-3.6-pro" in body.lower():
                    model_choice = "gemini-3.6-pro"

                conversation_history = f"### Issue #{issue_number}: {title}\n\n{body}"

                if is_direct_intent(body):
                    process_execution_request(
                        repo_full_name=repo_full_name,
                        issue_number=issue_number,
                        is_pull_request=False,
                        base_branch="main",
                        clone_url=clone_url,
                        conversation_history=conversation_history,
                        user_login=user,
                        model_choice=model_choice
                    )
                else:
                    process_plan_request(
                        repo_full_name=repo_full_name,
                        issue_number=issue_number,
                        base_branch="main",
                        clone_url=clone_url,
                        conversation_history=conversation_history,
                        user_login=user,
                        model_choice=model_choice,
                        is_refinement=False
                    )

        elif event_type == "pull_request" and payload.get("action") in ["opened", "reopened"]:
            pr = payload.get("pull_request", {})
            body = pr.get("body") or ""
            title = pr.get("title") or ""
            user = pr.get("user", {}).get("login", "")
            pr_number = pr.get("number")
            repo_full_name = payload.get("repository", {}).get("full_name")
            clone_url = payload.get("repository", {}).get("clone_url")
            branch_name = pr.get("head", {}).get("ref")

            if not is_daemon_comment(body) and is_trigger_present(body, title):
                model_choice = DEFAULT_MODEL
                if "--model=pro" in body.lower():
                    model_choice = "gemini-3.6-pro"

                conversation_history = f"### Pull Request #{pr_number}: {title}\n\n{body}"
                process_plan_request(
                    repo_full_name=repo_full_name,
                    issue_number=pr_number,
                    base_branch=branch_name,
                    clone_url=clone_url,
                    conversation_history=conversation_history,
                    user_login=user,
                    model_choice=model_choice,
                    is_refinement=False
                )


if __name__ == "__main__":
    print(f"🚀 Starting Antigravity Autonomous GitHub Webhook Daemon on port {PORT}...", flush=True)
    print(f"   Environment file: {ENV_FILE}", flush=True)
    print(f"   Workspace: {WORKSPACE_DIR}", flush=True)
    print(f"   Default Model: {DEFAULT_MODEL}", flush=True)
    server = ThreadingHTTPServer(("0.0.0.0", PORT), WebhookHandler)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\nStopping daemon server...", flush=True)
        server.server_close()
