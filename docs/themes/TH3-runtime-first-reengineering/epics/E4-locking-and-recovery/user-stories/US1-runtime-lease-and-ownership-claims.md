---
id: TH3.E4.US1
title: "Runtime lease and ownership claims"
type: standard
priority: high
size: M
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "The runtime defines a lease or ownership record for active run control"
  - AC2: "Only one controller instance can hold the active lease for a run at a time"
  - AC3: "Lease ownership is visible to operators and included in diagnostics"
depends-on: [TH3.E1.US2]
---

# TH3.E4.US1 — Runtime Lease and Ownership Claims

**As a** Loom runtime, **I want** an explicit ownership lease, **so that** only one controller can advance a run at a time.

## Acceptance Criteria

- [ ] AC1: The runtime defines a lease or ownership record for active run control
- [ ] AC2: Only one controller instance can hold the active lease for a run at a time
- [ ] AC3: Lease ownership is visible to operators and included in diagnostics

## BDD Scenarios

### Scenario: Controller claims ownership before advancing a run
- **Given** a run is ready for work
- **When** a controller attempts to process it
- **Then** it must claim the active lease first

### Scenario: Second controller cannot steal active ownership silently
- **Given** one controller already owns the run lease
- **When** another controller attempts to claim it
- **Then** the second claim is rejected or deferred

### Scenario: Operator can inspect current ownership
- **Given** a run has an active lease
- **When** the operator checks runtime diagnostics
- **Then** the owner and lease timing are visible
