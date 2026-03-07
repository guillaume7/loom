---
name: "E5 — CLI"
about: Wrap the MCP server in a cobra-based CLI with 7 subcommands, providing a human-operable interface to Loom.
title: "E5: CLI"
labels: ["epic", "E5"]
---

## Assigned Agents

| Role | Agent | Required Skills |
|---|---|---|
| Owner | [Backend Developer](../agents/backend-developer.md) | [`loom-architecture`](../skills/loom-architecture.md) · [`go-standards`](../skills/go-standards.md) · [`tdd-workflow`](../skills/tdd-workflow.md) |

## Goal

Wrap the MCP server in a `cobra`-based CLI with 7 subcommands, providing a human-operable interface to Loom.

## Description

The CLI is the entry point for human operators. It must:

- Route to the correct internal action per subcommand
- Load config from `~/.loom/config.toml` (or env overrides)
- Print structured status output for `status` and `log` commands
- Start the MCP server (stdio) for the `mcp` subcommand

## User Stories

- [ ] US-5.1 — `loom mcp` — start MCP stdio server
- [ ] US-5.2 — `loom start` — begin from IDLE or resume from checkpoint
- [ ] US-5.3 — `loom status` — print current state, phase, PR, last 20 log lines
- [ ] US-5.4 — `loom pause` / `loom resume` — graceful pause and continue
- [ ] US-5.5 — `loom reset` — clear state with confirmation prompt
- [ ] US-5.6 — `loom log` — stream structured JSON log
- [ ] US-5.7 — Config loading from file + env overrides

## Dependencies

- E4 (MCP server)

## Acceptance Criteria

- [ ] `loom --help` lists all 7 subcommands with descriptions
- [ ] `loom mcp` starts the MCP stdio server and reads from stdin
- [ ] `loom start` prints the current state before starting the loop
- [ ] `loom reset` prompts for confirmation: `Are you sure? [y/N]`
- [ ] Config loads from `LOOM_OWNER`, `LOOM_REPO`, `LOOM_TOKEN` env vars
- [ ] `loom status` exits 0 even when no checkpoint exists (prints "No active session")
- [ ] `go build ./cmd/loom` produces a binary < 30 MB

## Notes

<!-- Any additional context, design decisions, or blockers. -->
