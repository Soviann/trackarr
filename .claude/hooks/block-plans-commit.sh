#!/bin/bash
# Bloque un `git commit` si des fichiers sous docs/plans/ sont staged.
set -e

STAGED=$(git diff --cached --name-only 2>/dev/null || true)

if echo "$STAGED" | grep -q "^docs/plans/"; then
  echo "Blocked: docs/plans/ staged files detected:" >&2
  echo "$STAGED" | grep "^docs/plans/" >&2
  echo "" >&2
  echo "AGENTS.md forbids committing plans. Run: git reset docs/plans/" >&2
  exit 2
fi

exit 0
