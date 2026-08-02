---
name: release
description: Create a release — determine version bump from changelog, update CHANGELOG.md, tag and push to trigger deploy
disable-model-invocation: true
---

# Release

Create a new PlexTracker release.

## Steps

1. **Read `CHANGELOG.md`** — check `[Unreleased]` section
2. **Always** run `git log <last-tag>..HEAD --oneline` to check for commits not yet reflected in `[Unreleased]`. Add any missing entries before proceeding. If `[Unreleased]` is empty **and** no commits exist: tell user and stop.
3. **Get current version** from remote: `git fetch --tags && git tag --sort=-v:refname | head -1`
4. **Choose version bump** based on [Unreleased] content:
   - `major`: breaking changes, major rewrites
   - `minor`: new features (sections `Ajouté`)
   - `patch`: bug fixes, improvements only (sections `Corrigé`, `Amélioré`)
5. **Update `CHANGELOG.md`**:
   - Replace `## [Unreleased]` heading with `## [vX.Y.Z] — YYYY-MM-DD`
   - Add fresh `## [Unreleased]` section above it (empty, no subsections)
6. **Commit**: `docs: met à jour CHANGELOG pour vX.Y.Z`
7. **Tag**: `git tag vX.Y.Z`
8. **Push**: `git push origin main --tags`

9. **Monitor Deployment**:
   - The push triggers `.github/workflows/deploy.yml`.
   - Find the triggered run using `gh run list --workflow=deploy.yml --limit 1`.
   - **MUST DO**: You must actively watch the release/deploy until it succeeds or fails (`gh run watch <run-id>`).
   - If the run fails, fetch the failed logs (`gh run view <run-id> --log-failed`), diagnose the issue, and present it to the user.
   - If it succeeds, notify the user that the deployment is successfully complete.
## Rules

- **Never ask** the user which version bump — decide from the changes
- Verify the computed tag doesn't already exist locally or on remote (`git ls-remote --tags origin`) before creating it
- Announce the chosen version and rationale in one sentence before proceeding
