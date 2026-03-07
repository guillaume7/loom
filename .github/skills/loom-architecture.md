# Skill: Loom Architecture

Authoritative architecture reference for all Loom agents. Read this before implementing anything.

---

## What Loom Does

Loom drives software development workflows autonomously. It manages a persistent VS Code Copilot Agent session (the "master session") that follows **WORKFLOW_GITHUB.md** end-to-end: scanning project status, creating phase issues, assigning `@copilot`, polling for PRs, requesting reviews, handling debug and refactor cycles, merging, and advancing to the next phase — with no human in the loop.

---

## Three-Layer Architecture

```
┌──────────────────────────────────────────────────────────────────────┐
│  VS Code                                                              │
│                                                                      │
│  ┌────────────────────────────────────┐   ┌─────────────────────┐  │
│  │  Copilot Agent Session (Master)    │   │  GitHub MCP Server    │  │
│  │  • Reads agents/ & skills/         ├──▶│  (mcp_io_github_git_*)│  │
│  │  • Composes issue bodies           │   │  • Create issues      │  │
│  │  • Analyses review feedback        │   │  • Assign @copilot    │  │
│  │  • Decides next action             │   │  • Read PR state      │  │
│  └───────────┼───────────────────┘   │  • Post comments      │  │
│             │ MCP tool calls              │  • Merge PRs          │  │
│  ┌───────────┤───────────────────┐   └─────────────────────┘  │
│  │  Loom MCP Server (Go binary)       │                              │
│  │  • loom_next_step                  │                              │
│  │  • loom_checkpoint                 │                              │
│  │  • loom_heartbeat                  │                              │
│  │  • loom_get_state                  │                              │
│  │  • loom_abort                      │                              │
│  │  Internal: FSM + store + GitHub    │                              │
│  └────────────────────────────────────┘                              │
└──────────────────────────────────────────────────────────────────────┘
```

| Layer | Runs As | Responsibility |
|---|---|---|
| **Copilot Master Session** | VS Code Agent chat | Intelligence: reads project files, composes issue bodies, analyses feedback |
| **Loom Go Binary** | MCP server (stdio) | Plumbing: FSM, GitHub API polling, session keepalive, checkpointing, logging |
| **GitHub.com** | Cloud | Execution: `@copilot` coding agent, CI, Copilot reviewer |

---

## Package Layout

```
cmd/loom/              ← CLI entry point: parses args, wires dependencies, starts MCP server
internal/
  fsm/                 ← Pure state machine — NO external deps, fully unit-testable
    machine.go         ← Machine struct, Transition(), State(), CanTransition()
    states.go          ← State constants + metadata
    events.go          ← Event constants
    machine_test.go    ← Table-driven transition tests
  github/              ← GitHub REST API wrapper
    client.go          ← Implements GitHubClient interface
    types.go           ← Request/response types
    client_test.go     ← httptest-based fixture tests
  mcp/                 ← MCP stdio server
    server.go          ← Server setup, tool registration
    tools.go           ← Tool handlers (next_step, checkpoint, heartbeat, get_state, abort)
    server_test.go     ← Tool call round-trip tests
  store/               ← SQLite checkpoint persistence
    sqlite.go          ← Implements Store interface
    sqlite_test.go     ← Read/write/resume tests
  config/              ← Config file loading, defaults
    config.go
```

---

## FSM States

| State | Description |
|---|---|
| `IDLE` | Initial state; waiting for `loom start` |
| `SCANNING` | Reading epics, listing PRs/issues, identifying current phase |
| `ISSUE_CREATED` | Phase issue created and `@copilot` assigned |
| `AWAITING_PR` | Polling for a draft PR on the phase branch |
| `AWAITING_READY` | Polling until PR is no longer draft |
| `AWAITING_CI` | Polling CI check-runs for the PR |
| `REVIEWING` | Copilot review requested; polling for review result |
| `DEBUGGING` | Debug sub-issue created; polling for fix push |
| `ADDRESSING_FEEDBACK` | Review had CHANGES_REQUESTED; feedback posted; polling for fix |
| `MERGING` | Executing the merge |
| `REFACTORING` | Post-epic refactor issue created and merged |
| `COMPLETE` | All phases done; release tag created |
| `PAUSED` | Gate exhausted or abort requested; manual intervention needed |

## FSM Retry Budgets

| Gate State | Max Retries | Poll Interval | On Exhaustion |
|---|---|---|---|
| `AWAITING_PR` | 20 | 30s | → PAUSED |
| `AWAITING_READY` | 60 | 30s | Force-promote, then continue |
| `AWAITING_CI` | 20 | 30s | → PAUSED |
| `DEBUGGING` loop | 3 full cycles | — | → PAUSED |
| `ADDRESSING_FEEDBACK` loop | 5 full cycles | — | → PAUSED |

---

## MCP Tool Contracts

### `loom_next_step`

**Input:** none (reads internal FSM state)

**Output:**
```json
{
  "state": "SCANNING",
  "instruction": "Read epics to identify next unfinished phase",
  "params": {
    "owner": "guillaume7",
    "repo": "vectorgame",
    "phase": 2,
    "issue_template": ".github/ISSUE_TEMPLATE/03-phase-2-lobby.md",
    "branch_pattern": "phase/2-*",
    "agents": ["frontend-dev"]
  }
}
```

### `loom_checkpoint`

**Input:**
```json
{
  "action": "pr_opened",
  "pr_number": 43,
  "details": "optional string"
}
```

**Output:** new FSM state after transition.

### `loom_heartbeat`

**Input:** none

**Output:**
```json
{
  "state": "AWAITING_CI",
  "retry": 4,
  "max_retry": 20,
  "wait": true,
  "message": "Still waiting for CI. Call loom_heartbeat again in 30 seconds."
}
```

### `loom_get_state`

**Output:** Full checkpoint: current state, phase number, PR number, issue number, retry counts, last 20 log lines.

### `loom_abort`

**Input:** `{ "reason": "string" }`

**Output:** transition to PAUSED + final state summary.

---

## Session Loop Protocol

The master Copilot session must follow this loop continuously:

```
LOOP:
  1. Call loom_next_step → get instruction
  2. Execute instruction using GitHub MCP tools
  3. Call loom_checkpoint with result
  4. If response.state == "COMPLETE" → stop
  5. If response.wait == true → wait retry_in_seconds, call loom_heartbeat, GOTO 1
  6. GOTO 1
```

During waits: call `loom_heartbeat` every 30–60 seconds to keep the session alive.

---

## Key Design Principles

1. **Separate plumbing from intelligence** — Go binary handles deterministic transitions; LLM handles contextual reasoning
2. **Locally testable** — `internal/fsm` has zero external dependencies
3. **Observable** — structured JSON log (`slog`) for every transition and API call
4. **Resumable** — SQLite checkpoint survives process/machine restarts
5. **No undocumented APIs** — only `mcp_io_github_git_*` and GitHub public REST
6. **Retry budgets everywhere** — PAUSED state prevents infinite loops
