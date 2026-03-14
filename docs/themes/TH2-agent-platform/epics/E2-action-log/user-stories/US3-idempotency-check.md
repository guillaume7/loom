---
id: TH2.E2.US3
title: "Idempotency check in MCP tool handlers"
type: standard
priority: high
size: M
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "Before executing a write operation, the MCP handler checks action_log for existing operation_key"
  - AC2: "If operation_key exists, the handler returns the cached result without re-executing"
  - AC3: "If operation_key does not exist, the handler executes and logs the action"
  - AC4: "loom_checkpoint and loom_next_step both enforce idempotency"
depends-on: [TH2.E2.US2]
---

# TH2.E2.US3 — Idempotency Check in MCP Tool Handlers

**As a** Loom operator, **I want** MCP write operations to be idempotent, **so that** retrying a tool call after a disconnect does not cause duplicate actions on GitHub.

## Acceptance Criteria

- [ ] AC1: Before executing a write operation, the MCP handler checks `action_log` for existing `operation_key`
- [ ] AC2: If `operation_key` exists, the handler returns the cached result without re-executing
- [ ] AC3: If `operation_key` does not exist, the handler executes and logs the action
- [ ] AC4: `loom_checkpoint` and `loom_next_step` both enforce idempotency

## BDD Scenarios

### Scenario: First execution logs action and proceeds
- **Given** no action_log entry for operation_key "checkpoint:SCANNING→ISSUE_CREATED"
- **When** `loom_checkpoint` is called with that transition
- **Then** the FSM transition is executed
- **And** a new action_log entry is written with the operation_key

### Scenario: Retry returns cached result
- **Given** an action_log entry exists for operation_key "checkpoint:SCANNING→ISSUE_CREATED" with detail `{"pr":42}`
- **When** `loom_checkpoint` is called with the same transition
- **Then** the FSM transition is NOT re-executed
- **And** the cached result `{"pr":42}` is returned

### Scenario: Different operation key proceeds normally
- **Given** an action_log entry exists for "checkpoint:SCANNING→ISSUE_CREATED"
- **When** `loom_checkpoint` is called with operation_key "checkpoint:ISSUE_CREATED→AWAITING_PR"
- **Then** the new transition is executed normally
- **And** a new action_log entry is created

### Scenario: Read-only tools skip idempotency check
- **Given** any state of the action_log
- **When** `loom_get_state` or `loom_heartbeat` is called
- **Then** no action_log lookup is performed
- **And** the current state is returned directly
