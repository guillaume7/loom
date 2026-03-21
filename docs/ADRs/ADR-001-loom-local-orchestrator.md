# ADR-001: Loom — Local Automated Orchestrator

| Field | Value |
|---|---|
| **Status** | Superseded by ADR-008 |
| **Date** | 2026-03-07 |
| **Deciders** | Guillaume Riflet |
| **Relates to** | WORKFLOW_GITHUB.md, Loop Driver (deprecated) |

---

## Context

Software development workflows are decomposed into phases (E1–E8) executed in a
defined sequence. Each phase follows a five-step cross-process workflow
documented in [WORKFLOW_GITHUB.md](../../.github/squad_prompts/WORKFLOW_GITHUB.md):

1. **Orchestrate** — scan status, identify the next unblocked phase
2. **Implement** — create an issue, assign `@copilot`, wait for a PR
3. **Review** — request a Copilot code review, handle feedback loops
4. **Debug** — create a debug sub-issue if CI fails
5. **Refactor** — run a cleanup sweep at epic boundaries

This workflow previously required a human to perform each step manually. The
cadence was limited by human availability and attention.

A previous attempt to automate this via GitHub Actions
("Loop Driver", `loop-driver.yml`) failed due to:

- Dependence on undocumented/unstable Copilot APIs that silently fail
- A fragile label-based state machine with no local testability
- No observability — errors were swallowed inside `actions/github-script`
- GitHub Actions is a CI tool, not an orchestration engine

The Loop Driver was deprecated in March 2026 after ~15 failed fix iterations.

### Forces

- **Speed**: the human is the bottleneck in a workflow where each step is
  well-defined and deterministic.
- **Reliability**: the orchestrator must not silently fail — every state
  transition must be observable and resumable.
- **Testability**: the state machine must be verifiable locally, without pushing
  to GitHub and waiting for Actions to run.
- **Intelligence**: some steps (composing issue bodies, analysing review
  feedback) benefit from LLM reasoning, not just API calls.
- **Compatibility**: the solution must work with the existing WORKFLOW_GITHUB.md
  process and the agent squad defined in `.github/agents/`.

---

## Decision

Build **Loom**, a Go CLI tool that runs locally and drives the
WORKFLOW_GITHUB.md playbook autonomously, with no human in the loop.

### Architecture

Loom has three layers:

1. **Go binary** — a deterministic state machine with retry budgets, GitHub API
   polling, checkpoint persistence (SQLite), and structured logging. Runs as an
   **MCP server** registered in `.github/copilot/mcp.json`.

2. **Master Copilot session** — a VS Code Copilot Agent session that acts as the
   Orchestrator. It calls Loom's MCP tools (`loom_next_step`, `loom_checkpoint`,
   `loom_heartbeat`) in a loop. Between tool calls, it uses the GitHub MCP
   server (`mcp_io_github_git_*`) to create issues, assign `@copilot`, post
   comments, and merge PRs.

3. **GitHub.com** — the `@copilot` coding agent (invoked via issue assignment)
   implements code, CI validates it, and the Copilot reviewer reviews PRs.

### Why This Design

| Concern | How Loom Addresses It |
|---|---|
| **Separation of plumbing and intelligence** | Go binary handles deterministic state transitions; Copilot session handles contextual reasoning |
| **Local testability** | The FSM is a pure Go package with zero external dependencies — fully unit-testable |
| **Observability** | Structured JSON log for every transition; `loom status` and `loom log` CLI commands |
| **Resumability** | SQLite checkpoint store survives process/machine restarts |
| **Session keepalive** | MCP tool calls keep the Copilot session active; the Go binary detects connection loss and writes state before exiting |
| **Failure containment** | Retry budgets on every async gate; PAUSED escape state prevents infinite loops |
| **Stable APIs only** | Uses `mcp_io_github_git_*` (official MCP tools) and GitHub REST API; no undocumented endpoints |

### Technology

| Component | Technology | Rationale |
|---|---|---|
| Language | Go (latest stable) | Single binary; strong HTTP/JSON stdlib; `google/go-github` library |
| Session integration | MCP stdio server | Native VS Code Copilot support; no custom extension needed |
| State persistence | SQLite (`modernc.org/sqlite`) | Pure Go; no CGo; single file |
| Logging | `log/slog` (stdlib) | Structured JSON; zero dependencies |
| CLI | `spf13/cobra` | Standard Go CLI framework |

### State Machine

```
IDLE → SCANNING → ISSUE_CREATED → AWAITING_PR → AWAITING_READY
  → AWAITING_CI → { REVIEWING | DEBUGGING } → MERGING → REFACTORING
  → SCANNING (next phase) → ... → COMPLETE
```

Failure branches: DEBUGGING (CI failure), ADDRESSING_FEEDBACK (review
rejection). Both loop back to AWAITING_CI. Gate-specific retry budgets prevent
infinite loops, transitioning to PAUSED on exhaustion.

Full state machine: [analysis.md § 5](../loom/analysis.md#5-state-machine).

---

## Consequences

### Positive

- **Zero human intervention** for the steady-state development loop.
- **Locally testable** — the FSM can be exercised with table-driven Go tests,
  no GitHub dependency.
- **Observable** — every state transition, API call, and retry is logged with
  structured data.
- **Resumable** — kill and restart at any point; Loom picks up from the last
  checkpoint.
- **No undocumented APIs** — all GitHub interactions use official MCP tools or
  the public REST API.

### Negative

- **MCP server maturity** — VS Code's MCP host is an evolving feature.
  Breaking changes to the MCP protocol or tool-calling behaviour could require
  Loom updates.
- **LLM reliability** — the master session must reliably follow the
  `loom_next_step` → execute → `loom_checkpoint` loop. If the model diverges,
  the loop stalls. Mitigation: stall detection + session restart.
- **Session context limits** — long-running phases may exhaust the Copilot
  session's context window. Mitigation: checkpoint after each step.

### Neutral

- The Loop Driver workflow and its checkpoint issue template remain deprecated.
  Loom is the replacement, not an evolution.
- The manual WORKFLOW.md remains valid for exploratory and debugging sessions.
  Loom automates WORKFLOW_GITHUB.md only.

---

## Alternatives Considered

### A. Fix the Loop Driver

Continue iterating on the GitHub Actions state machine.

**Rejected**: After 15 failed iterations, the fundamental issues (no local
testability, undocumented APIs, YAML state machine complexity) are structural.

### B. Pure Go tool with direct GitHub API calls (no Copilot session)

The Go binary composes all issue bodies from templates without LLM involvement.

**Rejected**: Composing issue bodies requires reading squad prompts, agent files,
and skill files and assembling context — this is exactly what an LLM excels at.

### C. VS Code Extension (TypeScript)

Build a VS Code extension that manages the session lifecycle entirely in
TypeScript.

**Rejected**: Tighter coupling to VS Code internals; harder to test outside the
editor; MCP server achieves the same integration with less coupling.

### D. External orchestrator (Temporal, Prefect, etc.)

Use a dedicated workflow orchestration platform.

**Rejected**: Massively over-engineered for a single-project tool. Contradicts
the "single binary" philosophy.

---

## Related Documents

- [Loom Analysis](../loom/analysis.md) — full architecture, state machine, risks
- [WORKFLOW_GITHUB.md](../../.github/squad_prompts/WORKFLOW_GITHUB.md) — the workflow Loom automates
- [SQUAD.md](../../.github/SQUAD.md) — agent roster
