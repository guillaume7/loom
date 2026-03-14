---
id: TH2.E4.US1
title: "Orchestrator agent definition"
type: standard
priority: high
size: M
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "loom-orchestrator.agent.md exists in .github/agents/ with target: vscode"
  - AC2: "Agent has tools list including all 5 Loom MCP tools and github-mcp-server default"
  - AC3: "Agent declares handoff transitions to gate, debug, merge, and ask agents"
  - AC4: "Instructions reference the loom-mcp-loop skill for operational contract"
depends-on: []
---

# TH2.E4.US1 — Orchestrator Agent Definition

**As a** Loom operator, **I want** a `loom-orchestrator.agent.md` custom agent definition, **so that** the orchestration loop runs as a structured agent with constrained tools and handoff transitions.

## Acceptance Criteria

- [ ] AC1: `loom-orchestrator.agent.md` exists in `.github/agents/` with `target: vscode`
- [ ] AC2: Agent has tools list including all 5 Loom MCP tools and `github-mcp-server` default
- [ ] AC3: Agent declares handoff transitions to gate, debug, merge, and ask agents
- [ ] AC4: Instructions reference the loom-mcp-loop skill for operational contract

## BDD Scenarios

### Scenario: Agent file is valid custom agent format
- **Given** the file `.github/agents/loom-orchestrator.agent.md`
- **When** VS Code parses the agent definition
- **Then** it recognizes name, description, target, tools, and handoffs fields

### Scenario: Tools are correctly scoped
- **Given** the orchestrator agent definition
- **When** the tools list is inspected
- **Then** it includes `loom/loom_next_step`, `loom/loom_checkpoint`, `loom/loom_heartbeat`, `loom/loom_get_state`, `loom/loom_abort`
- **And** it includes `github/github-mcp-server/default`

### Scenario: Handoff to gate agent
- **Given** the orchestrator agent definition
- **When** the handoffs section is inspected
- **Then** there is a handoff labeled "Evaluate gate" targeting `loom-gate`
- **And** the prompt template includes `${pr_number}`

### Scenario: Handoff to debug agent on failure
- **Given** the orchestrator agent definition
- **When** a CI failure is detected
- **Then** the handoff labeled "Debug CI failure" targets `loom-debug`
