---
description: "Run Loom MCP weaving mode to drive server-side GitHub PRs human-out-of-loop until completion. Use when: running /run-loom, operating Loom, weaving PR lifecycle automatically."
agent: "loom-mcp-operator"
tools: [read, search, execute, loom, github, agent]
---

## Agents & Skills

| Agent | Skills |
|-------|--------|
| @loom-mcp-operator | `loom-mcp-loop` |

Begin Loom weaving mode.

## Pre-flight Checks

Before entering the loop, verify:
1. Loom MCP tools are available (`loom_next_step`, `loom_checkpoint`, `loom_heartbeat`, `loom_get_state`, `loom_abort`)
2. GitHub MCP tools for issues/PRs/reviews/merge are available
3. `.github/squad_prompts/WORKFLOW_GITHUB.md` exists for workflow semantics
4. `docs/plan/backlog.yaml` is present if Loom needs local planning context for diagnostics

## Execution

Start the canonical Loom loop and continue until Loom reports `COMPLETE` or transitions to `PAUSED`:
1. Call `loom_next_step`
2. Execute exactly one corresponding GitHub-side workflow step
3. Call `loom_checkpoint` with the canonical action
4. While waiting on async gates, call `loom_heartbeat` every 30 seconds

If Loom and live GitHub state diverge and cannot be reconciled safely, call `loom_abort` and return a concise operator handoff.
