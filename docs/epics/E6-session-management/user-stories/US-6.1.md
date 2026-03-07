# US-6.1 — Heartbeat timer: Go binary emits periodic log entries during gate waits

## Epic
E6: Session Management

## Assigned Agent

**[Backend Developer](../../../../.github/agents/backend-developer.md)** — apply [`loom-architecture`](../../../../.github/skills/loom-architecture.md) · [`go-standards`](../../../../.github/skills/go-standards.md) · [`tdd-workflow`](../../../../.github/skills/tdd-workflow.md).


## Goal
Start a background goroutine when the FSM enters a gate state that emits a structured `slog.Info("heartbeat")` log entry every 60 seconds, and stops when the FSM leaves the gate state.

## Acceptance Criteria

```
Given the FSM transitions into a gate state (AWAITING_PR, AWAITING_READY, AWAITING_CI, REVIEWING)
When 60 seconds elapse (using a fake clock in tests)
Then a `slog.Info("heartbeat")` entry is emitted with fields `state` and `elapsed_seconds`
```

```
Given a heartbeat goroutine is running in AWAITING_CI
When the FSM transitions out of AWAITING_CI to REVIEWING
Then the heartbeat goroutine stops within one tick interval
  And no further heartbeat entries are emitted for AWAITING_CI
```

```
Given the FSM is in a non-gate state (SCANNING, MERGING, etc.)
When time advances
Then no heartbeat entries are emitted
```

## Tasks

1. [ ] Write `heartbeat_test.go` using a fake clock interface to control tick timing (write tests first)
2. [ ] Define a `Clock` interface with `Now()` and `After(d)` in `internal/session/clock.go`
3. [ ] Implement `HeartbeatManager` in `internal/session/heartbeat.go` that starts/stops per gate state
4. [ ] Wire `HeartbeatManager` into the FSM transition hooks
5. [ ] Inject real `time` in production and fake clock in tests
6. [ ] Run `go test ./internal/session/... -race` and confirm green

## Dependencies
- US-4.4
- US-5.1

## Size Estimate
M
