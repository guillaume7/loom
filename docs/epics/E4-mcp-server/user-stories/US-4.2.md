# US-4.2 — Implement `loom_next_step` tool

## Epic
E4: MCP Server

## Goal
Implement the `loom_next_step` tool handler so it returns a structured instruction object describing the next WORKFLOW_GITHUB.md action for the current FSM state.

## Acceptance Criteria

```
Given the FSM is in state SCANNING
When the session calls `loom_next_step` with no parameters
Then the response contains `action: "identify_current_phase"` and `workflow_step: 1`
  And the response is a JSON-serialisable struct (not raw string)
```

```
Given the FSM is in state AWAITING_CI
When the session calls `loom_next_step`
Then the response contains `action: "poll_ci"` and a `retry_in_seconds` field
```

```
Given the FSM is in state COMPLETE
When the session calls `loom_next_step`
Then the response contains `action: "done"` and `complete: true`
```

```
Given the tool call carries a cancelled `context.Context`
When `loom_next_step` is called
Then it returns immediately with a context-cancelled error
```

## Tasks

1. [ ] Write `tools_next_step_test.go` with table-driven tests for each FSM state (write tests first)
2. [ ] Define `NextStepResponse` struct in `tools.go`
3. [ ] Implement `handleNextStep` handler mapping each FSM state to a `NextStepResponse`
4. [ ] Add `slog.Info("loom_next_step", "state", fsm.State(), "duration", ...)` logging
5. [ ] Wire handler into server registration in `server.go`
6. [ ] Run `go test ./internal/mcp/... -race` and confirm green

## Dependencies
- US-4.1

## Size Estimate
M
