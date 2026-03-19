---
description: "Run Loom MCP weaving mode to drive server-side GitHub PRs human-out-of-loop until completion. Use when: running /run-loom, operating Loom, weaving PR lifecycle automatically."
agent: "Loom MCP Operator"
tools: [read, search, execute, loom, github, agent]
---

## Agents & Skills

| Agent | Skills |
|-------|--------|
| @Loom MCP Operator | `loom-mcp-loop` |

Begin Loom weaving mode.

## Pre-flight Checks

Before entering the loop, verify:
1. Loom MCP tools are available (`loom_next_step`, `loom_checkpoint`, `loom_heartbeat`, `loom_get_state`, `loom_abort`)
2. GitHub MCP tools for issues/PRs/reviews/merge are available
3. `.github/squad_prompts/WORKFLOW_GITHUB.md` exists for workflow semantics
4. `docs/plan/backlog.yaml` is present if Loom needs local planning context for diagnostics

If any Loom MCP tool is unavailable, STOP immediately and return a setup error.
Do not continue with GitHub-only exploration or actions.

Setup error response must include these operator steps:
1. Verify `.github/copilot/mcp.json` contains the `loom` stdio server (`command: loom`, `args: ["mcp"]`)
2. Run `loom start` from the target repository root
3. Start a fresh `/run-loom` session and re-run pre-flight checks

## Execution

Start the canonical Loom loop and continue until Loom reports `COMPLETE` or transitions to `PAUSED`:
1. Call `loom_next_step`
2. Execute exactly one corresponding GitHub-side workflow step
3. Call `loom_checkpoint` with the canonical action
4. While waiting on async gates, call `loom_heartbeat` every 30 seconds

Interpret async gates with product intent:
1. In `AWAITING_READY`, if the PR is still draft but the Copilot coding job is complete, mark the PR ready for review and checkpoint `pr_ready`.
2. In `REVIEWING`, ensure a review is actually requested before only polling for approval or changes requested.

If Loom and live GitHub state diverge and cannot be reconciled safely, call `loom_abort` and return a concise operator handoff.
