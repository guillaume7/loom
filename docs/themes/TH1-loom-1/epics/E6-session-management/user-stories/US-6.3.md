# US-6.3 — Stall response: write PAUSED checkpoint, log stall reason

## Epic
E6: Session Management

## Assigned Agent

**[Backend Developer](../../../../.github/agents/backend-developer.md)** — apply [`loom-architecture`](../../../../.github/skills/loom-architecture.md) · [`go-standards`](../../../../.github/skills/go-standards.md) · [`tdd-workflow`](../../../../.github/skills/tdd-workflow.md).


## Goal
When the stall detector fires, transition the FSM to PAUSED, write a checkpoint with the stall reason, and emit a structured error log entry so the operator knows what happened.

## Acceptance Criteria

```
Given the stall detector fires while the FSM is in AWAITING_CI
When the stall response handler runs
Then the FSM transitions to PAUSED
  And a checkpoint is written to the store with `reason: "stall: no checkpoint for 5m0s"`
  And a `slog.Error("session stalled", ...)` entry is emitted
```

```
Given a stall response has been written
When `loom status` is executed
Then it shows `State: PAUSED` and the stall reason
```

## Tasks

1. [ ] Write `stall_response_test.go` asserting FSM state, store write, and log output (write tests first)
2. [ ] Implement `StallResponder` in `internal/session/stall_response.go`
3. [ ] On stall event: call `machine.Transition(EventAbort)`, write checkpoint with reason, emit `slog.Error`
4. [ ] Wire `StallDetector` and `StallResponder` together in the session manager
5. [ ] Run `go test ./internal/session/... -race` and confirm green

## Dependencies
- US-6.2

## Size Estimate
S
