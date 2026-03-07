# US-8.4 — Integration test: session stall detection → PAUSED → resume

## Epic
E8: Integration & Hardening

## Assigned Agent

**[Test Engineer](../../../../.github/agents/test-engineer.md)** — apply [`tdd-workflow`](../../../../.github/skills/tdd-workflow.md) · [`loom-architecture`](../../../../.github/skills/loom-architecture.md).

**[Debugger](../../../../.github/agents/debugger.md)** — apply [`loom-architecture`](../../../../.github/skills/loom-architecture.md) · [`tdd-workflow`](../../../../.github/skills/tdd-workflow.md).


## Goal
Write an integration test that simulates a session stall (no `loom_checkpoint` calls for the configured timeout) and verifies that the stall detector transitions the FSM to PAUSED and that `loom resume` recovers correctly.

## Acceptance Criteria

```
Given the stall timeout is set to 100ms (overridden for the test via a fake clock)
  And the FSM is in state AWAITING_CI
When no `loom_checkpoint` call arrives within 100ms
Then the stall detector fires
  And the FSM transitions to PAUSED
  And the store checkpoint contains `reason: "stall: no checkpoint for 100ms"`
```

```
Given the FSM is PAUSED due to a stall
When `loom resume` is called and `loom_checkpoint` calls resume normally
Then the FSM re-enters AWAITING_CI
  And the stall timer resets so no immediate re-stall occurs
```

## Tasks

1. [ ] Write `stall_detection_test.go` using a fake clock with a 100ms timeout override (write test first)
2. [ ] Inject the fake clock into the `StallDetector` for the test
3. [ ] Drive the FSM to AWAITING_CI then advance the fake clock past the stall threshold
4. [ ] Assert PAUSED state and stall reason in the store
5. [ ] Simulate resume and assert FSM re-enters AWAITING_CI with a reset stall timer
6. [ ] Run `go test ./integration/... -race -count=1` and confirm green

## Dependencies
- US-8.3

## Size Estimate
M
