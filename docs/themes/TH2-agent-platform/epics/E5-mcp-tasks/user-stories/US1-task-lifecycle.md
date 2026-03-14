---
id: TH2.E5.US1
title: "MCP Task lifecycle event emission"
type: standard
priority: high
size: M
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "MCP server can emit task/start events with id, title, and cancellable flag"
  - AC2: "MCP server can emit task/progress events with progress text"
  - AC3: "MCP server can emit task/done events with structured result"
  - AC4: "Task events are sent via the MCP stdio transport"
depends-on: []
---

# TH2.E5.US1 — MCP Task Lifecycle Event Emission

**As a** Loom MCP server developer, **I want** to emit MCP Task lifecycle events, **so that** long-running operations can report progress and survive client disconnects.

## Acceptance Criteria

- [ ] AC1: MCP server can emit `task/start` events with `id`, `title`, and `cancellable` flag
- [ ] AC2: MCP server can emit `task/progress` events with progress text
- [ ] AC3: MCP server can emit `task/done` events with structured result
- [ ] AC4: Task events are sent via the MCP stdio transport

## BDD Scenarios

### Scenario: Emit task start
- **Given** a long-running MCP tool handler
- **When** the handler begins polling
- **Then** a `task/start` event is emitted with a unique task ID and title

### Scenario: Emit task progress
- **Given** an active MCP task with ID "loom-ci-poll-pr-42"
- **When** a polling interval completes with intermediate results
- **Then** a `task/progress` event is emitted with the same task ID and progress description

### Scenario: Emit task done
- **Given** an active MCP task with ID "loom-ci-poll-pr-42"
- **When** polling completes successfully
- **Then** a `task/done` event is emitted with the result payload

### Scenario: Task events are valid JSON
- **Given** any task event emission
- **When** the event is captured from stdio
- **Then** the event is valid JSON conforming to the MCP Task spec
