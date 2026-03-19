# Squad Prompts — GitHub-Native Workflow Guide

How to drive Loom's GitHub-side workflow from GitHub.com and from `/run-loom` without drifting away from the product's intended behavior.

> For the local IDE workflow, see `WORKFLOW.md`.

---

## Purpose

This document is the workflow semantics reference for two related but different modes:

1. **Manual GitHub-native mode** — a maintainer works directly on GitHub.com with `@copilot`, CI, and PR reviews.
2. **Loom weaving mode** — the `loom-mcp-operator` runs `/run-loom` and drives the GitHub workflow end-to-end with no human in the loop until a true approval or pause boundary is reached.

For Loom itself, the authoritative product intent is:

- create issue
- assign `@copilot`
- wait for PR
- wait for CI
- request review
- handle review feedback
- merge

The review request in `/run-loom` is **not** a manual human step. It is an operator action Loom performs after CI passes.

---

## Review Model

Two review concepts exist in this repository:

| Context | Who performs review | How review starts |
|---|---|---|
| `/run-autopilot` | Local `@reviewer` agent | Local orchestrator delegates to the reviewer agent |
| GitHub-native PR flow | Copilot PR reviewer on GitHub | A review is requested on the PR |
| `/run-loom` | Copilot PR reviewer on GitHub | Loom requests the review automatically after `ci_green` |

Do not confuse the local `@reviewer` agent with the GitHub PR review flow. `/run-loom` does **not** assign the local reviewer agent. It moves the PR into the GitHub review stage.

---

## Loom-Specific Master Session

When running Loom from VS Code, use the persistent master session as the **Loom MCP Operator** defined in `.github/agents/loom-mcp-operator.agent.md` and apply `.github/skills/loom-mcp-loop.md`.

This session is distinct from the GitHub-side `@copilot` coding agent. Its job is to:

1. Call `loom_next_step`.
2. Execute exactly one GitHub-side action.
3. Call `loom_checkpoint` with the canonical action.
4. Call `loom_heartbeat` while waiting on async gates.
5. Abort safely if live GitHub state and Loom checkpoint state diverge.

---

## Repository Settings

### 1. Enable Copilot Coding Agent

On GitHub.com -> repository -> **Settings** -> **Copilot** -> **Coding agent**:

- Enable Copilot to create pull requests.
- Keep branch protection enabled so merges still require review and green checks.

### 2. Enable Copilot PR Reviews

On GitHub.com -> repository -> **Settings** -> **Copilot** -> **Pull request reviews**:

- Enable Copilot PR reviews.
- Prefer automatic review availability where GitHub supports it.
- Keep the manual **Request Copilot review** button as the fallback path when Loom is not operating the PR.

### 3. Ensure PR CI Exists

`/run-loom` cannot reach review until PR checks exist and turn green. A PR with no check runs is still blocked at `AWAITING_CI`, even if the draft flag has been cleared.

At minimum, ensure the repository has a workflow triggered on `pull_request` and that the PR branch is actually eligible to start checks.

### 4. Configure MCP Servers

For Loom's own operator workflow, `.github/copilot/mcp.json` must include the `loom` MCP server. Optional MCP servers can be added for implementation agents, but they are not substitutes for the Loom server.

---

## Canonical Lifecycle

### Manual GitHub-Native Flow

Use this when a human is driving GitHub directly without `/run-loom`:

1. Open or select an issue whose body contains the relevant planning context.
2. Assign the issue to `@copilot`.
3. Wait for `@copilot` to open a PR.
4. Wait for CI to pass.
5. Request Copilot review on the PR.
6. Read the review, address blockers, and re-request review if needed.
7. Approve and merge.

### `/run-loom` Automated Flow

Use this when Loom is operating server-side weaving:

1. `ISSUE_CREATED` -> Loom identifies or creates the issue and assigns `@copilot`.
2. `AWAITING_PR` -> Loom waits for the PR to appear.
3. `AWAITING_READY` -> Loom promotes the draft PR when coding is complete.
4. `AWAITING_CI` -> Loom waits for CI to produce a terminal result.
5. `REVIEWING` -> Loom requests Copilot review and waits for approval or changes requested.
6. `ADDRESSING_FEEDBACK` -> Loom waits for feedback to be addressed, then returns to CI/review as needed.
7. `MERGING` -> Loom merges the approved PR.

The operator must not skip from `AWAITING_CI` to merge-ready behavior. `REVIEWING` only begins after `ci_green`.

---

## Review Gate Semantics

### In `AWAITING_CI`

The operator should:

- poll GitHub for check runs or statuses
- checkpoint `ci_green` when checks pass
- checkpoint `ci_red` when checks fail

The operator should **not** request review while checks are still pending or absent.

### In `REVIEWING`

The operator should:

1. Ensure a review is actually requested on the PR.
2. If no review has been requested yet, request Copilot review.
3. Poll for review outcome.
4. Checkpoint `review_approved` or `review_changes_requested`.

Manual button usage is a fallback for a human-driven GitHub session. In `/run-loom`, requesting review is part of Loom's job.

### Human Approval Still Matters

Loom automates workflow progression, but branch protection should still require a real maintainer approval before merge if that is the repository policy. The product intent is to remove glue work, not to weaken release controls.

---

## Expected Operator Behavior in `/run-loom`

When Loom reaches each state, the expected GitHub-side action is:

| Loom state | Expected operator action |
|---|---|
| `ISSUE_CREATED` | Assign `@copilot` to the issue |
| `AWAITING_PR` | Find the PR opened by `@copilot` |
| `AWAITING_READY` | Mark the PR ready for review if coding is complete and the PR is still draft |
| `AWAITING_CI` | Wait for CI and classify result as green or red |
| `REVIEWING` | Request Copilot review if missing, then wait for review verdict |
| `ADDRESSING_FEEDBACK` | Wait for follow-up commits that address requested changes |
| `MERGING` | Merge the approved PR |

If Loom is in `REVIEWING` and no review was ever requested, that is an operator gap or implementation bug, not intended behavior.

---

## Fallbacks and Handoffs

Use these fallbacks only when the ideal automated path cannot be executed:

1. If Loom MCP is unavailable, stop and restore the Loom server before taking workflow actions.
2. If review could not be requested automatically but the PR is already in `REVIEWING`, request Copilot review manually and reconcile the checkpoint.
3. If live GitHub state and Loom checkpoint state diverge and cannot be reconciled safely, abort and hand off with a concise state summary.
4. If CI never starts, treat that as an `AWAITING_CI` infrastructure problem, not as implicit review readiness.

---

## Guidance for Issue and PR Authors

### Issue Bodies

Issue bodies are still the prompt surface for the GitHub coding agent. They should point to the canonical planning artifacts in this repository:

- relevant theme and epic docs under `docs/themes/`
- backlog context in `docs/plan/backlog.yaml` when sequencing matters
- architecture and ADR docs when implementation constraints matter

### PR Descriptions

PR descriptions should state:

- what changed
- which issue or story is being closed
- what tests or checks were run
- any constraints a reviewer should pay attention to

### Review Comments

Copilot review comments should be treated as inputs to the feedback loop, not as automatic merge authority. Loom or the human operator must still translate the final review outcome into the correct next workflow step.

---

## Anti-Gaps

If this document ever disagrees with Loom's vision or operator prompts, fix this file instead of teaching operators a one-off exception.

Common mistakes this file must avoid:

- describing `/run-loom` review as a manual button-click step
- implying the local `@reviewer` agent is assigned during GitHub PR weaving
- treating missing CI as equivalent to green CI
- using stale product-specific examples from another repository
