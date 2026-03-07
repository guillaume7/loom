# US-8.2 — Integration test: CI failure → DEBUGGING → fix → AWAITING_CI → REVIEWING → MERGING

## Epic
E8: Integration & Hardening

## Goal
Write an integration test that exercises the debug loop: CI fails, a debug issue is created, a fix is pushed, CI goes green, review is approved, and the PR is merged.

## Acceptance Criteria

```
Given an `httptest.Server` that returns `ci_red` on the first check-run poll for phase 1
  And returns `ci_green` on the second poll (after the fix push event)
When the integration test drives the FSM
Then the FSM visits states: AWAITING_CI → DEBUGGING → AWAITING_CI → REVIEWING → MERGING → SCANNING
  And a debug issue is created in the httptest request log
  And no PAUSED state is entered
```

```
Given the FSM exits DEBUGGING and re-enters AWAITING_CI
When CI returns green on the second poll
Then the transition to REVIEWING happens without resetting the phase counter
```

## Tasks

1. [ ] Write `debug_loop_test.go` with a two-stage httptest handler that fails CI then succeeds (write test first)
2. [ ] Extend the httptest handler to serve debug-issue creation and fix-push webhook
3. [ ] Drive the FSM loop and assert the state sequence
4. [ ] Assert debug issue creation appears in the request log
5. [ ] Run `go test ./integration/... -race -count=1` and confirm green

## Dependencies
- US-8.1

## Size Estimate
L
