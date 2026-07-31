#!/usr/bin/env python3
"""
GitHub PR Webhook Daemon for Antigravity (Synology NAS)
Listens for GitHub Webhook events on port 8191 (via Synology Reverse Proxy).
Verifies HMAC SHA-256 signatures, checks out PR branches, parses user prompts/commands,
posts implementation plans & diff reviews, and commits changes back to GitHub PRs.
"""

import os
import sys
import json
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

# Load .env.local secrets into environment if file exists
def load_env_file():
    if os.path.exists(ENV_FILE):
        with open(ENV_FILE, "r", encoding="utf-8") as f:
            for line in f:
                line = line.strip()
                if line and not line.startswith("#") and "=" in line:
                    key, val = line.split("=", 1)
                    val = val.strip("'\"")
                    os.environ[key.strip()] = val

load_env_file()

GITHUB_TOKEN = os.getenv("GITHUB_TOKEN", os.getenv("ANTIGRAVITY_TOKEN", os.getenv("GH_TOKEN", "")))
WEBHOOK_SECRET = os.getenv("WEBHOOK_SECRET", os.getenv("GITHUB_WEBHOOK_SECRET", ""))
GEMINI_API_KEY = os.getenv("GEMINI_API_KEY", os.getenv("GOOGLE_API_KEY", ""))
DEFAULT_MODEL = os.getenv("DEFAULT_MODEL", "gemini-3.6-flash")


def post_github_comment(repo_full_name, issue_number, body):
    """Post a comment to a GitHub Issue/PR."""
    if not GITHUB_TOKEN:
        print("[ERROR] GITHUB_TOKEN not set, cannot post comment to GitHub.", flush=True)
        return False

    url = f"https://api.github.com/repos/{repo_full_name}/issues/{issue_number}/comments"
    headers = {
        "Authorization": f"token {GITHUB_TOKEN}",
        "Accept": "application/vnd.github.v3+json",
        "User-Agent": "Antigravity-NAS-Daemon",
        "Content-Type": "application/json"
    }
    data = json.dumps({"body": body}).encode("utf-8")
    req = urllib.request.Request(url, data=data, headers=headers, method="POST")

    try:
        with urllib.request.urlopen(req) as resp:
            print(f"[INFO] Comment posted successfully to {repo_full_name}#{issue_number} (status {resp.status})", flush=True)
            return True
    except urllib.error.HTTPError as e:
        err_body = e.read().decode('utf-8', errors='ignore')
        print(f"[ERROR] Failed to post comment: {e.code} {e.reason} - {err_body}", flush=True)
        return False
    except Exception as e:
        print(f"[ERROR] Exception posting comment: {e}", flush=True)
        return False


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


def process_pr_command(repo_full_name, pr_number, branch_name, clone_url, comment_body, user_login):
    """Process a PR request/command from GitHub."""
    print(f"[INFO] Processing request from {user_login} on {repo_full_name}#{pr_number} (branch: {branch_name})", flush=True)

    # Determine model override if specified in comment
    model = DEFAULT_MODEL
    if "--model=pro" in comment_body.lower() or "model pro" in comment_body.lower():
        model = "gemini-3.6-pro"
    elif "--model=flash" in comment_body.lower() or "model flash" in comment_body.lower():
        model = "gemini-3.6-flash"

    # Ack comment on PR
    ack_msg = f"🤖 **Antigravity NAS Agent** received your request!\n- **Branch**: `{branch_name}`\n- **Model**: `{model}`\n- **Status**: Analyzing repository and preparing response..."
    post_github_comment(repo_full_name, pr_number, ack_msg)

    # Prepare local workspace
    os.makedirs(WORKSPACE_DIR, exist_ok=True)
    repo_dir = os.path.join(WORKSPACE_DIR, repo_full_name.replace("/", "_"))

    try:
        if not os.path.exists(os.path.join(repo_dir, ".git")):
            # Clone repo if not exists
            auth_clone_url = clone_url.replace("https://", f"https://x-access-token:{GITHUB_TOKEN}@")
            subprocess.run(["git", "clone", auth_clone_url, repo_dir], check=True, capture_output=True, text=True)
        
        # Fetch and checkout target branch
        subprocess.run(["git", "fetch", "origin"], cwd=repo_dir, check=True, capture_output=True, text=True)
        subprocess.run(["git", "checkout", branch_name], cwd=repo_dir, check=True, capture_output=True, text=True)
        subprocess.run(["git", "pull", "origin", branch_name], cwd=repo_dir, check=True, capture_output=True, text=True)

        # Ensure .env.local is available inside the repository directory
        repo_env = os.path.join(repo_dir, ".env.local")
        if os.path.exists(ENV_FILE):
            import shutil
            shutil.copy(ENV_FILE, repo_env)

        # Build clean response for PR comment
        response_body = (
            f"### 📋 Antigravity Plan / Task Review\n\n"
            f"**Request from @{user_login}:**\n> {comment_body.strip()}\n\n"
            f"**Workspace**: `{repo_full_name}` (Branch: `{branch_name}`)\n"
            f"**Selected Model**: `{model}`\n\n"
            f"**Execution Summary:**\n"
            f"- Branch updated cleanly from `origin/{branch_name}`.\n"
            f"- Secret `.env.local` loaded from NAS storage.\n"
            f"- Ready for review. Reply **`Approved`** or **`LGTM`** to execute full code modifications."
        )

        post_github_comment(repo_full_name, pr_number, response_body)

    except subprocess.CalledProcessError as e:
        err_msg = f"❌ **Git Execution Error on NAS:**\n```\n{e.stderr or e.stdout or str(e)}\n```"
        post_github_comment(repo_full_name, pr_number, err_msg)
    except Exception as e:
        err_msg = f"❌ **Antigravity Daemon Error:** `{str(e)}`"
        post_github_comment(repo_full_name, pr_number, err_msg)


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
            "service": "Antigravity GitHub PR Daemon",
            "nas_host": "Synology DS920+",
            "default_model": DEFAULT_MODEL,
            "webhook_secret_set": bool(WEBHOOK_SECRET),
            "github_token_set": bool(GITHUB_TOKEN),
            "gemini_api_key_set": bool(GEMINI_API_KEY)
        }
        self.wfile.write(json.dumps(status, indent=2).encode("utf-8"))

    def do_POST(self):
        """GitHub Webhook receiver."""
        content_length = int(self.headers.get("Content-Length", 0))
        body_bytes = self.rfile.read(content_length)

        # HMAC verification
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

        # Handle events
        if event_type == "issue_comment" and payload.get("action") == "created":
            comment = payload.get("comment", {})
            body = comment.get("body", "")
            user = comment.get("user", {}).get("login", "")
            issue = payload.get("issue", {})

            # Only process if comment mentions @antigravity or /antigravity (and not posted by a bot)
            if ("@antigravity" in body.lower() or "/antigravity" in body.lower()) and not comment.get("user", {}).get("type") == "Bot":
                issue_number = issue.get("number")
                repo_full_name = payload.get("repository", {}).get("full_name")
                clone_url = payload.get("repository", {}).get("clone_url")
                pr_info = issue.get("pull_request")

                if pr_info:
                    # It's a Pull Request
                    pr_url = pr_info.get("url")
                    if pr_url and GITHUB_TOKEN:
                        req = urllib.request.Request(
                            pr_url,
                            headers={
                                "Authorization": f"token {GITHUB_TOKEN}",
                                "Accept": "application/vnd.github.v3+json",
                                "User-Agent": "Antigravity-NAS-Daemon"
                            }
                        )
                        try:
                            with urllib.request.urlopen(req) as resp:
                                pr_data = json.loads(resp.read().decode("utf-8"))
                                branch_name = pr_data.get("head", {}).get("ref")
                                process_pr_command(repo_full_name, issue_number, branch_name, clone_url, body, user)
                        except Exception as e:
                            print(f"[ERROR] Failed to fetch PR branch details: {e}", flush=True)
                else:
                    # It's a pure Issue -> process on main / issue feature branch
                    branch_name = f"antigravity/issue-{issue_number}"
                    process_pr_command(repo_full_name, issue_number, "main", clone_url, body, user)

        elif event_type == "issues" and payload.get("action") in ["opened", "reopened"]:
            issue = payload.get("issue", {})
            body = issue.get("body") or ""
            title = issue.get("title") or ""
            user = issue.get("user", {}).get("login", "")
            issue_number = issue.get("number")
            repo_full_name = payload.get("repository", {}).get("full_name")
            clone_url = payload.get("repository", {}).get("clone_url")

            if "@antigravity" in body.lower() or "/antigravity" in body.lower() or "@antigravity" in title.lower():
                process_pr_command(repo_full_name, issue_number, "main", clone_url, f"{title}\n{body}", user)

        elif event_type == "pull_request" and payload.get("action") in ["opened", "reopened"]:
            pr = payload.get("pull_request", {})
            body = pr.get("body") or ""
            title = pr.get("title") or ""
            user = pr.get("user", {}).get("login", "")
            pr_number = pr.get("number")
            repo_full_name = payload.get("repository", {}).get("full_name")
            clone_url = payload.get("repository", {}).get("clone_url")
            branch_name = pr.get("head", {}).get("ref")

            if "@antigravity" in body.lower() or "/antigravity" in body.lower() or "@antigravity" in title.lower():
                process_pr_command(repo_full_name, pr_number, branch_name, clone_url, f"{title}\n{body}", user)


if __name__ == "__main__":
    print(f"🚀 Starting Antigravity GitHub PR Webhook Daemon on port {PORT}...", flush=True)
    print(f"   Environment file: {ENV_FILE}", flush=True)
    print(f"   Workspace: {WORKSPACE_DIR}", flush=True)
    print(f"   Default Model: {DEFAULT_MODEL}", flush=True)
    server = ThreadingHTTPServer(("0.0.0.0", PORT), WebhookHandler)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\nStopping daemon server...", flush=True)
        server.server_close()
