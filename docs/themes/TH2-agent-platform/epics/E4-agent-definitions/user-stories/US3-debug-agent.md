---
id: TH2.E4.US3
title: "Debug agent definition"
type: standard
priority: high
size: S
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "loom-debug.agent.md exists in .github/agents/ with target: vscode"
  - AC2: "Agent has read tools plus add_issue_comment — no merge, no editFiles"
  - AC3: "Instructions specify structured debug comment format"
  - AC4: "Agent returns {action: commented, comment_id: N} when done"
depends-on: []
---

# TH2.E4.US3 — Debug Agent Definition

**As a** Loom orchestrator agent, **I want** a `loom-debug.agent.md` agent that analyzes CI failures and posts debug comments, **so that** failure diagnosis is handled by a focused agent with constrained tools.

## Acceptance Criteria

- [ ] AC1: `loom-debug.agent.md` exists in `.github/agents/` with `target: vscode`
- [ ] AC2: Agent has read tools plus `add_issue_comment` — no merge, no `editFiles`
- [ ] AC3: Instructions specify structured debug comment format
- [ ] AC4: Agent returns `{"action":"commented","comment_id":N}` when done

## BDD Scenarios

### Scenario: Debug agent has correct tool set
- **Given** the file `.github/agents/loom-debug.agent.md`
- **When** the tools list is inspected
- **Then** it includes `github/.../pull_request_read`, `github/.../get_commit`, `github/.../add_issue_comment`, `search/codebase`
- **And** it does NOT include merge or file-editing tools

### Scenario: Debug agent posts analysis comment
- **Given** a PR with a failing check run
- **When** the debug agent is invoked with the PR number and run ID
- **Then** a structured debug comment is posted on the PR
- **And** the agent returns `{"action":"commented","comment_id":...}`

### Scenario: Debug agent does not modify files
- **Given** the debug agent instructions
- **When** a user asks the debug agent to fix the code
- **Then** the agent refuses and instructs to use a different agent
