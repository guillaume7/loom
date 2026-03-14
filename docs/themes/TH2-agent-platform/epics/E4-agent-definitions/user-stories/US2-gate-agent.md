---
id: TH2.E4.US2
title: "Gate agent definition"
type: standard
priority: high
size: S
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "loom-gate.agent.md exists in .github/agents/ with target: vscode"
  - AC2: "Agent has ONLY read-only tools — no editFiles, no shell, no write operations"
  - AC3: "Instructions specify structured PASS/FAIL JSON return format"
  - AC4: "Agent checks: CI green, approved review, not draft, no merge conflicts, deps resolved"
depends-on: []
---

# TH2.E4.US2 — Gate Agent Definition

**As a** Loom orchestrator agent, **I want** a `loom-gate.agent.md` subagent that evaluates merge readiness with read-only tools, **so that** gate decisions are isolated from the main context and constrained to observation only.

## Acceptance Criteria

- [ ] AC1: `loom-gate.agent.md` exists in `.github/agents/` with `target: vscode`
- [ ] AC2: Agent has ONLY read-only tools — no `editFiles`, no `shell`, no write operations
- [ ] AC3: Instructions specify structured `{"verdict":"PASS"|"FAIL","reason":"..."}` return format
- [ ] AC4: Agent checks: CI green, approved review, not draft, no merge conflicts, deps resolved

## BDD Scenarios

### Scenario: Gate agent has read-only tools only
- **Given** the file `.github/agents/loom-gate.agent.md`
- **When** the tools list is inspected
- **Then** it includes only `search/codebase`, `github/.../pull_request_read`, `github/.../get_commit`, `loom/loom_get_state`
- **And** no write-capable tools are present

### Scenario: Gate returns PASS verdict
- **Given** a PR with all CI checks green, one approved review, not a draft, no conflicts
- **When** the gate agent evaluates the PR
- **Then** the response is `{"verdict":"PASS","reason":"All checks green, review approved"}`

### Scenario: Gate returns FAIL verdict
- **Given** a PR with a failing CI check
- **When** the gate agent evaluates the PR
- **Then** the response is `{"verdict":"FAIL","reason":"CI check 'build' is red"}`
