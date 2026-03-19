---
id: TH2.E9.US2
title: "FSM transition and operator intervention ledger"
status: done
epic: E9
depends-on: [TH2.E9.US1]
---

# TH2.E9.US2 — FSM transition and operator intervention ledger

## As a
Loom operator

## I want
every meaningful FSM transition and operator intervention to be recorded as a
timestamped, append-only trace event

## So that
I can reconstruct the exact sequence of state changes, understand why the
workflow entered a particular state, and audit any manual interventions.

## Acceptance Criteria

- [ ] A `trace_event` table is created in SQLite with columns: `id`,
      `session_id`, `seq`, `kind`, `from_state`, `to_state`, `event`,
      `reason`, `pr_number`, `issue_number`, `created_at`.
- [ ] `loom_checkpoint` appends a `transition` trace event on every successful
      state change.
- [ ] `loom_abort` appends an `intervention` trace event.
- [ ] Budget exhaustion that triggers elicitation appends an `intervention`
      event with `reason = "budget exhaustion — elicitation prompt emitted"`.
- [ ] Stall detection appends a `system` event with `event = "stall_detected"`
      and the elapsed duration in the reason field.
- [ ] Trace event errors are logged as warnings and never interrupt the primary
      workflow path.

## BDD Scenarios

```gherkin
Given a session with trace ID active
When loom_checkpoint fires the "start" event successfully
Then a trace event exists with kind="transition" from_state="IDLE" to_state="SCANNING"

Given a session in SCANNING state
When loom_abort is called
Then a trace event exists with kind="intervention" event="abort" to_state="PAUSED"

Given a session in a gate state
When no checkpoint is received for longer than the stall timeout
Then a trace event exists with kind="system" event="stall_detected"
```
