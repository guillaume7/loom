# US-4.4 — Implement `loom_heartbeat` tool

## Epic
E4: MCP Server

## Goal
Implement the `loom_heartbeat` tool handler so it returns the current FSM state and a `wait` signal without advancing the state machine — keeping the Copilot session alive during long gate waits.

## Acceptance Criteria

```
Given the FSM is in a gate state (AWAITING_PR, AWAITING_CI, REVIEWING)
When the session calls `loom_heartbeat`
Then the response contains `"state": "<current_state>"`, `"wait": true`, and `"retry_in_seconds": 30`
  And the FSM state is unchanged after the call
```

```
Given the FSM is in a non-gate state (e.g. SCANNING)
When the session calls `loom_heartbeat`
Then the response contains `"wait": false`
  And the FSM state is unchanged
```

```
Given the tool call carries a cancelled context
When `loom_heartbeat` is called
Then it returns immediately with a context-cancelled error
```

## Tasks

1. [ ] Write `tools_heartbeat_test.go` with gate-state and non-gate-state cases (write tests first)
2. [ ] Define `HeartbeatResponse` struct with `State`, `Wait`, and `RetryInSeconds` fields
3. [ ] Implement `handleHeartbeat` returning current state without calling `Transition()`
4. [ ] Define the set of gate states as a package-level constant set
5. [ ] Wire handler into server registration
6. [ ] Run `go test ./internal/mcp/... -race` and confirm green

## Dependencies
- US-4.1

## Size Estimate
S
