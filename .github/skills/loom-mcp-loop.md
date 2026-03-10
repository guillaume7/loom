---
description: >
  Operating contract for Loom's master Copilot session. Use when running the
  local Loom MCP workflow via loom_next_step, loom_checkpoint,
  loom_heartbeat, loom_get_state, and loom_abort.
---

# Skill: Loom MCP Loop

This skill defines how the **Loom MCP Operator** drives Loom's deterministic
workflow engine from a persistent Copilot session.

> **Rule zero**: the FSM in Loom is authoritative. The session supplies
> execution and judgment, not its own workflow state.

## Objective

Drive the repository through the full `WORKFLOW_GITHUB.md` loop by repeating:

1. ask Loom for the next step
2. perform that step using GitHub MCP tools and repository files
3. checkpoint the outcome back into Loom

## Core Loop

```text
LOOP
  1. Call loom_next_step.
  2. If state == COMPLETE, stop.
  3. Execute exactly one workflow step implied by the instruction.
  4. Call loom_checkpoint with the canonical action for that completed step.
  5. If the workflow is waiting on an async gate, call loom_heartbeat every 30 seconds.
  6. Repeat.
```

Use `loom_get_state` only for diagnosis or reconciliation. Use `loom_abort` if
the machine and live GitHub state cannot be reconciled safely.

## Canonical Checkpoint Actions

Use only these action names with `loom_checkpoint`:

| Situation | Action |
|---|---|
| Starting the workflow from `IDLE` | `start` |
| Next phase identified and its issue created | `phase_identified` |
| All phases complete | `all_phases_done` |
| `@copilot` assigned to the issue | `copilot_assigned` |
| PR opened for the active issue | `pr_opened` |
| Gate wait budget elapsed with no progress | `timeout` |
| Draft PR is ready for review | `pr_ready` |
| CI passed | `ci_green` |
| CI failed and debug loop must begin | `ci_red` |
| Review approved | `review_approved` |
| Review requested changes | `review_changes_requested` |
| Debug fix pushed to the PR branch | `fix_pushed` |
| Review feedback addressed on the PR branch | `feedback_addressed` |
| PR merged and no epic boundary was crossed | `merged` |
| PR merged and an epic-boundary refactor sweep must begin | `merged_epic_boundary` |
| Refactor PR merged | `refactor_merged` |
| Unsafe condition or explicit stop | `abort` via `loom_abort` |

Do not invent synonyms such as "done", "green", or "approved".

## State-by-State Behaviour

### `IDLE`

- Call `loom_checkpoint` with `start` to begin.

### `SCANNING`

- Read epics, story files, open issues, and open PRs.
- Determine the next unblocked phase.
- Create or update the phase issue from the repository templates.
- When the phase issue is ready, checkpoint `phase_identified`.
- If everything is already shipped, checkpoint `all_phases_done`.

### `ISSUE_CREATED`

- Assign `@copilot` to the issue.
- Checkpoint `copilot_assigned`.

### `AWAITING_PR`

- Poll GitHub for a PR opened by `@copilot` for the active issue/branch.
- If found, checkpoint `pr_opened`.
- If the configured wait budget genuinely expires, checkpoint `timeout` once.
- While waiting, call `loom_heartbeat` every 30 seconds.

### `AWAITING_READY`

- Poll until the PR is no longer draft.
- Checkpoint `pr_ready` when the PR becomes ready.
- If the wait budget genuinely expires, checkpoint `timeout` once.
- Keep the session alive with `loom_heartbeat`.

### `AWAITING_CI`

- Poll the PR check-runs.
- Checkpoint `ci_green` only when the required checks are green.
- Checkpoint `ci_red` only when a debug issue must be created.
- Checkpoint `timeout` only when the CI wait budget is exhausted without a terminal result.
- Use `loom_heartbeat` during every wait interval.

### `REVIEWING`

- Request Copilot review and inspect the review result.
- Checkpoint `review_approved` or `review_changes_requested`.

### `DEBUGGING`

- Drive the debug issue until a fix lands on the active PR branch.
- Checkpoint `fix_pushed` when the fix is present.
- Use `loom_heartbeat` while waiting.

### `ADDRESSING_FEEDBACK`

- Wait for `@copilot` to address requested changes.
- Checkpoint `feedback_addressed` once a new revision addresses the blockers.
- Use `loom_heartbeat` while waiting.

### `MERGING`

- Merge only after review approval and green CI.
- If the merge crosses an epic boundary and a refactor sweep is required, checkpoint `merged_epic_boundary`.
- Otherwise checkpoint `merged`.

### `REFACTORING`

- Create and drive the refactor issue/PR for the just-completed epic.
- Checkpoint `refactor_merged` when that PR merges.
- Use `loom_heartbeat` while waiting.

### `PAUSED`

- Stop autonomous execution.
- Summarize the blocking condition for the human operator.
- Do not attempt to self-resume.

## Safety Rules

1. Never checkpoint a step that did not actually happen on GitHub.
2. Never post duplicate nudges, reviews, or issue bodies without checking current state first.
3. If `loom_checkpoint` rejects an action, inspect with `loom_get_state` before retrying.
4. If repository evidence and Loom state disagree, prefer safety: diagnose, then abort if needed.
5. Keep comments concise and operational; the repository timeline is part of the durable audit trail.

## Minimal Recovery Procedure

If the session is resumed mid-flight:

1. Call `loom_get_state`.
2. Inspect the live GitHub issue, PR, and review state for the current phase.
3. Re-enter the normal loop with `loom_next_step`.
4. Continue only when the GitHub state and Loom checkpoint agree.