---
id: TH2.E1.US3
title: "Unblocked and blocked evaluation functions"
type: standard
priority: high
size: M
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "Graph.Unblocked(done) returns all story IDs whose dependencies are satisfied"
  - AC2: "Graph.IsBlocked(id, done) returns true when any dependency is not in done set"
  - AC3: "Epic-level dependencies are respected — stories in a blocked epic are blocked"
  - AC4: "Stories with empty depends_on and in an unblocked epic are immediately unblocked"
depends-on: [TH2.E1.US2]
---

# TH2.E1.US3 — Unblocked and Blocked Evaluation Functions

**As a** Loom orchestrator, **I want** to query the dependency graph for unblocked stories, **so that** I can determine which stories are eligible for execution.

## Acceptance Criteria

- [ ] AC1: `Graph.Unblocked(done)` returns all story IDs whose dependencies are satisfied
- [ ] AC2: `Graph.IsBlocked(id, done)` returns true when any dependency is not in done set
- [ ] AC3: Epic-level dependencies are respected — stories in a blocked epic are blocked
- [ ] AC4: Stories with empty `depends_on` and in an unblocked epic are immediately unblocked

## BDD Scenarios

### Scenario: All root stories are unblocked initially
- **Given** a graph with epics E1 (no deps) containing US-1.1 (no deps) and US-1.2 (depends on US-1.1)
- **When** `Unblocked([]string{})` is called with an empty done set
- **Then** the result contains "US-1.1"
- **And** the result does not contain "US-1.2"

### Scenario: Completing a dependency unblocks dependents
- **Given** a graph where US-2.1 depends on US-1.1
- **When** `Unblocked([]string{"US-1.1"})` is called
- **Then** the result contains "US-2.1"

### Scenario: Epic-level dependency blocks all stories in epic
- **Given** a graph where epic E2 depends on E1, and E2 contains US-2.1 (no story deps)
- **When** `IsBlocked("US-2.1", []string{})` is called with no done stories from E1
- **Then** the result is `true`

### Scenario: Epic-level dependency satisfied unblocks epic stories
- **Given** a graph where epic E2 depends on E1, and all E1 stories are in done set
- **When** `IsBlocked("US-2.1", []string{"US-1.1", "US-1.2"})` is called
- **Then** the result is `false`

### Scenario: Multiple dependencies all must be satisfied
- **Given** a story US-3.1 that depends on both US-1.1 and US-2.1
- **When** `IsBlocked("US-3.1", []string{"US-1.1"})` is called (US-2.1 not done)
- **Then** the result is `true`

### Scenario: Unknown ID returns error
- **Given** a valid graph
- **When** `IsBlocked("US-99.99", done)` is called with a non-existent ID
- **Then** an error is returned containing the unknown ID
