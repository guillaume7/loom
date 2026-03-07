# US-4.7 — Round-trip tests for all 5 tools

## Epic
E4: MCP Server

## Assigned Agent

**[Test Engineer](../../../../.github/agents/test-engineer.md)** — apply [`tdd-workflow`](../../../../.github/skills/tdd-workflow.md) · [`loom-architecture`](../../../../.github/skills/loom-architecture.md).


## Goal
Verify the complete MCP request→handler→response cycle for every tool using injected mock FSM and mock GitHub client, without a live VS Code session.

## Acceptance Criteria

```
Given a mock FSM and mock GitHub client injected via `NewServer(deps)`
When each of the 5 tool handlers is invoked via the mcp-go test harness
Then every tool returns a well-formed JSON response
  And no tool panics under any valid input
  And the `-race` detector reports no data races
```

```
Given a `loom_checkpoint` round-trip with a valid action
When the full request/response cycle completes
Then the mock FSM records exactly one `Transition` call
  And the mock store records exactly one `WriteCheckpoint` call
```

```
Given a `loom_heartbeat` round-trip during a gate state
When the full request/response cycle completes
Then the mock FSM records zero `Transition` calls
```

## Tasks

1. [ ] Write `server_roundtrip_test.go` with a shared `newTestServer(t)` helper using mock deps (write tests first)
2. [ ] Define `MockMachine` and `MockStore` in `internal/mcp/mocks_test.go` implementing the respective interfaces
3. [ ] Write round-trip tests for each tool: `loom_next_step`, `loom_checkpoint`, `loom_heartbeat`, `loom_get_state`, `loom_abort`
4. [ ] Assert call counts on mock objects after each round-trip
5. [ ] Run `go test ./internal/mcp/... -race -count=1` and confirm all pass

## Dependencies
- US-4.2
- US-4.3
- US-4.4
- US-4.5
- US-4.6

## Size Estimate
M
