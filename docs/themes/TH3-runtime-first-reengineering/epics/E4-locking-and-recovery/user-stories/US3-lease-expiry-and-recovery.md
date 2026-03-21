---
id: TH3.E4.US3
title: "Lease expiry and recovery"
type: standard
priority: medium
size: M
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "The runtime defines when a lease is considered expired or abandoned"
  - AC2: "Recovery of expired ownership is explicit and avoids re-running already committed external actions"
  - AC3: "Recovery attempts are recorded for later audit and debugging"
depends-on: [TH3.E4.US1, TH3.E2.US4]
---

# TH3.E4.US3 — Lease Expiry and Recovery

**As a** Loom operator, **I want** abandoned work to be recoverable, **so that** a crashed or stalled controller does not leave the system permanently blocked.

## Acceptance Criteria

- [ ] AC1: The runtime defines when a lease is considered expired or abandoned
- [ ] AC2: Recovery of expired ownership is explicit and avoids re-running already committed external actions
- [ ] AC3: Recovery attempts are recorded for later audit and debugging

## BDD Scenarios

### Scenario: Lease expires after missed renewal window
- **Given** a controller stops renewing its lease
- **When** the expiry condition is met
- **Then** Loom marks the lease as recoverable

### Scenario: Recovery avoids repeating committed side effects
- **Given** the abandoned run performed some external actions before failing
- **When** a new controller recovers ownership
- **Then** it does not repeat those actions blindly

### Scenario: Recovery is auditable
- **Given** a recovery occurred
- **When** the operator reviews the runtime history
- **Then** the recovery attempt and reason are visible
