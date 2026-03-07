# US-6.2 — Stall detection: detect no `loom_checkpoint` call for > N minutes while in a gate state

## Epic
E6: Session Management

## Assigned Agent

**[Backend Developer](../../../../.github/agents/backend-developer.md)** — apply [`loom-architecture`](../../../../.github/skills/loom-architecture.md) · [`go-standards`](../../../../.github/skills/go-standards.md) · [`tdd-workflow`](../../../../.github/skills/tdd-workflow.md).


## Goal
Implement a stall detector that monitors the time elapsed since the last `loom_checkpoint` call and fires a stall event if no checkpoint arrives within a configurable timeout (default 5 minutes) while the FSM is in a gate state.

## Acceptance Criteria

```
Given the FSM is in state AWAITING_CI
  And the stall timeout is set to 5 minutes (using a fake clock)
When 5 minutes elapse without a `loom_checkpoint` call
Then the stall detector fires a stall event
```

```
Given the FSM is in state AWAITING_CI with a 5-minute stall timeout
When `loom_checkpoint` is called at 4 minutes 50 seconds
Then the stall timer resets and no stall event fires
```

```
Given the FSM is in a non-gate state (MERGING)
When any amount of time passes without a checkpoint call
Then no stall event fires
```

## Tasks

1. [ ] Write `stall_test.go` with stall-fires, timer-reset, and non-gate-state cases using a fake clock (write tests first)
2. [ ] Implement `StallDetector` in `internal/session/stall.go` that wraps the `Clock` interface
3. [ ] Reset the stall timer each time `loom_checkpoint` is called
4. [ ] Fire a `StallEvent` on the returned channel when the timeout elapses in a gate state
5. [ ] Make the timeout configurable via `Config.StallTimeoutSeconds` (default 300)
6. [ ] Run `go test ./internal/session/... -race` and confirm green

## Dependencies
- US-6.1

## Size Estimate
M
