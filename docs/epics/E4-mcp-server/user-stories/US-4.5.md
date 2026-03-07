# US-4.5 — Implement `loom_get_state` tool

## Epic
E4: MCP Server

## Assigned Agent

**[Backend Developer](../../../../.github/agents/backend-developer.md)** — apply [`loom-architecture`](../../../../.github/skills/loom-architecture.md) · [`go-standards`](../../../../.github/skills/go-standards.md) · [`tdd-workflow`](../../../../.github/skills/tdd-workflow.md).


## Goal
Implement the `loom_get_state` tool handler so it returns a full state dump — including current phase, pending gates, and recent transition history — for debugging purposes.

## Acceptance Criteria

```
Given the FSM has completed 3 transitions (IDLE→SCANNING→ISSUE_CREATED→AWAITING_PR)
When the session calls `loom_get_state`
Then the response contains `"current_state": "AWAITING_PR"`
  And `"phase"` reflects the current phase number
  And `"history"` contains the last 3 transitions with timestamps
```

```
Given the FSM has no transition history
When `loom_get_state` is called
Then the response contains `"history": []` (empty array, not null)
  And no error is returned
```

## Tasks

1. [ ] Write `tools_get_state_test.go` with history and empty-history cases (write tests first)
2. [ ] Add a `TransitionHistory` slice to `Machine` that records the last N (configurable, default 20) transitions
3. [ ] Define `GetStateResponse` struct with `CurrentState`, `Phase`, `PRNumber`, `RetryBudgets`, `History` fields
4. [ ] Implement `handleGetState` reading from the Machine and the Store
5. [ ] Wire handler into server registration
6. [ ] Run `go test ./internal/mcp/... -race` and confirm green

## Dependencies
- US-4.1

## Size Estimate
S
