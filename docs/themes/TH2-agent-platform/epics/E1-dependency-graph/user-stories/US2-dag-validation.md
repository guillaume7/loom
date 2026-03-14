---
id: TH2.E1.US2
title: "DAG validation — cycle detection and ID reference validation"
type: standard
priority: high
size: M
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "Circular dependencies are detected and reported with the cycle path"
  - AC2: "References to non-existent epic or story IDs are rejected with the invalid ID"
  - AC3: "Duplicate IDs within the same scope are rejected"
  - AC4: "Valid DAGs pass validation without error"
depends-on: [TH2.E1.US1]
---

# TH2.E1.US2 — DAG Validation — Cycle Detection and ID Reference Validation

**As a** Loom developer, **I want** the dependency graph to reject circular dependencies and invalid references at parse time, **so that** runtime evaluation never encounters an infinite loop or dangling reference.

## Acceptance Criteria

- [ ] AC1: Circular dependencies are detected and reported with the cycle path
- [ ] AC2: References to non-existent epic or story IDs are rejected with the invalid ID
- [ ] AC3: Duplicate IDs within the same scope are rejected
- [ ] AC4: Valid DAGs pass validation without error

## BDD Scenarios

### Scenario: Detect direct circular dependency
- **Given** a dependency file where epic E1 depends on E2 and E2 depends on E1
- **When** `Load(path)` is called
- **Then** an error is returned containing "circular dependency" and the cycle path "E1 → E2 → E1"

### Scenario: Detect transitive circular dependency
- **Given** a dependency file where story US-1.1 → US-1.2 → US-1.3 → US-1.1
- **When** `Load(path)` is called
- **Then** an error is returned containing "circular dependency" and the full cycle path

### Scenario: Reject reference to non-existent story
- **Given** a dependency file where story US-2.1 depends on "US-99.1" which does not exist
- **When** `Load(path)` is called
- **Then** an error is returned containing "unknown dependency" and "US-99.1"

### Scenario: Reject duplicate epic IDs
- **Given** a dependency file with two epics both having `id: E1`
- **When** `Load(path)` is called
- **Then** an error is returned containing "duplicate" and "E1"

### Scenario: Valid DAG passes validation
- **Given** a well-formed dependency file with no cycles and all references valid
- **When** `Load(path)` is called
- **Then** no error is returned
- **And** the resulting `Graph` is usable for evaluation
