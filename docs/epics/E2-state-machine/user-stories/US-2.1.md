# US-2.1 — Define all State and Event constants with metadata

## Epic
E2: State Machine

## Goal
Declare all 13 `State` constants and all FSM `Event` constants in `internal/fsm/states.go` and `internal/fsm/events.go` so the rest of the FSM can reference them as typed values.

## Acceptance Criteria

```
Given `internal/fsm/states.go` is compiled
When the package is imported
Then all 13 state constants are accessible: IDLE, SCANNING, ISSUE_CREATED, AWAITING_PR, AWAITING_READY, AWAITING_CI, REVIEWING, DEBUGGING, ADDRESSING_FEEDBACK, MERGING, REFACTORING, COMPLETE, PAUSED
  And each constant has a `String()` method returning a human-readable name
```

```
Given `internal/fsm/events.go` is compiled
When the package is imported
Then event constants for every transition in §5.2 of analysis.md are accessible
  And each event has a typed `Event` type (not bare string or int)
```

```
Given `internal/fsm` is compiled
When `go test ./internal/fsm/...` is executed
Then it exits 0 with the state and event constant tests passing
  And `internal/fsm` imports zero packages outside the Go standard library
```

## Tasks

1. [ ] Write `states_test.go` asserting all 13 state constants exist and `String()` returns non-empty strings (write test first)
2. [ ] Implement `internal/fsm/states.go` with `type State int` and all 13 constants with `String()`
3. [ ] Write `events_test.go` asserting all required event constants exist and are distinct
4. [ ] Implement `internal/fsm/events.go` with `type Event int` and all transition events
5. [ ] Run `go test ./internal/fsm/... -race` and confirm green

## Dependencies
- US-1.2

## Size Estimate
S
