---
id: TH3.E3.US4
title: "Policy evaluation audit trail"
type: standard
priority: medium
size: M
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "Each policy evaluation stores the input snapshot reference, the decision name, and the resulting outcome"
  - AC2: "Operators can inspect why a runtime decision was made without replaying prompt history"
  - AC3: "Audit records support later replay and regression analysis"
depends-on: [TH3.E3.US2, TH3.E3.US3]
---

# TH3.E3.US4 — Policy Evaluation Audit Trail

**As a** Loom operator, **I want** each runtime decision to leave an audit trail, **so that** unexpected behavior can be explained and replayed later.

## Acceptance Criteria

- [ ] AC1: Each policy evaluation stores the input snapshot reference, the decision name, and the resulting outcome
- [ ] AC2: Operators can inspect why a runtime decision was made without replaying prompt history
- [ ] AC3: Audit records support later replay and regression analysis

## BDD Scenarios

### Scenario: Policy evaluation records its input and outcome
- **Given** the runtime evaluates a policy decision
- **When** the decision completes
- **Then** Loom records the input snapshot reference, decision name, and outcome

### Scenario: Operator inspects a previous decision
- **Given** a run behaved unexpectedly
- **When** the operator inspects the audit trail
- **Then** the recorded policy inputs and outcome explain the runtime choice

### Scenario: Audit record is reusable in replay tests
- **Given** a replay or regression fixture is being assembled
- **When** a past decision is selected
- **Then** its audit record can be used to reproduce the runtime branch
