# Loom — Copilot Instructions

## Project Overview

**Loom** is a Go CLI tool and MCP server that drives software development workflows autonomously. It manages a persistent VS Code Copilot Agent session (the "master session") that follows [WORKFLOW_GITHUB.md](.github/squad_prompts/WORKFLOW_GITHUB.md) end-to-end: scanning project status, creating issues, assigning `@copilot`, polling for PRs, requesting reviews, handling debug and refactor cycles, merging, and advancing to the next phase — with no human in the loop.

## Architecture Summary

Loom has three layers:

1. **Go binary** — deterministic FSM, GitHub API polling, SQLite checkpoints, structured logging, MCP stdio server
2. **Master Copilot session** — VS Code Agent that calls Loom MCP tools (`loom_next_step`, `loom_checkpoint`, `loom_heartbeat`) in a loop and uses GitHub MCP tools to act
3. **GitHub.com** — `@copilot` coding agent implements code, CI validates, Copilot reviewer reviews

## Repository Structure

```
cmd/loom/          # CLI entry point
internal/
  fsm/             # State machine — pure Go, no external deps
  github/          # GitHub REST API client wrapper
  mcp/             # MCP stdio server + tool implementations
  store/           # SQLite checkpoint persistence
  config/          # Repo config, phase sequence, timeouts
docs/
  adr/             # Architecture Decision Records
  loom/            # Analysis, design docs
  epics/           # Epic and user story definitions
  vision_of_product/ # Project vision and goals
.github/
  agents/          # Squad agent definitions
  skills/          # Shared skill files
  squad_prompts/   # Workflow guides
  workflows/       # GitHub Actions CI
```

## Key Design Decisions (All Resolved)

- **Language**: Go (latest stable) — single binary, strong HTTP/JSON stdlib, `google/go-github`
- **Session integration**: MCP stdio server registered in `.github/copilot/mcp.json`
- **State persistence**: SQLite via `modernc.org/sqlite` (pure Go, no CGo)
- **Logging**: `log/slog` stdlib — structured JSON
- **Retry budgets**: every async gate has a max retry count; PAUSED state is the escape hatch
- **Keepalive**: `loom_heartbeat` tool calls prevent the Copilot session from timing out during long waits
- **No undocumented APIs**: only `mcp_io_github_git_*` (official MCP) and GitHub public REST

## State Machine States

```
IDLE → SCANNING → ISSUE_CREATED → AWAITING_PR → AWAITING_READY
  → AWAITING_CI → { REVIEWING | DEBUGGING } → MERGING → REFACTORING
  → SCANNING (next phase) → ... → COMPLETE
```

Full diagram and transition table: `docs/loom/analysis.md § 5`

## Coding Standards

- **No global mutable state** — pass context explicitly
- **No `panic` in library code** — return errors
- **Interfaces for testability** — inject GitHub client, store, clock as interfaces
- **Table-driven tests** — prefer `[]struct{ name, input, want }` in `*_test.go`
- **Error wrapping** — `fmt.Errorf("doing X: %w", err)`
- **Conventional Commits** — `feat(fsm): add PAUSED state`
- **One package per concern** — `internal/fsm`, `internal/github`, `internal/mcp`, `internal/store`

## MCP Tool Surface

| Tool | Purpose |
|---|---|
| `loom_next_step` | Returns the current instruction: which WORKFLOW_GITHUB.md step to execute, with parameters |
| `loom_checkpoint` | Called after completing a step. Advances the FSM and persists state. |
| `loom_heartbeat` | Lightweight keepalive. Returns current state without advancing. |
| `loom_get_state` | Returns full state for debugging |
| `loom_abort` | Emergency stop → PAUSED state |

## Master Session Operating Contract

When a Copilot session is operating Loom itself rather than implementing Loom,
it should behave as the **Loom MCP Operator** defined in
`.github/agents/loom-mcp-operator.md` and apply the
`loom-mcp-loop` skill from `.github/skills/loom-mcp-loop.md`.

That contract is strict:

- ask Loom for the next step before acting
- execute one GitHub-side workflow step at a time
- checkpoint with the canonical action name immediately after the step completes
- use `loom_heartbeat` during waits
- use `loom_get_state` for diagnosis and `loom_abort` when the session cannot proceed safely

## When Writing or Editing Docs

- Maintain the existing Markdown style: H2 sections, tables for comparisons, blockquotes for key insights
- Preserve document numbering and cross-linking
- Always document the "why" alongside decisions
- Rejected alternatives are preserved with reasoning
