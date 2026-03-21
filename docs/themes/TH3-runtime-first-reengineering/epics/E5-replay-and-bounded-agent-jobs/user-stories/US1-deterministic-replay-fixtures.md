---
id: TH3.E5.US1
title: "Deterministic replay fixtures"
type: standard
priority: high
size: M
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "Loom can assemble replay fixtures from persisted observations, policy decisions, and checkpoint snapshots"
  - AC2: "Replay input captures enough information to reproduce a runtime branch without prompt history"
  - AC3: "Fixtures are suitable for debugging and automated regression tests"
depends-on: [TH3.E2.US2, TH3.E2.US3, TH3.E3.US4]
---

# TH3.E5.US1 — Deterministic Replay Fixtures

**As a** Loom maintainer, **I want** deterministic replay fixtures, **so that** runtime behavior can be reproduced and debugged without rerunning live GitHub interactions.

## Acceptance Criteria

- [ ] AC1: Loom can assemble replay fixtures from persisted observations, policy decisions, and checkpoint snapshots
- [ ] AC2: Replay input captures enough information to reproduce a runtime branch without prompt history
- [ ] AC3: Fixtures are suitable for debugging and automated regression tests

## BDD Scenarios

### Scenario: Fixture captures the facts needed to replay a decision path
- **Given** a runtime branch completed in production or testing
- **When** Loom exports a replay fixture
- **Then** it includes the checkpoint snapshot, relevant observations, and policy outcomes

### Scenario: Replay does not require prompt transcript reconstruction
- **Given** a fixture has been exported
- **When** a developer replays it locally
- **Then** the runtime can reproduce the branch without using chat history

### Scenario: Fixture can seed a regression test
- **Given** a bug was reproduced with a replay fixture
- **When** the fixture is added to the test suite
- **Then** it can guard against recurrence
