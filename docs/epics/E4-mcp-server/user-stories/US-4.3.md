# US-4.3 — Implement `loom_checkpoint` tool

## Epic
E4: MCP Server

## Goal
Implement the `loom_checkpoint` tool handler so it accepts an outcome from the session, fires the corresponding FSM event, persists the new state to the store, and returns the new state.

## Acceptance Criteria

```
Given the FSM is in state ISSUE_CREATED
When the session calls `loom_checkpoint` with `{"action": "copilot_assigned"}`
Then the FSM transitions to AWAITING_PR
  And a checkpoint is written to the store with state AWAITING_PR
  And the response contains `"new_state": "AWAITING_PR"`
```

```
Given the session calls `loom_checkpoint` with `{"action": "invalid_event"}`
When the handler processes it
Then a structured error response is returned (not a panic)
  And the FSM state is unchanged
  And nothing is written to the store
```

```
Given the store write fails
When `loom_checkpoint` is called with a valid action
Then the FSM state is rolled back (or the error is surfaced)
  And the response contains a non-nil error field
```

## Tasks

1. [ ] Write `tools_checkpoint_test.go` with success, invalid-action, and store-failure cases (write tests first)
2. [ ] Define `CheckpointRequest` and `CheckpointResponse` structs in `tools.go`
3. [ ] Implement `handleCheckpoint` with FSM transition + store write in a single logical unit
4. [ ] Add rollback / error surfacing if the store write fails
5. [ ] Log with `slog.Info("loom_checkpoint", "action", req.Action, "new_state", ...)`
6. [ ] Wire handler into server registration
7. [ ] Run `go test ./internal/mcp/... -race` and confirm green

## Dependencies
- US-4.1

## Size Estimate
M
