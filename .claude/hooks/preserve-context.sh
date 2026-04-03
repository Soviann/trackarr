#!/bin/bash
# Runs before every compaction
# Read what's about to be compacted
INPUT=$(cat)
# Ask Claude to extract and save the most critical context
claude -p "From this conversation, extract and save to .claude/session-handoff.md:
1. All files modified
2. All decisions made with their reasoning
3. Current task state and next step
4. Any open questions or blockers
Format it as markdown. Be specific." <<< "$INPUT"
echo "Context preserved to .claude/session-handoff.md"
