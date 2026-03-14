#!/usr/bin/env bash
# Save session state on session end for resumability
set -euo pipefail

TIMESTAMP=$(date -u +%Y-%m-%dT%H:%M:%SZ)
PROCESS_ROOT="docs"
if [ ! -f "docs/plan/backlog.yaml" ] && [ -f "docs/plan/backlog.yaml" ]; then
    PROCESS_ROOT="docs"
fi
LOG_FILE="${PROCESS_ROOT}/plan/session-log.md"

# Ensure the log file directory exists
mkdir -p "$(dirname "$LOG_FILE")"

# Count story-level statuses only (lines indented with 14+ spaces are story-level)
if [ -f "${PROCESS_ROOT}/plan/backlog.yaml" ]; then
    TODO=$(grep -E '^\s{14,}status: todo' "${PROCESS_ROOT}/plan/backlog.yaml" 2>/dev/null | wc -l)
    IN_PROGRESS=$(grep -E '^\s{14,}status: in-progress' "${PROCESS_ROOT}/plan/backlog.yaml" 2>/dev/null | wc -l)
    DONE=$(grep -E '^\s{14,}status: done' "${PROCESS_ROOT}/plan/backlog.yaml" 2>/dev/null | wc -l)
    FAILED=$(grep -E '^\s{14,}status: failed' "${PROCESS_ROOT}/plan/backlog.yaml" 2>/dev/null | wc -l)
    echo "### Session ended: ${TIMESTAMP}" >> "$LOG_FILE"
    echo "- Todo: ${TODO} | In-progress: ${IN_PROGRESS} | Done: ${DONE} | Failed: ${FAILED}" >> "$LOG_FILE"
    echo "" >> "$LOG_FILE"
else
    echo "### Session ended: ${TIMESTAMP}" >> "$LOG_FILE"
    echo "- No backlog file found" >> "$LOG_FILE"
    echo "" >> "$LOG_FILE"
fi
