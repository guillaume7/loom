# US-2.3 — Implement retry budgets per gate state

## Epic
E2: State Machine

## Assigned Agent

**[Backend Developer](../../../../.github/agents/backend-developer.md)** — apply [`loom-architecture`](../../../../.github/skills/loom-architecture.md) · [`tdd-workflow`](../../../../.github/skills/tdd-workflow.md) · [`go-standards`](../../../../.github/skills/go-standards.md).


## Goal
Add per-gate retry counters to `Machine` so each gate state (AWAITING_PR, AWAITING_READY, AWAITING_CI) tracks how many timeout events it has consumed against a configurable maximum.

## Acceptance Criteria

```
Given the FSM is in state AWAITING_PR with a budget of 3
When `Transition(EventTimeout)` is called 3 times
Then the retry counter for AWAITING_PR reaches the budget limit
  And no PAUSED transition has occurred yet (budget exhaustion handled in US-2.4)
```

```
Given the FSM is created with `Config{MaxRetriesAWAITINGPR: 20}`
When `Machine.RetryCount(StateAWAITINGPR)` is called after 5 timeout events
Then it returns 5
```

```
Given the FSM transitions away from AWAITING_PR to AWAITING_READY
When `Machine.RetryCount(StateAWAITINGPR)` is called
Then it returns 0 (counter resets on successful transition)
```

## Tasks

1. [ ] Write `machine_retry_test.go` asserting retry counters increment and reset correctly (write test first)
2. [ ] Add `Config` struct with `MaxRetries` fields for each gate state
3. [ ] Add per-state retry counters to `Machine` struct
4. [ ] Increment retry counter inside `Transition()` for timeout events on gate states
5. [ ] Reset retry counter when a gate state exits via a non-timeout event
6. [ ] Expose `RetryCount(state State) int` and `BudgetRemaining(state State) int` methods
7. [ ] Run `go test ./internal/fsm/... -race` and confirm green

## Dependencies
- US-2.2

## Size Estimate
S
