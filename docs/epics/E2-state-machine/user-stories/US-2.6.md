# US-2.6 — Full table-driven test matrix for all invalid transitions

## Epic
E2: State Machine

## Assigned Agent

**[Test Engineer](../../../../.github/agents/test-engineer.md)** — apply [`tdd-workflow`](../../../../.github/skills/tdd-workflow.md) · [`loom-architecture`](../../../../.github/skills/loom-architecture.md).


## Goal
Ensure every (from-state, event) pair that does NOT appear in the transition table returns a non-nil error and leaves the FSM state unchanged.

## Acceptance Criteria

```
Given a table of invalid (from-state, event) pairs
When `Machine.Transition(event)` is called for each row
Then a non-nil error is returned describing the invalid transition
  And `Machine.State()` equals the original from-state (no mutation on error)
```

```
Given the FSM is in state COMPLETE
When any event except EventAbort is sent
Then an error is returned
  And the FSM state remains COMPLETE
```

```
Given the FSM is in state IDLE
When `Transition(EventCIGreen)` is called (wrong state)
Then a non-nil error is returned
  And the state remains IDLE
```

## Tasks

1. [ ] Enumerate all invalid (from, event) pairs by computing the complement of the valid transition table (write test data first)
2. [ ] Write `TestInvalidTransitions` iterating the slice, asserting non-nil error and unchanged state
3. [ ] Add at least one test row per state to ensure full from-state coverage of the error path
4. [ ] Run `go test ./internal/fsm/... -race -cover` and confirm coverage remains at 100%

## Dependencies
- US-2.5

## Size Estimate
S
