---
id: TH2.E3.US5
title: "MCP server instructions with phase summary and dependency digest"
type: standard
priority: medium
size: S
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "MCP server's instructions field includes a one-paragraph phase summary"
  - AC2: "Instructions include a dependency digest listing blocked and unblocked stories"
  - AC3: "Instructions are refreshed on each server initialization"
depends-on: [TH2.E3.US1]
---

# TH2.E3.US5 — MCP Server Instructions with Phase Summary and Dependency Digest

**As a** Loom agent session, **I want** the MCP server to inject phase and dependency context into my base prompt, **so that** I have immediate awareness of the current workflow state.

## Acceptance Criteria

- [ ] AC1: MCP server's `instructions` field includes a one-paragraph phase summary
- [ ] AC2: Instructions include a dependency digest listing blocked and unblocked stories
- [ ] AC3: Instructions are refreshed on each server initialization

## BDD Scenarios

### Scenario: Server instructions include phase summary
- **Given** the FSM is in state "AWAITING_CI" at phase 3
- **When** the MCP server starts and reports its instructions
- **Then** the instructions contain a sentence describing the current phase and state

### Scenario: Server instructions include dependency digest
- **Given** a `.loom/dependencies.yaml` with 2 unblocked and 3 blocked stories
- **When** the MCP server starts
- **Then** the instructions list the unblocked story IDs and the count of blocked stories

### Scenario: No dependency file available
- **Given** no `.loom/dependencies.yaml` exists
- **When** the MCP server starts
- **Then** the instructions include the phase summary
- **And** the dependency section states "No dependency graph loaded"
