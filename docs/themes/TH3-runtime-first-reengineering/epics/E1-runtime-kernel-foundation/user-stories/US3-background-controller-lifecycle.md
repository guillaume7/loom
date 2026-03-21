---
id: TH3.E1.US3
title: "Background controller lifecycle"
type: standard
priority: high
size: M
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "The runtime lifecycle defines start, claim, sleep, wake, resume, and shutdown behavior"
  - AC2: "Controller steps are driven by persisted runtime state rather than an active session heartbeat"
  - AC3: "The lifecycle specifies how CLI and MCP surfaces observe and interact with the active controller"
depends-on: [TH3.E1.US2]
---

# TH3.E1.US3 — Background Controller Lifecycle

**As a** Loom operator, **I want** a defined controller lifecycle, **so that** I know how the runtime progresses, sleeps, resumes, and shuts down safely.

## Acceptance Criteria

- [ ] AC1: The runtime lifecycle defines start, claim, sleep, wake, resume, and shutdown behavior
- [ ] AC2: Controller steps are driven by persisted runtime state rather than an active session heartbeat
- [ ] AC3: The lifecycle specifies how CLI and MCP surfaces observe and interact with the active controller

## BDD Scenarios

### Scenario: Controller starts and claims work from persisted state
- **Given** Loom is launched with an existing state database
- **When** the controller starts
- **Then** it determines whether work is due from persisted state
- **And** it claims only the work it is allowed to own

### Scenario: Controller sleeps until the next due wake-up
- **Given** there is no immediate work to execute
- **When** the controller evaluates the wake schedule
- **Then** it records the next wake condition
- **And** it can resume without an interactive chat session

### Scenario: Operator surfaces inspect controller progress
- **Given** the controller is active
- **When** the operator uses CLI or MCP state inspection
- **Then** the runtime lifecycle state is visible without reconstructing it from prompt history
