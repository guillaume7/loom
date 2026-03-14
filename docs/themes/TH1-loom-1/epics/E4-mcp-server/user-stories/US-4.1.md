# US-4.1 — Server setup and tool registration with typed input schemas

## Epic
E4: MCP Server

## Assigned Agent

**[Backend Developer](../../../../.github/agents/backend-developer.md)** — apply [`loom-architecture`](../../../../.github/skills/loom-architecture.md) · [`go-standards`](../../../../.github/skills/go-standards.md) · [`tdd-workflow`](../../../../.github/skills/tdd-workflow.md).


## Goal
Initialise the `mcp-go` server in `internal/mcp/server.go` and register all 5 Loom tools with typed input schemas so the server can accept and validate tool-call requests.

## Acceptance Criteria

```
Given `internal/mcp/server.go` is compiled
When `NewServer(fsm, github, store)` is called
Then a `*mcp.Server` is returned with all 5 tools registered: `loom_next_step`, `loom_checkpoint`, `loom_heartbeat`, `loom_get_state`, `loom_abort`
```

```
Given the MCP server is initialised
When it receives a `tools/list` request
Then the response contains all 5 tool names with their input schemas
  And no schema field uses `map[string]any` (all schemas are typed structs)
```

```
Given a tool-call request with a missing required field
When the server processes it
Then it returns a structured error response (not a panic)
```

## Tasks

1. [ ] Write `server_test.go` with a `tools/list` round-trip test asserting all 5 tool names are present (write test first)
2. [ ] Define input schema structs for all 5 tools in `internal/mcp/tools.go`
3. [ ] Implement `NewServer(deps Deps) *Server` in `internal/mcp/server.go` using `mark3labs/mcp-go`
4. [ ] Register all 5 tools with their typed schemas (no-op handlers for now)
5. [ ] Add `slog.Info` logging on server start with tool count
6. [ ] Run `go test ./internal/mcp/... -race` and confirm green

## Dependencies
- US-2.4
- US-3.7

## Size Estimate
S
