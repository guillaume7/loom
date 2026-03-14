---
id: TH2.E4.US4
title: "Merge agent definition"
type: standard
priority: high
size: S
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "loom-merge.agent.md exists in .github/agents/ with target: vscode"
  - AC2: "Agent has ONLY merge-related tools — no issue creation, no commenting"
  - AC3: "Instructions specify merge-only behavior with branch protection compliance"
depends-on: []
---

# TH2.E4.US4 — Merge Agent Definition

**As a** Loom orchestrator agent, **I want** a `loom-merge.agent.md` agent that executes merge operations only, **so that** the merge step is isolated and auditable.

## Acceptance Criteria

- [ ] AC1: `loom-merge.agent.md` exists in `.github/agents/` with `target: vscode`
- [ ] AC2: Agent has ONLY merge-related tools — no issue creation, no commenting
- [ ] AC3: Instructions specify merge-only behavior with branch protection compliance

## BDD Scenarios

### Scenario: Merge agent has minimal tool set
- **Given** the file `.github/agents/loom-merge.agent.md`
- **When** the tools list is inspected
- **Then** it includes `github/.../merge_pull_request`
- **And** no other write tools are present

### Scenario: Merge agent executes merge
- **Given** a PR that has passed the gate evaluation
- **When** the merge agent is invoked with the PR number
- **Then** the PR is merged
- **And** the agent returns `{"action":"merged","pr":N}`

### Scenario: Merge agent respects branch protection
- **Given** a PR that does not satisfy branch protection rules
- **When** the merge agent attempts to merge
- **Then** the merge fails with a clear error from the GitHub API
- **And** the agent does NOT force-push or bypass protections
