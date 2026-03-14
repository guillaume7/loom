---
name: loom-mcp-loop
description: "Operating contract for Loom master-session weaving. Use when running the server-side loop via loom_next_step, loom_checkpoint, loom_heartbeat, loom_get_state, and loom_abort."
---

# Skill: Loom MCP Loop

This skill defines how the Loom MCP Operator drives Loom's deterministic workflow engine from a persistent Copilot session.

## Objective

Drive the repository through the full workflow by repeating:
1. Ask Loom for the next step.
2. Perform exactly one corresponding GitHub action.
3. Checkpoint the completed action back into Loom.

## Core Loop

```text
LOOP
  1. Call loom_next_step.
  2. If state == COMPLETE, stop.
  3. Execute exactly one workflow step implied by the instruction.
  4. Call loom_checkpoint with the canonical action for that step.
  5. If waiting on an async gate, call loom_heartbeat every 30 seconds.
  6. Repeat.
```

Use `loom_get_state` only for diagnosis/reconciliation. Use `loom_abort` if Loom state and live GitHub state cannot be reconciled safely.

## Canonical Checkpoint Actions

| Situation | Action |
|---|---|
| Starting from `IDLE` | `start` |
| Phase issue identified/created | `phase_identified` |
| All phases complete | `all_phases_done` |
| `@copilot` assigned | `copilot_assigned` |
| PR opened | `pr_opened` |
| Wait budget elapsed | `timeout` |
| Draft PR ready | `pr_ready` |
| CI passed | `ci_green` |
| CI failed | `ci_red` |
| Review approved | `review_approved` |
| Review requested changes | `review_changes_requested` |
| Debug fix pushed | `fix_pushed` |
| Feedback addressed | `feedback_addressed` |
| PR merged (no epic boundary) | `merged` |
| PR merged (epic boundary) | `merged_epic_boundary` |
| Refactor PR merged | `refactor_merged` |

Do not invent alternate action names.

## Safety Rules

1. Never checkpoint a step that did not happen.
2. Never post duplicate nudges/reviews/comments without checking current state.
3. If checkpoint is rejected, inspect with `loom_get_state` before retrying.
4. If live state and Loom state disagree, prefer safety: reconcile or abort.
5. Keep operator comments concise and operational.
