---
id: TH3.E3.US2
title: "CI review and merge policy decisions"
type: standard
priority: high
size: M
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "The runtime expresses CI, review, and merge readiness as named policy decisions with explicit inputs and outputs"
  - AC2: "A decision can produce continue, wait, retry, escalate, or block outcomes without invoking an agent"
  - AC3: "Merge safety checks remain conservative when observations are incomplete or stale"
depends-on: [TH3.E3.US1]
---

# TH3.E3.US2 — CI Review and Merge Policy Decisions

**As a** Loom operator, **I want** CI, review, and merge gates to be runtime decisions, **so that** progress does not depend on an agent remaining alive to interpret waiting conditions.

## Acceptance Criteria

- [ ] AC1: The runtime expresses CI, review, and merge readiness as named policy decisions with explicit inputs and outputs
- [ ] AC2: A decision can produce continue, wait, retry, escalate, or block outcomes without invoking an agent
- [ ] AC3: Merge safety checks remain conservative when observations are incomplete or stale

## BDD Scenarios

### Scenario: CI state produces a deterministic wait or continue outcome
- **Given** Loom has the latest CI observation for a PR
- **When** the runtime evaluates the CI gate
- **Then** it produces an explicit continue, wait, retry, escalate, or block outcome

### Scenario: Review state influences merge readiness deterministically
- **Given** review observations are available
- **When** Loom evaluates merge readiness
- **Then** the policy result reflects those observations without requiring an agent judgement call

### Scenario: Incomplete observations fail closed
- **Given** CI or review observations are stale or incomplete
- **When** Loom evaluates merge readiness
- **Then** it does not proceed optimistically
- **And** it returns a conservative outcome
