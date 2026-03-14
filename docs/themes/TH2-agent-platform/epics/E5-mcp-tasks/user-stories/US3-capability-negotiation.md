---
id: TH2.E5.US3
title: "Client capability negotiation and fallback"
type: standard
priority: high
size: M
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "MCP server checks client capabilities for Task support during initialization"
  - AC2: "If client supports Tasks, loom_heartbeat uses Task lifecycle"
  - AC3: "If client does NOT support Tasks, loom_heartbeat falls back to blocking behavior"
  - AC4: "Fallback behavior is identical to v1 loom_heartbeat"
depends-on: [TH2.E5.US1]
---

# TH2.E5.US3 — Client Capability Negotiation and Fallback

**As a** Loom MCP server, **I want** to check whether the connected client supports MCP Tasks, **so that** I fall back to blocking behavior for older clients.

## Acceptance Criteria

- [ ] AC1: MCP server checks client capabilities for Task support during initialization
- [ ] AC2: If client supports Tasks, `loom_heartbeat` uses Task lifecycle
- [ ] AC3: If client does NOT support Tasks, `loom_heartbeat` falls back to blocking behavior
- [ ] AC4: Fallback behavior is identical to v1 `loom_heartbeat`

## BDD Scenarios

### Scenario: Client supports Tasks
- **Given** an MCP client that advertises Task capability
- **When** `loom_heartbeat` enters a polling loop
- **Then** Task lifecycle events are emitted

### Scenario: Client does not support Tasks
- **Given** an MCP client that does NOT advertise Task capability
- **When** `loom_heartbeat` enters a polling loop
- **Then** the tool blocks until polling completes (v1 behavior)
- **And** no task events are emitted

### Scenario: Capability check runs once at init
- **Given** an MCP server starting up
- **When** the client connects and capabilities are exchanged
- **Then** the Task support flag is cached for the session lifetime
- **And** no per-tool-call re-negotiation occurs
