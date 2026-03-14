---
id: TH2.E3.US4
title: "loom://log MCP resource"
type: standard
priority: medium
size: M
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "Resource is registered at URI loom://log"
  - AC2: "Reading the resource returns NDJSON of the last 200 action_log entries"
  - AC3: "Each line is a valid JSON object matching the log schema from data-model.md"
  - AC4: "Empty action_log returns empty content (not an error)"
depends-on: [TH2.E3.US1]
---

# TH2.E3.US4 — loom://log MCP Resource

**As a** Loom agent, **I want** to read the action log via `loom://log`, **so that** I can review Loom's history for debugging and decision-making.

## Acceptance Criteria

- [ ] AC1: Resource is registered at URI `loom://log`
- [ ] AC2: Reading the resource returns NDJSON of the last 200 `action_log` entries
- [ ] AC3: Each line is a valid JSON object matching the log schema from data-model.md
- [ ] AC4: Empty action_log returns empty content (not an error)

## BDD Scenarios

### Scenario: Read action log
- **Given** an action_log with 5 entries
- **When** `resources/read` is called for "loom://log"
- **Then** the response contains 5 newline-delimited JSON objects
- **And** each object has fields: id, session_id, operation_key, state_before, state_after, event, detail, created_at

### Scenario: Limit to 200 entries
- **Given** an action_log with 300 entries
- **When** `resources/read` is called for "loom://log"
- **Then** exactly 200 entries are returned (the most recent)

### Scenario: Empty action log
- **Given** an empty action_log table
- **When** `resources/read` is called for "loom://log"
- **Then** an empty response is returned (not an error)

### Scenario: Log entries are chronologically ordered
- **Given** an action_log with entries created at T1, T2, T3
- **When** `resources/read` is called for "loom://log"
- **Then** entries are ordered from oldest to newest (T1, T2, T3)
