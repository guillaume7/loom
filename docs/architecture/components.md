# Loom v2 — Component Breakdown

> Traces to: [VP2 §3](../vision_of_product/VP2-agent-platform/02-vision-agent-platform.md) (revised architecture), [VP2 §8](../vision_of_product/VP2-agent-platform/02-vision-agent-platform.md) (what Loom still owns)

## Component Map

```text
┌────────────────────────────────────────────────────────────────┐
│  Agent Layer (.github/agents/)          [not Go — declarative] │
│  ┌──────────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────┐  │
│  │ orchestrator │ │ gate     │ │ debug    │ │ merge        │  │
│  └──────┬───────┘ └────┬─────┘ └────┬─────┘ └──────┬───────┘  │
│         │ handoffs      │ subagent   │ handoff      │ handoff  │
└─────────┼───────────────┼────────────┼──────────────┼──────────┘
          │               │            │              │
          ▼               ▼            ▼              ▼
┌────────────────────────────────────────────────────────────────┐
│  MCP Server (internal/mcp)                                     │
│  ┌───────────┐ ┌───────────┐ ┌───────────┐ ┌───────────────┐  │
│  │ Tools     │ │ Resources │ │ Tasks     │ │ Elicitation   │  │
│  └─────┬─────┘ └─────┬─────┘ └─────┬─────┘ └───────┬───────┘  │
└────────┼──────────────┼─────────────┼───────────────┼──────────┘
         │              │             │               │
         ▼              ▼             ▼               ▼
┌────────────────────────────────────────────────────────────────┐
│  Core Layer                                                    │
│  ┌──────┐  ┌─────────┐  ┌────────────┐  ┌──────────────────┐  │
│  │ FSM  │  │ DepGraph │  │ Monitor    │  │ Config           │  │
│  └──┬───┘  └────┬─────┘  └─────┬──────┘  └────────┬─────────┘  │
└─────┼───────────┼───────────────┼──────────────────┼───────────┘
      │           │               │                  │
      ▼           ▼               ▼                  ▼
┌────────────────────────────────────────────────────────────────┐
│  Persistence Layer                                             │
│  ┌──────────────────┐  ┌─────────────────────────────┐        │
│  │ Store (SQLite)   │  │ .loom/dependencies.yaml     │        │
│  └──────────────────┘  └─────────────────────────────┘        │
└────────────────────────────────────────────────────────────────┘
      │
      ▼
┌────────────────────────────────────────────────────────────────┐
│  GitHub Integration (internal/github)                          │
│  HTTPClient → GitHub REST API                                  │
└────────────────────────────────────────────────────────────────┘
```text

---

## Component Details

### 1. Agent Definitions (`.github/agents/`)

- **Responsibility**: Declarative agent personas with constrained tool sets, handoff wiring, and behavioral instructions.
- **Interface**: VS Code reads `.agent.md` files; agents invoke MCP tools and hand off to each other.
- **Data ownership**: None — agents are stateless definitions.
- **Dependencies**: VS Code custom agent runtime (v1.106+).
- **Components**:
  - `loom-orchestrator.agent.md` — full Loom + GitHub MCP tools; drives the FSM loop.
  - `loom-gate.agent.md` — read-only tools; returns structured PASS/FAIL verdict as subagent.
  - `loom-debug.agent.md` — read + comment tools; posts debug analysis on failing PRs.
  - `loom-merge.agent.md` — merge tool; executes merge after gate PASS.
- **ADR**: [ADR-002](../ADRs/ADR-002-multi-agent-orchestration.md)

### 2. MCP Server (`internal/mcp`)

- **Responsibility**: Expose Loom's tools, resources, tasks, and elicitations over MCP stdio.
- **Interface**: MCP protocol (stdio transport). Five tools, five resources, task lifecycle events, elicitation schema.
- **Data ownership**: None directly — delegates to Store and FSM.
- **Dependencies**: FSM, Store, GitHub Client, DepGraph, Monitor.
- **Sub-components**:
  - **Tools**: `loom_next_step`, `loom_checkpoint`, `loom_heartbeat`, `loom_get_state`, `loom_abort`.
  - **Resources** (v2): `loom://dependencies`, `loom://state`, `loom://log`, `loom://sessions`, `loom://session/<id>`.
  - **Tasks** (v2): MCP Task wrapping for long-running `loom_heartbeat` polls.
  - **Elicitation** (v2): Structured schema on budget exhaustion.
  - **Server Instructions** (v2): Phase summary + dependency digest injected into base prompt.
  - **Session Trace Rendering** (v2): Human-readable per-session trace surface with run header, FSM ledger, and GitHub entity evolution.
- **ADR**: [ADR-003](../ADRs/ADR-003-mcp-resources.md), [ADR-004](../ADRs/ADR-004-mcp-tasks-and-elicitation.md)

### 3. FSM (`internal/fsm`)

- **Responsibility**: Deterministic state machine enforcing valid workflow transitions and retry budgets.
- **Interface**: `State()`, `Transition(event)` — pure functions, no I/O.
- **Data ownership**: Current state and retry counters (in-memory; persisted via Store).
- **Dependencies**: None (zero external deps).
- **Unchanged from v1**: 13 states, event set, transition table, budget config.

### 4. Dependency Graph (`internal/depgraph`) — NEW in v2

- **Responsibility**: Parse `.loom/dependencies.yaml`, evaluate the DAG, determine unblocked stories/epics.
- **Interface**: `Load(path) → Graph`, `Graph.Unblocked() → []StoryID`, `Graph.IsBlocked(id) → bool`.
- **Data ownership**: `.loom/dependencies.yaml` schema and validation.
- **Dependencies**: `gopkg.in/yaml.v3` for parsing.
- **ADR**: [ADR-003](../ADRs/ADR-003-mcp-resources.md)

### 5. Store (`internal/store`)

- **Responsibility**: Persist and retrieve workflow checkpoints, action log entries.
- **Interface**: `ReadCheckpoint`, `WriteCheckpoint`, `DeleteAll`, `Close` (existing). v2 adds: `WriteAction`, `ReadActions(limit)`, `CreateSessionRun`, `AppendTraceEvent`, `ListSessionRuns`, `ReadSessionTrace`.
- **Data ownership**: SQLite database at `.loom/state.db`.
- **Dependencies**: `modernc.org/sqlite`.
- **v2 additions**: Action log table for idempotency keys, plus session trace tables for operator-facing replay and post-mortem analysis.

### 6. Monitor (`internal/mcp/monitor.go`)

- **Responsibility**: Stall detection, heartbeat log emission, session health monitoring.
- **Interface**: `RunStallCheck()`, `startMonitor(ctx)` (internal).
- **Data ownership**: Last-activity timestamp (in-memory).
- **Dependencies**: FSM, Store, Clock interface.
- **Unchanged from v1**.

### 7. GitHub Client (`internal/github`)

- **Responsibility**: Typed GitHub REST API client with rate-limit handling and retry.
- **Interface**: `GitHubClient` interface (CreateIssue, ListPRs, MergePR, etc.).
- **Data ownership**: None (GitHub.com owns the data).
- **Dependencies**: `google/go-github`, `net/http`.
- **Unchanged from v1**.

### 8. Config (`internal/config`)

- **Responsibility**: Load runtime configuration from `~/.loom/config.toml` and environment variables.
- **Interface**: `Load() → Config`.
- **Data ownership**: Configuration schema.
- **Dependencies**: `pelletier/go-toml/v2`.
- **Unchanged from v1**.

### 9. CLI (`cmd/loom`)

- **Responsibility**: Human operator surface: `start`, `status`, `pause`, `resume`, `log`, `mcp`, `reset`, `version`.
- **Interface**: Shell commands via `spf13/cobra`.
- **Data ownership**: None.
- **Dependencies**: All internal packages.
- **Unchanged from v1**.

---

## Boundary Rules

1. **Agent definitions do not contain Go code** — they are declarative Markdown files consumed by VS Code.
2. **MCP Server is the only bridge** between agent sessions and Loom internals. No direct Go package imports from agents.
3. **FSM has zero external dependencies** — testable in isolation with table-driven Go tests.
4. **Store owns all durable state** — no other component writes to SQLite directly.
5. **DepGraph owns `.loom/dependencies.yaml`** — other components read the parsed graph, not the raw YAML.
6. **GitHub Client never mutates FSM state** — it returns results; the MCP server decides the FSM event.
7. **Session trace is append-only** — existing trace entries are never rewritten; retained traces are removed only by whole-session retention rules.
