# Loom v2 Architecture — System Context & High-Level Design

> Traces to: [VP2 — The Native Agent Platform](../vision_of_product/VP2-agent-platform/02-vision-agent-platform.md)

## 1. System Context

Loom is a Go CLI tool and MCP server that orchestrates autonomous software
development workflows. It bridges VS Code's multi-agent platform with GitHub's
issue/PR lifecycle.

```text
                 ┌─────────────────────────────────────────┐
                 │        Human Operator                    │
                 │  (loom start | status | pause | resume) │
                 └─────────────────┬───────────────────────┘
                                   │ CLI
                                   ▼
┌──────────────────────────────────────────────────────────────────────┐
│  .github/agents/                                                     │
│  ┌───────────────────┐  ┌──────────────┐  ┌──────────────────┐      │
│  │ loom-orchestrator  │  │ loom-gate    │  │ loom-debug       │      │
│  │ .agent.md          │→ │ .agent.md    │  │ .agent.md        │      │
│  │ (full Loom tools)  │  │ (read-only)  │  │ (comment-only)   │      │
│  └────────┬───────────┘  └──────────────┘  └──────────────────┘      │
│           │ handoffs / subagent calls                                 │
└───────────┼──────────────────────────────────────────────────────────┘
            │ MCP stdio
            ▼
┌──────────────────────────────────────────────────────────────────────┐
│  Loom MCP Server (Go binary)                                         │
│                                                                      │
│  Tools:  loom_next_step · loom_checkpoint · loom_heartbeat ·         │
│          loom_get_state · loom_abort                                  │
│                                                                      │
│  Resources:  loom://dependencies · loom://state · loom://log         │
│              loom://sessions · loom://session/<id>                  │
│                                                                      │
│  Internals:  FSM │ Store (SQLite) │ GitHub Client │ Config │ Monitor │
└──────────────────────────────────┬───────────────────────────────────┘
                                   │ GitHub REST API
                                   ▼
┌──────────────────────────────────────────────────────────────────────┐
│  GitHub.com                                                          │
│  Issues · Pull Requests · CI Checks · Copilot Coding Agent           │
└──────────────────────────────────────────────────────────────────────┘
```

## 2. High-Level Design

### 2.1 Core Principle: Plumbing vs. Intelligence

The Go binary owns **deterministic plumbing**: FSM transitions, retry budgets,
checkpoint persistence, dependency DAG evaluation, and idempotency enforcement.

VS Code custom agents own **contextual intelligence**: parsing GitHub responses,
composing issue bodies, choosing GitHub MCP tools per step, and recovering from
ambiguous states.

### 2.2 Layered Architecture

| Layer | Responsibility | Technology |
| ------- | --------------- | ------------ |
| **Agent Definitions** | Workflow personas with constrained tool sets and handoff wiring | `.github/agents/*.agent.md` |
| **MCP Server** | Tool + resource surface between agents and Loom internals | Go, `mcp-go` library, stdio transport |
| **Orchestration Core** | FSM, retry budgets, dependency DAG, session monitoring | Pure Go packages (`internal/fsm`, `internal/mcp`) |
| **Persistence** | Checkpoint store, action log, session trace, dependency graph | SQLite via `modernc.org/sqlite` |
| **GitHub Integration** | REST API client with rate-limit handling | `google/go-github`, `net/http` |
| **CLI** | Human operator commands | `spf13/cobra` |

### 2.3 v1 → v2 Evolution

VP2 is **additive** over v1. No existing interface is removed or renamed.

| v1 (ADR-001) | v2 (VP2) |
| --- | --- |
| Single master Copilot session | Multiple custom agents with handoffs |
| Workflow steps in one prompt | Workflow steps as agent handoff transitions |
| Polling in LLM context | Polling in background agent + MCP Tasks |
| Gate evaluation in LLM reasoning | Gate evaluation as isolated read-only subagent |
| `PAUSED` + manual `loom resume` | MCP elicitation with structured choices |
| No MCP resources | `loom://dependencies`, `loom://state`, `loom://log`, session trace resources |
| Sequential story execution | Parallel execution via background agents + worktrees |

### 2.4 Key Invariants

1. **FSM is the single source of truth** for workflow state. Agents read state
   via `loom_get_state`; mutations go through `loom_checkpoint` only.
2. **One checkpoint per state transition** — every FSM event is persisted before
   the agent proceeds.
3. **Retry budgets are enforced in Go**, not in prompts. Budget exhaustion
   triggers MCP elicitation, not silent failure.
4. **Agent tool sets are constrained by definition** — the gate agent cannot
   write; the debug agent cannot merge.
5. **Dependencies are machine-readable** — `.loom/dependencies.yaml` is the
   canonical store; MCP resources surface it to agents.
6. **Session traces are derived audit artifacts** — they are append-only,
   operator-facing observability surfaces backed by SQLite, not the source of
   truth for orchestration state.

## 3. Cross-References

| Document | Content |
| ---------- | --------- |
| [tech-stack.md](tech-stack.md) | Technology choices and rationale |
| [components.md](components.md) | Component breakdown and boundaries |
| [data-model.md](data-model.md) | Data entities and persistence |
| [project-setup.md](project-setup.md) | Repository structure and build |
| [deployment.md](deployment.md) | Distribution and operational concerns |
| [ADR-001](../adr/ADR-001-loom-local-orchestrator.md) | Original Loom architecture decision |
| [ADR-002](../ADRs/ADR-002-multi-agent-orchestration.md) | Multi-agent orchestration via custom agents |
| [ADR-003](../ADRs/ADR-003-mcp-resources.md) | MCP resources for observability and dependency graph |
| [ADR-004](../ADRs/ADR-004-mcp-tasks-and-elicitation.md) | MCP Tasks and elicitation for resilient polling |
| [ADR-005](../ADRs/ADR-005-parallel-execution.md) | Parallel execution via background agents and worktrees |
| [ADR-006](../ADRs/ADR-006-security-model.md) | Security: tool eligibility, auth, org registry |
| [ADR-007](../ADRs/ADR-007-session-trace-resource-and-storage.md) | Session trace storage, audit model, and operator surface |
