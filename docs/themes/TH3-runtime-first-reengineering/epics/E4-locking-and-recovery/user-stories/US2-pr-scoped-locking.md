---
id: TH3.E4.US2
title: "PR-scoped locking"
type: standard
priority: high
size: M
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "Mutating actions against the same PR are serialized by a PR-scoped lock or equivalent claim model"
  - AC2: "Read-only observations can proceed without violating the mutation lock"
  - AC3: "Lock contention is visible and does not corrupt action ordering"
depends-on: [TH3.E4.US1]
---

# TH3.E4.US2 — PR-Scoped Locking

**As a** Loom maintainer, **I want** PR mutations serialized, **so that** concurrent controllers or agent jobs do not race on the same branch or PR.

## Acceptance Criteria

- [ ] AC1: Mutating actions against the same PR are serialized by a PR-scoped lock or equivalent claim model
- [ ] AC2: Read-only observations can proceed without violating the mutation lock
- [ ] AC3: Lock contention is visible and does not corrupt action ordering

## BDD Scenarios

### Scenario: Two mutation attempts target the same PR
- **Given** two runtime paths want to mutate the same PR
- **When** they attempt to proceed
- **Then** only one holds the mutation lock at a time

### Scenario: Read-only inspection remains allowed
- **Given** a PR mutation lock is held
- **When** Loom performs a read-only observation
- **Then** the observation can proceed if it does not violate mutation safety

### Scenario: Contention is diagnosable
- **Given** a mutation attempt waits on a held lock
- **When** the operator inspects runtime state
- **Then** the contention is visible with enough detail to explain the wait
