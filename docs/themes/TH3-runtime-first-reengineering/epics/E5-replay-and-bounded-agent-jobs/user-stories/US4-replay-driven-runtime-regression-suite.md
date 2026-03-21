---
id: TH3.E5.US4
title: "Replay-driven runtime regression suite"
type: standard
priority: medium
size: M
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "Representative replay fixtures are exercised automatically in the test suite"
  - AC2: "The suite covers wake-up, policy, locking, recovery, and bounded-job failure paths"
  - AC3: "Regression failures identify which replay fixture or runtime branch diverged"
depends-on: [TH3.E5.US1, TH3.E5.US3]
---

# TH3.E5.US4 — Replay-Driven Runtime Regression Suite

**As a** Loom maintainer, **I want** replay-driven regression coverage, **so that** runtime-first behavior stays stable as the implementation evolves.

## Acceptance Criteria

- [ ] AC1: Representative replay fixtures are exercised automatically in the test suite
- [ ] AC2: The suite covers wake-up, policy, locking, recovery, and bounded-job failure paths
- [ ] AC3: Regression failures identify which replay fixture or runtime branch diverged

## BDD Scenarios

### Scenario: Replay fixtures run in automated tests
- **Given** a set of approved replay fixtures exists
- **When** the automated test suite runs
- **Then** those fixtures are executed against the runtime

### Scenario: Core runtime-first branches are covered
- **Given** the regression suite is maintained
- **When** its coverage is reviewed
- **Then** it includes wake-up, policy, locking, recovery, and bounded-agent failure paths

### Scenario: Divergence points to a specific fixture
- **Given** a runtime change breaks prior behavior
- **When** a replay-driven regression fails
- **Then** the failure identifies the fixture or runtime branch that diverged
