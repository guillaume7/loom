---
id: TH2.E7.US2
title: "MCP tool readOnlyHint annotations"
type: standard
priority: medium
size: S
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "loom_get_state and loom_heartbeat have readOnlyHint: true"
  - AC2: "loom_next_step, loom_checkpoint, and loom_abort have readOnlyHint: false"
  - AC3: "Annotations are set in tool registration, not in handler logic"
depends-on: []
---

# TH2.E7.US2 — MCP Tool readOnlyHint Annotations

**As a** VS Code enterprise admin, **I want** Loom MCP tools to carry correct `readOnlyHint` values, **so that** auto-approval policies can distinguish read-only from write operations.

## Acceptance Criteria

- [ ] AC1: `loom_get_state` and `loom_heartbeat` have `readOnlyHint: true`
- [ ] AC2: `loom_next_step`, `loom_checkpoint`, and `loom_abort` have `readOnlyHint: false`
- [ ] AC3: Annotations are set in tool registration, not in handler logic

## BDD Scenarios

### Scenario: Read-only tools are annotated
- **Given** the MCP server is initialized
- **When** `tools/list` is called
- **Then** `loom_get_state` has annotation `readOnlyHint: true`
- **And** `loom_heartbeat` has annotation `readOnlyHint: true`

### Scenario: Write tools are annotated
- **Given** the MCP server is initialized
- **When** `tools/list` is called
- **Then** `loom_next_step` has annotation `readOnlyHint: false`
- **And** `loom_checkpoint` has annotation `readOnlyHint: false`
- **And** `loom_abort` has annotation `readOnlyHint: false`

### Scenario: Auto-approve respects annotations
- **Given** a VS Code client with auto-approve enabled for read-only tools
- **When** `loom_heartbeat` is called
- **Then** no human approval prompt is shown
