---
id: TH3.E5.US3
title: "Agent failure containment"
type: standard
priority: medium
size: M
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "Agent timeout, crash, or malformed output resolves to explicit runtime outcomes instead of indefinite stalls"
  - AC2: "Failed jobs do not silently hold run leases or PR locks forever"
  - AC3: "Failure handling preserves enough context for operator review and replay"
depends-on: [TH3.E5.US2, TH3.E4.US4]
---

# TH3.E5.US3 — Agent Failure Containment

**As a** Loom operator, **I want** agent failures contained, **so that** a bad worker result does not stall or corrupt the whole run.

## Acceptance Criteria

- [ ] AC1: Agent timeout, crash, or malformed output resolves to explicit runtime outcomes instead of indefinite stalls
- [ ] AC2: Failed jobs do not silently hold run leases or PR locks forever
- [ ] AC3: Failure handling preserves enough context for operator review and replay

## BDD Scenarios

### Scenario: Agent timeout resolves explicitly
- **Given** an agent job exceeds its deadline
- **When** the runtime handles the timeout
- **Then** it returns an explicit outcome such as retry, escalate, or fail-safe block

### Scenario: Failed job releases runtime ownership correctly
- **Given** an agent job fails while the run is active
- **When** the runtime records the failure
- **Then** any lease or lock handling is left in a recoverable state

### Scenario: Failure context is preserved for debugging
- **Given** a job returned malformed or partial output
- **When** the operator inspects the run later
- **Then** the relevant failure context is still available for diagnosis or replay
