---
id: TH3.E3.US1
title: "Runtime observation model"
type: standard
priority: high
size: M
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "The policy engine consumes a typed observation model for CI, review, branch, PR, and operator inputs"
  - AC2: "Observations are derived from persisted runtime records rather than prompt transcripts"
  - AC3: "The model distinguishes authoritative observations from derived summaries"
depends-on: [TH3.E1.US2]
---

# TH3.E3.US1 — Runtime Observation Model

**As a** Loom maintainer, **I want** a typed runtime observation model, **so that** policy decisions are made from stable inputs instead of reconstructed chat context.

## Acceptance Criteria

- [ ] AC1: The policy engine consumes a typed observation model for CI, review, branch, PR, and operator inputs
- [ ] AC2: Observations are derived from persisted runtime records rather than prompt transcripts
- [ ] AC3: The model distinguishes authoritative observations from derived summaries

## BDD Scenarios

### Scenario: Policy input is assembled from persisted runtime facts
- **Given** Loom has stored checkpoint, event, and poll observations
- **When** the policy engine prepares to evaluate a decision
- **Then** it builds its input from those persisted facts
- **And** it does not require prompt history to infer current state

### Scenario: Derived summaries do not replace authoritative observations
- **Given** Loom may compute summaries for display or debugging
- **When** the policy engine evaluates a decision
- **Then** authoritative observations remain distinguishable from summaries

### Scenario: Operator intent is part of the observation set
- **Given** an operator has paused, resumed, or overridden execution
- **When** policy input is assembled
- **Then** that operator intent is represented explicitly in the observation model
