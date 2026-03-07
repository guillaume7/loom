# US-8.1 — Integration test: simulate full 3-phase lifecycle (IDLE → COMPLETE)

## Epic
E8: Integration & Hardening

## Goal
Write an end-to-end integration test that drives the full FSM + MCP + GitHub client + Store stack through a simulated 3-phase lifecycle from IDLE to COMPLETE using an `httptest` server.

## Acceptance Criteria

```
Given an `httptest.Server` simulating GitHub API responses for 3 phases
  And a real FSM, real store (`:memory:`), and real MCP server wired together
When the integration test drives the FSM through all 3 phases
Then the FSM reaches state COMPLETE
  And the store contains a checkpoint with `state: "COMPLETE"`
  And structured log entries are emitted for every state transition
```

```
Given the integration test completes
When the httptest server request log is inspected
Then requests were made to: create issue, list PRs, get check-runs, request review, get reviews, merge PR, create tag
  And no unexpected requests were made
```

```
Given `go test ./integration/... -race -count=1`
When executed
Then all assertions pass in < 30 seconds
```

## Tasks

1. [ ] Create `integration/` package and write `lifecycle_test.go` skeleton that fails (write test first)
2. [ ] Implement an `httptest.Server` handler simulating all required GitHub API endpoints for 3 phases
3. [ ] Wire FSM + MCP server + SQLite (`:memory:`) + GitHub client pointing to the test server
4. [ ] Drive tool calls (`loom_next_step` → `loom_checkpoint`) in a loop until COMPLETE
5. [ ] Assert final FSM state and store contents
6. [ ] Run `go test ./integration/... -race -count=1` and confirm green

## Dependencies
- US-2.6
- US-3.7
- US-4.7
- US-5.2
- US-6.5
- US-7.6

## Size Estimate
L
