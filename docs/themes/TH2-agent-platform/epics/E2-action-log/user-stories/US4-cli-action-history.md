---
id: TH2.E2.US4
title: "loom log CLI shows action history"
type: standard
priority: medium
size: S
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "loom log reads from action_log table instead of or in addition to existing log source"
  - AC2: "Output includes timestamp, operation_key, state transition, and event for each entry"
  - AC3: "Default limit is 50 entries; --limit flag overrides"
depends-on: [TH2.E2.US2]
---

# TH2.E2.US4 — loom log CLI Shows Action History

**As a** human operator, **I want** `loom log` to display structured action history from the database, **so that** I can audit what Loom has done without reading raw SQLite.

## Acceptance Criteria

- [ ] AC1: `loom log` reads from `action_log` table instead of or in addition to existing log source
- [ ] AC2: Output includes timestamp, operation_key, state transition, and event for each entry
- [ ] AC3: Default limit is 50 entries; `--limit` flag overrides

## BDD Scenarios

### Scenario: Display recent actions
- **Given** an action_log with 5 entries
- **When** `loom log` is executed
- **Then** all 5 entries are displayed in reverse chronological order
- **And** each line shows created_at, operation_key, state_before → state_after, event

### Scenario: Limit output
- **Given** an action_log with 100 entries
- **When** `loom log --limit 10` is executed
- **Then** exactly 10 entries are displayed (most recent)

### Scenario: Empty action log
- **Given** an empty action_log table
- **When** `loom log` is executed
- **Then** a message "No actions recorded" is displayed (not an error)
