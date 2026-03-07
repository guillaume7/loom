# US-8.3 — Integration test: retry budget exhaustion → PAUSED → resume

## Epic
E8: Integration & Hardening

## Goal
Write an integration test that drives the AWAITING_PR retry budget to exhaustion, asserts PAUSED is reached, and then verifies `loom resume` correctly re-enters the FSM from AWAITING_PR.

## Acceptance Criteria

```
Given the AWAITING_PR retry budget is set to 3 (overridden for the test)
  And the `httptest.Server` never returns an open PR
When the FSM receives 3 timeout events on AWAITING_PR
Then the FSM transitions to PAUSED
  And the store contains a checkpoint with `state: "PAUSED"` and a reason field
```

```
Given the FSM is PAUSED at AWAITING_PR budget exhaustion
  And the httptest server now returns an open PR
When `loom resume` is called
Then the FSM re-enters AWAITING_PR and advances to AWAITING_READY
  And the PAUSED checkpoint is replaced with AWAITING_READY
```

## Tasks

1. [ ] Write `budget_exhaustion_test.go` with a budget=3 override and a never-opens-PR handler (write test first)
2. [ ] Assert PAUSED state and store checkpoint after 3 timeout events
3. [ ] Update the httptest handler to return an open PR for the resume phase
4. [ ] Simulate `loom resume` by re-initialising the FSM from the PAUSED checkpoint
5. [ ] Assert the FSM advances past AWAITING_PR after resume
6. [ ] Run `go test ./integration/... -race -count=1` and confirm green

## Dependencies
- US-8.1

## Size Estimate
M
