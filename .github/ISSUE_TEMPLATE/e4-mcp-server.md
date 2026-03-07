---
name: "E4 — MCP Server"
about: Expose the FSM and GitHub client through a standards-compliant MCP stdio server callable from a VS Code Copilot session.
title: "E4: MCP Server"
labels: ["epic", "E4"]
---

## Goal

Expose the FSM and GitHub client through a standards-compliant MCP stdio server that a VS Code Copilot session can call.

## Description

The MCP server wires together `internal/fsm`, `internal/github`, and `internal/store` into 5 callable tools. Tool handlers must be fully testable (round-trip tool call tests) without a live VS Code session.

The server uses `github.com/mark3labs/mcp-go`.

## User Stories

- [ ] US-4.1 — Server setup and tool registration with typed input schemas
- [ ] US-4.2 — Implement `loom_next_step` tool
- [ ] US-4.3 — Implement `loom_checkpoint` tool (advances FSM + persists)
- [ ] US-4.4 — Implement `loom_heartbeat` tool
- [ ] US-4.5 — Implement `loom_get_state` tool
- [ ] US-4.6 — Implement `loom_abort` tool
- [ ] US-4.7 — Round-trip tests for all 5 tools

## Dependencies

- E2 (FSM)
- E3 (GitHub client)

## Acceptance Criteria

- [ ] All 5 tools registered and callable
- [ ] All tool input schemas are defined as typed structs (no `map[string]any`)
- [ ] All tool outputs are JSON-serialisable structs
- [ ] Every tool call logged with `slog.Info` (state, tool name, duration)
- [ ] Tool handlers respect `context.Context` cancellation
- [ ] `loom_checkpoint` with an invalid action returns an error (not panics)
- [ ] Round-trip tests pass with injected mock FSM and mock GitHub client
- [ ] `go test ./internal/mcp/... -race` exits 0

## Notes

<!-- Any additional context, design decisions, or blockers. -->
