---
id: TH3.E2.US2
title: "Poll-driven resumptions"
type: standard
priority: high
size: M
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "Polling is the baseline resumption path for CI, review, and PR readiness waits"
  - AC2: "Poll outcomes feed the runtime policy engine through explicit observation records"
  - AC3: "Polling resumes the run from persisted state rather than replaying prompt history"
depends-on: [TH3.E2.US1]
---

# TH3.E2.US2 — Poll-Driven Resumptions

**As a** Loom maintainer, **I want** polling to be the guaranteed baseline resume mechanism, **so that** Loom can make progress even without external event delivery.

## Acceptance Criteria

- [ ] AC1: Polling is the baseline resumption path for CI, review, and PR readiness waits
- [ ] AC2: Poll outcomes feed the runtime policy engine through explicit observation records
- [ ] AC3: Polling resumes the run from persisted state rather than replaying prompt history

## BDD Scenarios

### Scenario: Polling checks a waiting gate from persisted state
- **Given** a run is waiting on CI, review, or PR readiness
- **When** the next poll wake-up is due
- **Then** Loom queries GitHub and records the observation
- **And** it evaluates the result from persisted runtime state

### Scenario: Polling remains valid without event adapters
- **Given** no webhook or callback integration exists
- **When** Loom manages a waiting state
- **Then** polling still provides a complete baseline resume path

### Scenario: Poll observation is traceable
- **Given** a poll result changes the runtime decision
- **When** the result is recorded
- **Then** the corresponding observation can be traced in logs or replay inputs
