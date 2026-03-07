# Loom — Autonomous Development Orchestrator

> *A loom weaves individual threads into fabric following a pattern.
> This tool weaves agents, skills, and GitHub workflows into working software
> following the WORKFLOW_GITHUB.md playbook — with no human in the loop.*

---

## 1. Executive Summary

**Loom** is a Go CLI tool that drives software development workflows
autonomously. It manages a persistent VS Code Copilot Agent session (the
"master session") that follows
[WORKFLOW_GITHUB.md](../../.github/squad_prompts/WORKFLOW_GITHUB.md) end-to-end:
scanning project status, creating phase issues, assigning `@copilot`, polling
for PRs, requesting reviews, handling debug and refactor cycles, merging, and
advancing to the next phase — all without human intervention.

A deterministic state machine in the Go binary provides the reliable plumbing
(state transitions, GitHub API polling, session keepalives, checkpointing),
while the Copilot session provides the intelligence (reading project context,
composing prompts, analysing review feedback).

## 2. Background

### 2.1 The Loop Driver Experiment

The VECTOR project previously attempted a GitHub Actions-based "Autopoietic
Loop Driver" (`loop-driver.yml`) to automate the development cycle. The workflow
used a YAML state machine built on label transitions and `@copilot` directive
comments to chain: CI → review → refactor → merge. A permanent GitHub issue
served as a JSON checkpoint store.

### 2.2 Why It Failed

| Root Cause | Impact |
|---|---|
| **Undocumented Copilot APIs** (`copilot_reviews`, `copilot/issues`) silently fail | Review and assignment steps appeared to succeed but did nothing |
| **No local testability** — the state machine only ran inside GitHub Actions | Every fix required push→wait→observe→retry cycles |
| **Labels as state** — fragile, race-prone, invisible without the issue page | State corruption was common and hard to diagnose |
| **No observability** — errors were swallowed inside `actions/github-script` | Failures produced misleading "success" comments |
| **Wrong tool for the job** — GitHub Actions is a CI executor, not an orchestration engine | The YAML became a tangled mess of conditionals |

After ~15 failed iterations, the Loop Driver was deprecated (March 2026).

### 2.3 Key Lessons

1. **Separate plumbing from intelligence.** A state machine should be
   deterministic and testable. AI provides the intelligence within each state,
   not the state transitions themselves.
2. **Run locally first.** If you can't test the orchestration loop on your
   laptop, you cannot debug it.
3. **Use official, stable APIs only.** Never depend on undocumented endpoints
   for critical-path operations.
4. **Keep state in a real store.** A JSON blob in an issue body is not a
   database.

---

## 3. Requirements

### 3.1 Functional

| ID | Requirement |
|---|---|
| **F1** | Drive all development phases (E1–E8) following the sequence in `docs/epics/README.md` |
| **F2** | Execute each phase via the 5-step cross-process workflow in WORKFLOW_GITHUB.md: Orchestrate → Implement → Review → Debug → Refactor |
| **F3** | Handle failure loops: CI failure → debug issue → fix → re-review; review rejection → feedback → re-review |
| **F4** | Resume from any checkpoint after session termination (Copilot crash, VS Code restart, machine reboot) |
| **F5** | Compose issue bodies dynamically by reading squad prompts, agent definitions, and skill files from the repository |
| **F6** | Merge PRs only after CI is green and review is approved |
| **F7** | Run a post-epic refactor sweep at each epic boundary |
| **F8** | Provide a `status` command that prints the current state, phase, and recent activity |

### 3.2 Non-Functional

| ID | Requirement |
|---|---|
| **NF1** | **Observable** — structured JSON logging for every state transition and GitHub API call |
| **NF2** | **Testable** — the state machine can be exercised in unit tests without any GitHub or Copilot dependency |
| **NF3** | **Idempotent** — safe to restart at any point; re-running a completed step is a no-op |
| **NF4** | **Minimal footprint** — single Go binary, no Docker required for dev use |
| **NF5** | **Resumable** — persistent state stored in a local SQLite database that survives restarts |

---

## 4. Architecture

### 4.1 Component Overview

```
┌──────────────────────────────────────────────────────────────────────┐
│  VS Code                                                             │
│                                                                      │
│  ┌────────────────────────────────────┐   ┌───────────────────────┐  │
│  │  Copilot Agent Session (Master)    │   │  GitHub MCP Server    │  │
│  │  • Reads agents/ & skills/         ├──▶│  (mcp_io_github_git_*)│  │
│  │  • Composes issue bodies           │   │  • Create issues      │  │
│  │  • Analyses review feedback        │   │  • Assign @copilot    │  │
│  │  • Decides next action             │   │  • Read PR state      │  │
│  └──────────┬─────────────────────────┘   │  • Post comments      │  │
│             │ MCP tool calls              │  • Merge PRs          │  │
│  ┌──────────▼─────────────────────────┐   └───────────────────────┘  │
│  │  Loom MCP Server (Go binary)       │                              │
│  │  • Exposes: next_step, checkpoint, │                              │
│  │    heartbeat, get_state, abort     │                              │
│  │  • Runs state machine internally   │                              │
│  │  • Polls GitHub API for gates      │                              │
│  │  • Persists state to SQLite        │                              │
│  └──────────┬─────────────────────────┘                              │
│             │                                                        │
│  ┌──────────▼─────────────────────────┐                              │
│  │  SQLite Checkpoint Store           │                              │
│  │  (.loom/state.db)                  │                              │
│  └────────────────────────────────────┘                              │
└──────────────────────────────────────────────────────────────────────┘
         │
         │  GitHub REST API (via MCP + direct polling)
         ▼
┌──────────────────────────────────────────────────────────────────────┐
│  GitHub.com                                                          │
│                                                                      │
│  Issues ──▶ @copilot coding agent ──▶ Draft PRs ──▶ CI ──▶ Reviews  │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

### 4.2 The Three Layers

| Layer | Runs As | Responsibility |
|---|---|---|
| **Copilot Master Session** | VS Code Agent chat | Intelligence — reads project files, composes issue bodies, analyses feedback, decides what prompt to send to which agent |
| **Loom Go Binary** | MCP server (stdio) | Plumbing — state machine, GitHub API polling for async gates, session keepalive, checkpointing, structured logging |
| **GitHub.com** | Cloud | Execution — `@copilot` coding agent implements code, CI validates, Copilot reviewer reviews |

### 4.3 Why an MCP Server?

The Go binary runs as a **Model Context Protocol (MCP) server** registered in
`.github/copilot/mcp.json`. This gives us:

1. **Native VS Code integration** — no custom extension required.
2. **Bidirectional control** — the Copilot session calls Loom tools; Loom tools
   return instructions that steer the session's next action.
3. **Session keepalive via tool calls** — each `loom_next_step` /
   `loom_checkpoint` call is activity that prevents the session from timing out.
4. **Clean separation** — the Go binary handles deterministic logic; the LLM
   handles contextual reasoning.

### 4.4 MCP Tool Surface

| Tool | Purpose |
|---|---|
| `loom_next_step` | Returns a structured instruction for the current state: which WORKFLOW_GITHUB.md step to execute, with parameters (issue template, branch pattern, agent names, etc.) |
| `loom_checkpoint` | Called by the session after completing a step. Receives the outcome. Advances the state machine and persists the new state. |
| `loom_heartbeat` | Lightweight keepalive. Returns current state without advancing. Call during long waits (CI polling, review polling). |
| `loom_get_state` | Returns full state for debugging: current phase, step, pending gates, transition history. |
| `loom_abort` | Emergency stop. Sets the state to PAUSED and returns a summary. |

### 4.5 Session Lifecycle & Keepalive

```
LOOP:
1. Call loom_next_step to get your next task.
2. Execute the task using GitHub MCP tools.
3. Call loom_checkpoint with the result.
4. If loom_next_step returns COMPLETE, stop.
5. During waits (CI, review), call loom_heartbeat every 30–60 seconds.
6. GOTO 1.
```

**Keepalive mechanism**: Loom tool calls themselves are the heartbeat. During
async gates (waiting for `@copilot` to open a PR, waiting for CI, waiting for
review), the session calls `loom_heartbeat` on a timer. This prevents VS Code
from treating the session as idle.

**Session death recovery**: If the session dies despite keepalives, the Go
binary detects the MCP connection drop and writes the current state to the
persistent store. When `loom start` runs again, it reads the last checkpoint
and instructs the new session to resume from that point.

---

## 5. State Machine

### 5.1 States

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

### 5.2 State Transition Table

| From | Event | To | Action |
|---|---|---|---|
| IDLE | `start` | SCANNING | Read epics, list open PRs/issues |
| SCANNING | `phase_identified(N)` | ISSUE_CREATED | Create issue from template, assign @copilot |
| SCANNING | `all_phases_done` | COMPLETE | Tag release, log completion |
| ISSUE_CREATED | `copilot_assigned` | AWAITING_PR | Begin polling for PR |
| AWAITING_PR | `pr_opened(pr_num)` | AWAITING_READY | Record PR number |
| AWAITING_PR | `timeout(10min)` | AWAITING_PR | Nudge @copilot via issue comment |
| AWAITING_READY | `pr_ready` | AWAITING_CI | PR is no longer draft |
| AWAITING_READY | `timeout(30min)` | AWAITING_READY | Force-promote draft → ready |
| AWAITING_CI | `ci_green` | REVIEWING | Request Copilot review |
| AWAITING_CI | `ci_red` | DEBUGGING | Create debug issue |
| REVIEWING | `review_approved` | MERGING | Merge the PR |
| REVIEWING | `review_changes_requested` | ADDRESSING_FEEDBACK | Post blockers as comment |
| DEBUGGING | `fix_pushed` | AWAITING_CI | Loop back to CI check |
| ADDRESSING_FEEDBACK | `feedback_addressed` | AWAITING_CI | Loop back to CI check |
| MERGING | `merged` | REFACTORING | If epic boundary; else → SCANNING |
| MERGING | `merged` | SCANNING | If not an epic boundary |
| REFACTORING | `refactor_merged` | SCANNING | Next phase |

### 5.3 Failure Budget

| Gate | Max Retries | On Exhaustion |
|---|---|---|
| AWAITING_PR | 20 (= 10 min at 30s) | → PAUSED |
| AWAITING_READY | 60 (= 30 min) | Force-promote, then proceed |
| AWAITING_CI | 20 (= 10 min) | → PAUSED |
| DEBUGGING → AWAITING_CI loop | 3 full cycles | → PAUSED |
| REVIEWING → ADDRESSING_FEEDBACK loop | 5 full cycles | → PAUSED |

---

## 6. Mapping to WORKFLOW_GITHUB.md

| WORKFLOW_GITHUB.md Step | Loom States |
|---|---|
| 🔵 **Step 1 — Orchestrate** | SCANNING |
| 🟢 **Step 2 — Implement** | ISSUE_CREATED → AWAITING_PR → AWAITING_READY → AWAITING_CI |
| 🟡 **Step 3 — Review** | REVIEWING → (ADDRESSING_FEEDBACK →) MERGING |
| 🔴 **Step 4 — Debug** | DEBUGGING → AWAITING_CI (loop) |
| 🟣 **Step 5 — Refactor** | REFACTORING |

---

## 7. CLI Commands

| Command | Description |
|---|---|
| `loom start` | Begin from IDLE or resume from last checkpoint |
| `loom status` | Print current state, phase, PR, and recent log |
| `loom pause` | Gracefully pause at the next safe checkpoint |
| `loom resume` | Continue from PAUSED state |
| `loom reset` | Clear all state (with confirmation prompt) |
| `loom log` | Stream structured JSON log output |
| `loom mcp` | Start MCP stdio server (called internally by VS Code) |

---

## 8. Project Layout

```
cmd/loom/              ← CLI entry point: parses args, wires dependencies
internal/
  fsm/                 ← Pure state machine — NO external deps, fully unit-testable
    machine.go
    states.go
    events.go
    machine_test.go
  github/              ← GitHub REST API wrapper
    client.go          ← Implements GitHubClient interface
    types.go
    client_test.go     ← httptest-based fixture tests
  mcp/                 ← MCP stdio server
    server.go
    tools.go           ← Tool handlers: next_step, checkpoint, heartbeat, get_state, abort
    server_test.go
  store/               ← SQLite checkpoint persistence
    sqlite.go          ← Implements Store interface
    sqlite_test.go
  config/              ← Config file loading, defaults
    config.go
```

---

## 9. Technology Choices

| Choice | Rationale |
|---|---|
| **Go** | Single binary; fast startup; strong HTTP/JSON stdlib; `google/go-github` library; `mark3labs/mcp-go` for MCP server |
| **MCP (stdio)** | First-class VS Code Copilot integration; no custom extension; no IPC hacking |
| **SQLite (`modernc.org/sqlite`)** | Pure Go driver, no CGo; single file; proven at scale |
| **`log/slog` (stdlib)** | Structured JSON logging; zero dependencies; Go 1.21+ |
| **`spf13/cobra`** | Standard Go CLI framework; subcommand routing |

---

## 10. Risks & Mitigations

| # | Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|---|
| R1 | Copilot session ignores the `loom_next_step` loop instruction | Medium | High | Reinforce in system prompt; detect stall in Go binary; restart session |
| R2 | VS Code MCP stdio connection drops silently | Medium | Medium | Detect EOF on stdin, write checkpoint, exit with non-zero code |
| R3 | `@copilot` on GitHub.com fails to create a PR | Medium | Medium | Retry budget + nudge comment; after exhaustion → PAUSED |
| R4 | Copilot review endpoint is unreliable | High | Medium | Use review-request comment as fallback; document degraded-mode operation |
| R5 | Rate limiting on GitHub API | Low | Medium | Respect `X-RateLimit-Remaining`; back off exponentially |
| R6 | FSM bug causes infinite loop | Low | High | Retry budgets on every gate; PAUSED escape state; comprehensive unit tests |
| R7 | Session context window exhaustion | Medium | Medium | Checkpoint after each step; `loom_next_step` returns minimal context |

---

## 11. Open Questions

| # | Question | Options |
|---|---|---|
| Q1 | How to programmatically start a VS Code Copilot Agent session from CLI? | (a) `code --chat` CLI, (b) VS Code activation command, (c) user starts manually |
| Q2 | Should Loom poll GitHub directly alongside the Copilot session calling GitHub MCP tools? | (a) Hybrid: Loom polls gates, session handles intelligent tasks, (b) Session-only |
| Q3 | What happens when context window fills mid-phase? | (a) Checkpoint + restart, (b) Summarise history |
| Q4 | Should `loom start` block terminal (foreground) or daemonise? | (a) Foreground with Ctrl-C, (b) Background daemon |
| Q5 | Can MCP stdio servers persist goroutines between tool calls in VS Code? | Needs verify — affects whether Loom polls internally or relies on session polling |

---

## 12. Success Criteria

Loom v1.0 is successful when:

1. Running `loom start` in a configured project repository results in all
   phases being implemented, reviewed, and merged — with no human intervention.
2. Each phase produces a merged PR with green CI and an approved review.
3. Loom can be killed and restarted mid-phase and resumes correctly.
4. The full run produces structured logs tracing every state transition and
   GitHub API call.

---

*Next step: [ADR-001 — Adopt Loom as the local automated orchestrator](../adr/ADR-001-loom-local-orchestrator.md)*
