---
name: release
description: Create a release — determine version bump from changelog, update CHANGELOG.md, tag and push to trigger deploy
user_invocable: true
---

# Release

Create a new PlexTracker release.

## Steps

1. **Read `CHANGELOG.md`** — check `[Unreleased]` section
2. **If [Unreleased] is empty**, check `git log` since last tag for unreleased commits. If commits exist, build the changelog entries from them before proceeding. If truly nothing: tell user and stop.
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

Deploy triggers automatically via `.github/workflows/deploy.yml`.

## Rules

- **Never ask** the user which version bump — decide from the changes
- Verify the computed tag doesn't already exist locally or on remote (`git ls-remote --tags origin`) before creating it
- Announce the chosen version and rationale in one sentence before proceeding
