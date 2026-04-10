#!/usr/bin/env bash
# Stop hook : déclenche un point de contrôle de commit aux intervalles
# significatifs. Le hook ne fait que déclencher — il n'effectue jamais le
# commit lui-même. Toute la logique de rédaction et l'escalade sont à la
# charge de Claude.
#
# Conditions de déclenchement (TOUTES doivent être vraies) :
#   - ≥3 fichiers modifiés (staged + unstaged confondus)
#   - ≥30 min depuis le dernier commit
#   - Pas de marqueur "WIP" dans le message du HEAD
#   - Pas de sentinelle de snooze fraîche
#
# Sortie :
#   - Si une condition échoue : exit 0 silencieux (Claude peut s'arrêter)
#   - Si toutes passent : JSON `{"decision":"block","reason":"..."}` qui
#     empêche l'arrêt et injecte des instructions pour Claude
#
# Note : le hook Stop se déclenche à chaque fin de tour. Si le seuil
# 30min/3-fichiers s'avère trop bruyant, augmenter 1800 ou 3 dans le script.

set -e

INPUT=$(cat)
SID=$(echo "$INPUT" | jq -r '.session_id // empty')

PROJ="${CLAUDE_PROJECT_DIR:-.}"
cd "$PROJ" || exit 0

git rev-parse --git-dir >/dev/null 2>&1 || exit 0

# Snooze : sentinelle TTL 30 min
SNOOZE="/tmp/claude-commit-snooze-$SID"
if [ -f "$SNOOZE" ]; then
    MTIME=$(stat -f %m "$SNOOZE" 2>/dev/null || stat -c %Y "$SNOOZE" 2>/dev/null || echo 0)
    AGE=$(( $(date +%s) - MTIME ))
    if [ "$AGE" -lt 1800 ]; then
        exit 0
    fi
    rm -f "$SNOOZE"
fi

# Compteur de fichiers modifiés (staged + unstaged)
DIRTY=$(git status --porcelain | wc -l | tr -d ' ')
[ "$DIRTY" -ge 3 ] || exit 0

# Temps écoulé depuis le dernier commit
LAST=$(git log -1 --format=%ct 2>/dev/null) || exit 0
ELAPSED=$(( $(date +%s) - LAST ))
[ "$ELAPSED" -ge 1800 ] || exit 0

# Pas de WIP dans le HEAD
git log -1 --format=%s | grep -qi 'wip' && exit 0

MIN=$(( ELAPSED / 60 ))

REASON="COMMIT CHECKPOINT — ${DIRTY} fichiers modifiés, ${MIN} min depuis le dernier commit. Avant de t'arrêter : (1) lance \`git status\` et \`git diff\` pour examiner les changements ; (2) rédige un message de commit conventionnel français selon la skill \`commit\` (format \`<type>(scope): description\` avec 3e personne impératif — ajoute/corrige/supprime, le titre = impact visible et non détail d'implémentation) ; (3) escalade via AskUserQuestion avec EXACTEMENT ces trois options : **commit** (avec ton message proposé), **skip** (continuer sans commit), **snooze** (exécute \`touch /tmp/claude-commit-snooze-${SID}\` pour suspendre 30 min). NE COMMIT PAS automatiquement — l'utilisateur doit choisir."

jq -nc --arg r "$REASON" '{decision:"block",reason:$r}'
