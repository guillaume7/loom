---
id: TH2.E6.US3
title: "Elicitation response handler with fallback"
type: standard
priority: medium
size: M
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "Operator response 'skip' fires skip_story FSM event"
  - AC2: "Operator response 'reassign' closes current PR and resets to ISSUE_CREATED"
  - AC3: "Operator response 'pause_epic' transitions to PAUSED (existing behavior)"
  - AC4: "If client does not support elicitation, PAUSED transition occurs immediately (v1 fallback)"
  - AC5: "Invalid response values are rejected with a clear error"
depends-on: [TH2.E6.US1, TH2.E6.US2]
---

# TH2.E6.US3 — Elicitation Response Handler with Fallback

**As a** Loom MCP server, **I want** to map operator elicitation responses to FSM events, **so that** the workflow can resume based on the operator's choice.

## Acceptance Criteria

- [ ] AC1: Operator response `skip` fires `skip_story` FSM event
- [ ] AC2: Operator response `reassign` closes current PR and resets to `ISSUE_CREATED`
- [ ] AC3: Operator response `pause_epic` transitions to `PAUSED` (existing behavior)
- [ ] AC4: If client does not support elicitation, `PAUSED` transition occurs immediately (v1 fallback)
- [ ] AC5: Invalid response values are rejected with a clear error

## BDD Scenarios

### Scenario: Handle skip response
- **Given** an active elicitation for PR #42
- **When** the operator responds with `{"action":"skip"}`
- **Then** the FSM event `skip_story` is fired
- **And** the checkpoint records the skip

### Scenario: Handle reassign response
- **Given** an active elicitation for PR #42
- **When** the operator responds with `{"action":"reassign"}`
- **Then** PR #42 is closed
- **And** the FSM transitions to `ISSUE_CREATED`
- **And** a new issue is created for the story

### Scenario: Handle pause_epic response
- **Given** an active elicitation for PR #42
- **When** the operator responds with `{"action":"pause_epic"}`
- **Then** the FSM transitions to `PAUSED`

### Scenario: Fallback to PAUSED without elicitation support
- **Given** an MCP client that does NOT support elicitation
- **When** budget is exhausted
- **Then** the FSM transitions directly to `PAUSED` (v1 behavior)
- **And** a log message indicates elicitation was unavailable

### Scenario: Reject invalid response
- **Given** an active elicitation
- **When** the operator responds with `{"action":"invalid_action"}`
- **Then** an error is returned listing valid choices
- **And** the elicitation remains active
