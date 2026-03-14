---
id: TH2.E2.US2
title: "WriteAction and ReadActions store methods"
type: standard
priority: high
size: M
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "WriteAction inserts a row into action_log with all required fields"
  - AC2: "WriteAction with a duplicate operation_key returns a specific error (not a panic)"
  - AC3: "ReadActions(limit) returns the most recent N actions ordered by created_at desc"
  - AC4: "ReadActions with limit 0 returns an empty slice"
depends-on: [TH2.E2.US1]
---

# TH2.E2.US2 — WriteAction and ReadActions Store Methods

**As a** Loom MCP server, **I want** store methods to write and read action log entries, **so that** tool handlers can record actions and resources can serve the log.

## Acceptance Criteria

- [ ] AC1: `WriteAction` inserts a row into `action_log` with all required fields
- [ ] AC2: `WriteAction` with a duplicate `operation_key` returns a specific error (not a panic)
- [ ] AC3: `ReadActions(limit)` returns the most recent N actions ordered by `created_at` desc
- [ ] AC4: `ReadActions` with limit 0 returns an empty slice

## BDD Scenarios

### Scenario: Write a new action
- **Given** an empty action_log table
- **When** `WriteAction` is called with session_id "s1", operation_key "create_issue:E2-US1", state_before "SCANNING", state_after "ISSUE_CREATED", event "issue_created", detail `{"issue":42}`
- **Then** the row is inserted successfully
- **And** `ReadActions(1)` returns exactly that action

### Scenario: Reject duplicate operation key
- **Given** an action_log containing an entry with operation_key "create_issue:E2-US1"
- **When** `WriteAction` is called with the same operation_key
- **Then** an error is returned indicating a duplicate key
- **And** the original row is unchanged

### Scenario: Read actions with limit
- **Given** an action_log with 10 entries
- **When** `ReadActions(3)` is called
- **Then** exactly 3 actions are returned
- **And** they are the 3 most recent by `created_at`

### Scenario: Read actions from empty table
- **Given** an empty action_log table
- **When** `ReadActions(10)` is called
- **Then** an empty slice is returned (not nil)

### Scenario: Read actions with zero limit
- **Given** an action_log with entries
- **When** `ReadActions(0)` is called
- **Then** an empty slice is returned
