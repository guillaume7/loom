---
id: TH3.E4.US4
title: "Resume conflict handling"
type: standard
priority: medium
size: M
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "The runtime detects resume conflicts such as stale observations, lock contention, or superseded wake-ups"
  - AC2: "Conflicts resolve to explicit policy outcomes instead of implicit retries"
  - AC3: "Conflict handling preserves diagnosability for later replay and debugging"
depends-on: [TH3.E4.US2, TH3.E4.US3, TH3.E3.US4]
---

# TH3.E4.US4 — Resume Conflict Handling

**As a** Loom maintainer, **I want** resume conflicts handled explicitly, **so that** the runtime can recover predictably when conditions have changed since work was scheduled.

## Acceptance Criteria

- [ ] AC1: The runtime detects resume conflicts such as stale observations, lock contention, or superseded wake-ups
- [ ] AC2: Conflicts resolve to explicit policy outcomes instead of implicit retries
- [ ] AC3: Conflict handling preserves diagnosability for later replay and debugging

## BDD Scenarios

### Scenario: Wake-up becomes stale before execution
- **Given** a wake-up was scheduled from earlier observations
- **When** current observations show that state has changed materially
- **Then** Loom treats the wake-up as a conflict or superseded event

### Scenario: Lock contention prevents immediate resume
- **Given** the runtime wants to resume work
- **When** the required lock is unavailable
- **Then** the runtime resolves the situation with an explicit outcome such as wait or escalate

### Scenario: Conflict record supports later diagnosis
- **Given** a resume conflict occurred
- **When** an operator or replay test examines the run
- **Then** the recorded conflict explains why the original resume path changed
