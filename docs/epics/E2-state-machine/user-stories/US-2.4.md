# US-2.4 — Implement PAUSED escape state (triggered on budget exhaustion)

## Epic
E2: State Machine

## Goal
Automatically transition the FSM to `PAUSED` when any gate state's retry budget is exhausted, and support the `EventAbort` event as an emergency escape from any state.

## Acceptance Criteria

```
Given the FSM is in state AWAITING_PR
  And the retry budget for AWAITING_PR is 20
When `Transition(EventTimeout)` is called for the 21st time
Then the FSM state becomes PAUSED
  And the returned error is nil (budget exhaustion is an expected transition, not an error)
```

```
Given the FSM is in any non-terminal state
When `Transition(EventAbort)` is called
Then the FSM state becomes PAUSED
  And no error is returned
```

```
Given the FSM is in state PAUSED
When `Transition(EventStart)` is called
Then an error is returned (PAUSED is a terminal gate; no automatic escape)
  And the FSM state remains PAUSED
```

## Tasks

1. [ ] Write `machine_paused_test.go` covering budget exhaustion and abort transitions (write test first)
2. [ ] Add budget-exhaustion check inside `Transition()` after incrementing the retry counter
3. [ ] Register `EventAbort → PAUSED` as a wildcard transition applicable from all non-PAUSED states
4. [ ] Ensure `PAUSED` state has no outgoing transitions in the table (only `EventAbort` and `EventStart` are checked, both return error)
5. [ ] Run `go test ./internal/fsm/... -race -cover` and verify PAUSED paths are covered

## Dependencies
- US-2.3

## Size Estimate
S
