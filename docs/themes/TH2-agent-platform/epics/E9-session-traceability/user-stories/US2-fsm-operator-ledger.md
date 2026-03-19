---
id: TH2.E9.US2
title: "FSM transition and operator intervention ledger"
type: standard
priority: medium
size: M
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "Each checkpointed FSM transition appends from_state, to_state, timestamp, and human-readable reason to the session trace"
  - AC2: "Retry increments, budget exhaustion, pause, resume, elicitation prompt, and elicitation response create trace entries in order"
  - AC3: "No-op polls without meaningful change do not emit false transition entries"
depends-on: [TH2.E9.US1]
---

# TH2.E9.US2 — FSM Transition and Operator Intervention Ledger

**As a** Loom maintainer, **I want** every FSM transition and operator intervention recorded in the session trace, **so that** I can explain why a run advanced, retried, or stalled.

## Acceptance Criteria

- [ ] AC1: Each checkpointed FSM transition appends `from_state`, `to_state`, timestamp, and human-readable reason to the session trace
- [ ] AC2: Retry increments, budget exhaustion, pause, resume, elicitation prompt, and elicitation response create trace entries in order
- [ ] AC3: No-op polls without meaningful change do not emit false transition entries

## BDD Scenarios

### Scenario: Valid transition is added to the ledger
- **Given** Loom advances from `AWAITING_CI` to `REVIEWING`
- **When** the checkpoint is persisted
- **Then** the session trace appends one `fsm_transition` entry with both states and the transition reason

### Scenario: Elicitation flow is fully represented
- **Given** a retry budget is exhausted during a session
- **When** Loom emits an elicitation and the operator responds
- **Then** the session trace contains entries for budget exhaustion, the elicitation prompt, and the operator response in chronological order

### Scenario: No-op polling does not fabricate transitions
- **Given** `loom_heartbeat` polls a gate state and nothing changes
- **When** the poll completes
- **Then** the session trace does not append a false FSM transition entry