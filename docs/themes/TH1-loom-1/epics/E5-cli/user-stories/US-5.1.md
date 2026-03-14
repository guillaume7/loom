# US-5.1 — `loom mcp` — start MCP stdio server

## Epic
E5: CLI

## Assigned Agent

**[Backend Developer](../../../../.github/agents/backend-developer.md)** — apply [`loom-architecture`](../../../../.github/skills/loom-architecture.md) · [`go-standards`](../../../../.github/skills/go-standards.md) · [`tdd-workflow`](../../../../.github/skills/tdd-workflow.md).


## Goal
Add the `loom mcp` subcommand that wires config and dependencies, starts the MCP stdio server, and blocks until stdin is closed or the process is interrupted.

## Acceptance Criteria

```
Given `loom mcp` is executed
When the process starts
Then the MCP server begins reading JSON-RPC messages from stdin
  And a `slog.Info("mcp server started")` log line is emitted to stderr
  And the process blocks (does not exit immediately)
```

```
Given `loom mcp` is running
When stdin is closed (EOF)
Then the process exits 0
```

```
Given `LOOM_TOKEN` is not set and no config file exists
When `loom mcp` is executed
Then the process starts without error (token is optional at startup; errors surface on first GitHub call)
```

## Tasks

1. [ ] Write `mcp_cmd_test.go` asserting `loom mcp --help` outputs a description (write test first)
2. [ ] Create `cmd/loom/cmd_mcp.go` with a `mcpCmd` cobra.Command
3. [ ] Wire `config.Load()`, `github.NewClient(cfg)`, `fsm.NewMachine()`, and `store.New(cfg.DBPath)` in the command's `RunE`
4. [ ] Call `mcp.NewServer(deps).ServeStdio()` to start the server
5. [ ] Register `mcpCmd` on the root command
6. [ ] Run `go build ./cmd/loom` and confirm it exits 0

## Dependencies
- US-5.7
- US-4.1

## Size Estimate
S
