---
id: TH2.E3.US3
title: "loom://state MCP resource"
type: standard
priority: medium
size: M
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "Resource is registered at URI loom://state"
  - AC2: "Reading the resource returns JSON with state, phase, pr_number, issue_number, retry_count, updated_at"
  - AC3: "JSON includes unblocked_stories array from depgraph evaluation"
  - AC4: "Content type is application/json"
depends-on: [TH2.E3.US1]
---

# TH2.E3.US3 — loom://state MCP Resource

**As a** Loom agent, **I want** to read the current FSM state via `loom://state`, **so that** I have immediate context about the workflow without calling a tool.

## Acceptance Criteria

- [ ] AC1: Resource is registered at URI `loom://state`
- [ ] AC2: Reading the resource returns JSON with `state`, `phase`, `pr_number`, `issue_number`, `retry_count`, `updated_at`
- [ ] AC3: JSON includes `unblocked_stories` array from depgraph evaluation
- [ ] AC4: Content type is `application/json`

## BDD Scenarios

### Scenario: Read current state
- **Given** the FSM is in state "AWAITING_CI" with pr_number 42 and retry_count 1
- **When** `resources/read` is called for "loom://state"
- **Then** the response is JSON containing `{"state":"AWAITING_CI","pr_number":42,"retry_count":1,...}`

### Scenario: State includes unblocked stories
- **Given** the FSM is in state "SCANNING" and depgraph has 2 unblocked stories
- **When** `resources/read` is called for "loom://state"
- **Then** the JSON response includes `"unblocked_stories":["US-2.1","US-2.2"]`

### Scenario: No checkpoint exists
- **Given** a fresh database with no checkpoint row
- **When** `resources/read` is called for "loom://state"
- **Then** the response returns a default state JSON with state "IDLE" and zero values

### Scenario: Depgraph unavailable
- **Given** no `.loom/dependencies.yaml` exists
- **When** `resources/read` is called for "loom://state"
- **Then** the JSON response has `"unblocked_stories":[]` (empty, not an error)
