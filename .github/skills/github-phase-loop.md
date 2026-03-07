---
description: >
  Drives the full Loom phase-delivery loop using GitHub MCP server tools.
  One Orchestrator session can advance a phase from issue creation through
  merge — then iterate to the next phase until all epics are complete.
applyTo: "**"
---

# Skill: GitHub Phase Delivery Loop

This skill equips the **Orchestrator** with step-by-step instructions for running the complete Loom development loop via the GitHub MCP tools (`mcp_io_github_git_*`).

> **Automation boundary**: Steps that depend on another agent (Copilot coding, CI, Reviewer) are asynchronous. Poll until completion. If the session ends before a gate clears, re-invoke the Orchestrator with this skill — it will resume from the current state by reading live GitHub data.

---

## Repository Identity

- **owner**: `guillaume7`
- **repo**: `loom`

---

## Phase Sequence

| Phase | Epic | Branch Pattern | Primary Agents |
|---|---|---|---|
| 0 | E1 Foundation | `phase/0-foundation` | devops-release, backend-developer |
| 1 | E2 State Machine | `phase/1-fsm` | architect, backend-developer |
| 2 | E3 GitHub Client | `phase/2-github-client` | backend-developer, test-engineer |
| 3 | E4 MCP Server | `phase/3-mcp-server` | backend-developer |
| 4 | E5 CLI | `phase/4-cli` | backend-developer |
| 5 | E6 Session Management | `phase/5-session` | backend-developer |
| 6 | E7 Checkpointing | `phase/6-checkpointing` | backend-developer |
| 7 | E8 Integration | `phase/7-integration` | test-engineer, debugger |

---

## The Loop (repeat for each phase)

### Step 1 — Determine current phase

1. Read each epic file, check which acceptance criteria are ticked.
2. List open PRs and issues to find which phase is in flight.
3. Identify the lowest-numbered phase whose preconditions are met and branch is not yet merged.
4. If all phases merged → **project complete**.

### Step 2 — Create the phase issue

Read the appropriate template from `.github/ISSUE_TEMPLATE/` and create the issue.

### Step 3 — Assign @copilot to the issue

Use `mcp_io_github_git_assign_copilot_to_issue`. Poll for a matching PR every 30 seconds. After 10 minutes without a PR, nudge with an issue comment.

### Step 4 — Wait for implementation

Poll until `draft: false`. If no progress after 30 minutes, nudge with a comment.

### Step 5 — Confirm CI green

Check CI check-runs on the PR. If red, create a debug sub-issue and assign `@copilot`.

### Step 6 — Request Copilot review

```
@copilot Please review this PR as the Reviewer agent.

Agent: .github/agents/reviewer.md
Skills:
- .github/skills/review-checklist.md
- .github/skills/loom-architecture.md
- .github/skills/go-standards.md

Run automated gates first:
  go vet ./...
  golangci-lint run
  go test ./... -race -cover
  go build ./cmd/loom

Then review code quality, architecture, and FSM correctness.
Leave inline comments for every issue found.
Submit a formal review (APPROVED or CHANGES_REQUESTED).
```

Poll for a submitted review.

### Step 7 — Handle review feedback

- `APPROVED` → proceed to merge
- `CHANGES_REQUESTED` → extract BLOCKERs, post as comment tagging `@copilot`, wait for fix push, loop back to CI check

### Step 8 — Merge the PR

Use `mcp_io_github_git_merge_pull_request` (squash). Close the issue.

### Step 9 — Post-epic refactor (at epic boundaries)

Create a refactor issue:

```
@copilot Please perform a refactor sweep of the work just merged.

Agent: .github/agents/refactoring-agent.md
Skills:
- .github/skills/go-standards.md
- .github/skills/review-checklist.md

Confirm go test ./... passes first.
Scan for: magic numbers, functions > 50 lines, missing interface injections,
time.Sleep in tests, global mutable state.
Fix each smell, re-run tests after each change, commit per smell.
Open a PR titled: refactor: [area] post-epic-E{N} cleanup
```

### Step 10 — Advance to next phase

Return to Step 1.
