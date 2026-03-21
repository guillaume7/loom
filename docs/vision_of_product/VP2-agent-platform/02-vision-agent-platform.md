# Loom — Vision 2: The Native Agent Platform

> *VS Code is no longer a tool host. It is a multi-agent operating system.
> Loom v2 is its scheduler.*

**Status:** Proposed  
**Supersedes:** Partially — `01-vision.md` remains valid for the current implementation.  
**Context:** VS Code Copilot v1.99–v1.107 (March–November 2025) introduced a
new class of agent primitives. This document reinterprets Loom's architecture
and roadmap in light of those primitives.

---

## 1. The Platform Shift

When Loom v1 was designed, VS Code Copilot was a smart text editor with an
optional MCP server slot. The mental model was simple: Loom sits in the MCP
server slot; the one Copilot agent session calls Loom tools in a loop.

That model is now outdated.

Between March and November 2025, VS Code shipped — in rapid succession:

| Feature | Release | Significance |
|---|---|---|
| Custom chat modes | v1.101 | Constrained agent personas, per-workflow tool sets |
| MCP sampling + elicitation | v1.101–v1.102 | MCP servers can call back to the LLM and request human input |
| `code chat` CLI | v1.102 | External processes can spawn agent sessions programmatically |
| Chat checkpoints | v1.103 | Session state can be snapshotted and restored |
| Handoffs | v1.105 | Agents chain to other agents with pre-filled context |
| Isolated subagents | v1.105 | Agents spawn focused sub-workers with separate context windows |
| Custom agents (`.github/agents/`) | v1.106 | Agents are version-controlled, workspace-aware, portable to GitHub cloud |
| Local agent sessions remain active | v1.107 | Sessions run in background even when the UI tab is closed |
| Background agents + Git worktrees | v1.107 | Parallel autonomous agents with isolated working trees |
| Continue in (local → cloud) | v1.107 | Seamless escalation from local session to GitHub Copilot coding agent |
| MCP Tasks | v1.107 spec | Long-running, resilient tool calls with explicit lifecycle |
| Organization-level custom agents | v1.107 | Centrally managed agent definitions shared across teams |

Taken together, these features constitute a **first-class multi-agent operating
system** inside VS Code. Agents can now:

- Run in the background without a focused UI tab
- Be defined as portable files in `.github/agents/`
- Hand off to each other with structured workflow transitions
- Delegate to isolated sub-processes with their own context windows
- Escalate from local to cloud when the task demands it
- Run in separate Git worktrees to avoid state collisions
- Use MCP Tasks for resilient, crash-resumable tool calls

The gap between what VS Code's agent platform offers and what Loom needs is
sharply narrower than the original gap analysis described.

---

## 2. Gap-by-Gap Bridges

The original gap analysis (`docs/loom/gap_analysis.md`) identified six blockers
for full autonomy. Here is the status of each, mapped to new VS Code features.

### Gap 1: Event/Timer Runtime

**Original problem:** A persistent process is needed to wake Loom on webhook
events or polling intervals. Without one, Loom stalls in waiting states.

**New bridges:**

1. **Background agents** (v1.107) — a local agent session stays alive even
	 after the UI tab is closed. Loom's FSM polling loop can run unattended.

2. **`code chat` CLI** (v1.102) — Loom's Go binary can spawn a new agent
	 session via `code chat -m agent "resume FSM from state X"` in response to a
	 webhook, a cron job, or a file-system event. The Loom binary is the timer;
	 VS Code is the executor.

3. **MCP Tasks** (v1.107 MCP spec) — long-running Loom tool calls
	 (`loom_next_step` while waiting for CI) declare themselves as Tasks, giving
	 the client explicit lifecycle events (started / progress / done) rather than
	 a silent blocking call.

**Verdict:** Closed. Loom's Go binary continues to own the timer/polling logic,
but can now delegate execution to a persistent background agent. No separate
daemon process is needed.

---

### Gap 2: Deterministic Gate Evaluator

**Original problem:** A strict policy engine must decide "safe to merge" vs
"hold" based on CI state, reviews, mergeability, and the dependency DAG.

**New bridges:**

1. **Custom agents with constrained tool sets** — a `loom-gate.agent.md`
	 custom agent can be defined with only read tools (`search`, no `shell`, no
	 `editFiles`) and explicit gate criteria in its instructions. The LLM is
	 constrained to return a structured verdict, not take action.

2. **Handoffs** — the Gate agent hands off to `loom-merge.agent.md` only if
	 the gate verdict is PASS, pre-filling the prompt with the PR number. A FAIL
	 verdict hands off to `loom-debug.agent.md`.

3. **Subagents** (v1.105+) — the main Loom orchestrator agent can invoke the
	 gate evaluator as an isolated subagent (`#runSubagent loom-gate`) with its
	 own context window. The subagent returns a single structured result; the
	 orchestrator does not share the evaluator's reasoning context.

4. **MCP tool annotations** — Loom's read-only gate tools (`loom_get_state`,
	 `loom_heartbeat`) already carry `readOnlyHint: true`. VS Code auto-approves
	 these, removing the human confirmation bottleneck during gate polling.

**Verdict:** Closed. The gate evaluator becomes a first-class custom agent
definition, not ad-hoc prompt engineering in the master session.

---

### Gap 3: Dependency Graph Source of Truth

**Original problem:** Loom needs machine-readable dependencies, not prose in
issue bodies. Recommended: `.loom/dependencies.yaml`.

**New bridges:**

1. **MCP resources** — Loom's MCP server can expose `.loom/dependencies.yaml`
	 as a browseable resource. Any agent session can attach it as context with
	 `#resources loom/dependencies`.

2. **MCP server instructions** (v1.104) — Loom's MCP server can include a
	 concise dependency summary in its server instructions, which VS Code
	 automatically injects into the base prompt of every session that activates
	 Loom.

3. **AGENTS.md / nested AGENTS.md** (v1.104–v1.105) — a top-level `AGENTS.md`
	 can include a human-readable dependency section. Loom's tooling can keep
	 `.loom/dependencies.yaml` and `AGENTS.md` in sync.

**Verdict:** Closed. `.loom/dependencies.yaml` is the canonical store;
MCP resources and AGENTS.md surface it to agents automatically.

---

### Gap 4: Idempotency and Concurrency Control

**Original problem:** Operation keys to avoid duplicate actions; per-PR locks
to prevent two loops acting on the same PR simultaneously.

**New bridges:**

1. **Background agents in Git worktrees** (v1.107) — each background agent
	 session operates in an isolated worktree. Two Loom sessions cannot collide
	 on the same working tree by construction.

2. **Loom SQLite action log** — remains the canonical idempotency store.
	 Loom's MCP tools check the log before executing any write operation.

3. **Chat checkpoints** (v1.103) — session state snapshots let Loom roll back
	 to a known-good point without re-executing completed steps.

4. **Post-approval for external data** (v1.106) — tool calls that pull in
	 external data (e.g., CI results, PR body from GitHub) go through
	 post-approval, letting the operator catch unexpected content before it
	 affects the FSM transition.

**Verdict:** Substantially closed. The combination of worktree isolation +
SQLite action log + checkpoints provides sufficient idempotency for the current
workflow. Per-PR locks remain valuable in the store layer for multi-session
scenarios.

---

### Gap 5: Failure-Handling Policy

**Original problem:** Bounded retry counts, automatic debug escalation,
escalation comments; recovery from bad states.

**New bridges:**

1. **Handoffs with explicit failure branches** — a `loom-ci-watch.agent.md`
	 agent, upon detecting red CI, hands off to `loom-debug.agent.md` with the
	 failing check run attached as context. This replaces ad-hoc
	 "if CI red, post debug comment" prompt logic with a structured workflow
	 transition.

2. **Todo list tool** (v1.104+) — each phase of the Loom FSM is tracked as a
	 named todo item. The operator can see at a glance which step failed and
	 at what retry count.

3. **OS notifications** (v1.103+) — when a Loom session requires human
	 confirmation (retry budget exhausted, ambiguous gate verdict), VS Code
	 fires an OS notification even when the window is not focused.

4. **MCP elicitations** (v1.102) — Loom's MCP server, on budget exhaustion,
	 issues a structured elicitation prompt (e.g., "PR #42 has failed CI 3 times.
	 Choices: [Skip] [Re-assign] [Pause epic]"). The operator responds from
	 within the chat UI.

5. **MCP Tasks** (v1.107 spec) — long-running waits (CI polling) are declared
	 as Tasks with explicit progress events. If the client disconnects, the
	 task can be resumed without re-sending the initial prompt.

6. **Chat checkpoints** (v1.103) — Loom can checkpoint before each destructive
	 operation (merge, force-close issue). If the operation produces an
	 unexpected result, the operator can restore the previous session state.

**Verdict:** Closed. Failure policy is now expressible as structured agent
handoffs and MCP elicitations, not embedded in prompt templates.

---

### Gap 6: Auth and Permission Hardening

**Original problem:** Token/app permissions, branch protection, merge strategy
alignment.

**New bridges:**

1. **MCP auth — CIMD and WWW-Authenticate** (v1.106–v1.107) — Loom's MCP
	 server can use the Client ID Metadata Document auth flow for remote
	 connections and can request scope escalation dynamically via
	 `WWW-Authenticate` headers when a write operation requires elevated
	 permissions.

2. **Organization-level MCP registry** (v1.106) — Loom can be published to the
	 organization's private MCP registry, ensuring all team sessions use the
	 same vetted server version.

3. **Enterprise tool eligibility policy** (v1.107) — `loom_checkpoint` and
	 `loom_abort` can be marked ineligible for auto-approval via
	 `chat.tools.eligibleForAutoApproval`. Write operations always require
	 explicit human approval.

4. **`loom_heartbeat` + `loom_get_state`** already carry `readOnlyHint: true`
	 and are auto-approved; no token escalation is needed for polling.

**Verdict:** Closed. MCP's mature auth story (CIMD, scope escalation,
org registry) maps cleanly onto Loom's permission requirements.

---

## 3. Revised Architecture

```
┌─────────────────────────────────────────────────────────┐
│  .github/agents/                                         │
│  ┌──────────────────┐  ┌──────────────────┐             │
│  │ loom-orchestrator│  │ loom-gate        │             │
│  │ .agent.md        │  │ .agent.md        │             │
│  │ target: vscode   │  │ target: vscode   │             │
│  │ tools: [loom/*]  │  │ tools: [read/*]  │             │
│  │ handoffs:        │  │ (subagent only)  │             │
│  │  - gate → merge  │  └──────────────────┘             │
│  │  - gate → debug  │                                   │
│  └──────────────────┘  ┌──────────────────┐             │
│                        │ loom-debug       │             │
│  ┌──────────────────┐  │ .agent.md        │             │
│  │ loom-merge       │  │ target: vscode   │             │
│  │ .agent.md        │  │ tools: [read/*,  │             │
│  │ target: vscode   │  │  github/comment] │             │
│  └──────────────────┘  └──────────────────┘             │
└──────────────────────────────┬──────────────────────────┘
															 │ calls
															 ▼
┌─────────────────────────────────────────────────────────┐
│  Loom MCP Server (Go binary, stdio)                      │
│                                                          │
│  Tools:                                                  │
│  • loom_next_step     (readOnlyHint: false)              │
│  • loom_checkpoint    (side-effect: FSM advance)         │
│  • loom_heartbeat     (readOnlyHint: true, auto-approve) │
│  • loom_get_state     (readOnlyHint: true, auto-approve) │
│  • loom_abort         (readOnlyHint: false)              │
│                                                          │
│  Resources:                                              │
│  • loom://dependencies   (.loom/dependencies.yaml)       │
│  • loom://state          (current FSM state + history)   │
│  • loom://log            (structured action log)         │
│                                                          │
│  Server Instructions: phase summary + dependency digest  │
│                                                          │
│  FSM + SQLite (unchanged)                                │
└──────────────────────────────┬──────────────────────────┘
															 │ GitHub REST / GitHub MCP
															 ▼
┌─────────────────────────────────────────────────────────┐
│  GitHub.com                                              │
│  Issues · PRs · CI Checks · Copilot Coding Agent        │
└─────────────────────────────────────────────────────────┘
```

### Key Architectural Changes from v1

| v1 | v2 |
|---|---|
| One master session, one `.github/copilot-instructions.md` | Multiple focused custom agents in `.github/agents/`, each with constrained tools |
| Workflow steps embedded in one long prompt | Workflow steps as handoff transitions between agents |
| Polling loop in LLM context | Polling loop in background agent (session remains active when closed) |
| Gate evaluation in LLM reasoning | Gate evaluation as isolated subagent returning structured verdict |
| Failure handling with `PAUSED` + human re-run | Failure policy as named handoff branch + MCP elicitation |
| MCP resources: none | MCP resources: `loom://dependencies`, `loom://state`, `loom://log` |
| Auth: GitHub token in env | Auth: MCP CIMD / org registry / tool eligibility policy |

---

## 4. New Tool Surface

### 4.1 MCP Resource Additions

Loom's MCP server gains three resources beyond the existing five tools:

| Resource URI | Content | Description |
|---|---|---|
| `loom://dependencies` | YAML | `.loom/dependencies.yaml` — machine-readable epic/issue dependency graph |
| `loom://state` | JSON | Current FSM state, active PR, retry counts, last action timestamp |
| `loom://log` | NDJSON | Structured action log (last 200 entries) |

### 4.2 MCP Task Wrapping

Blocking `loom_heartbeat` calls that poll CI or PR status shall be wrapped as
MCP Tasks (MCP spec 2025-11-25):

```json
{
	"type": "task/start",
	"id": "loom-ci-poll-pr-42",
	"title": "Watching CI for PR #42",
	"cancellable": true
}
```

Progress events fire every polling interval. The client can disconnect and
reconnect without losing the polling state.

### 4.3 Elicitation Schema

When a retry budget is exhausted, Loom issues a structured elicitation:

```json
{
	"type": "elicitation",
	"title": "PR #42 — CI budget exhausted",
	"description": "check_suite 'build' has failed 5 times. Choose an action.",
	"schema": {
		"action": {
			"type": "string",
			"enum": ["skip", "reassign", "pause_epic"],
			"enumDescriptions": [
				"Skip this user story and advance to the next",
				"Re-assign the PR to a fresh @copilot session",
				"Pause the epic and require human intervention"
			]
		}
	}
}
```

No `PAUSED` state is entered until the operator responds.

---

## 5. Custom Agent Definitions

All Loom agent files live in `.github/agents/`. Each file has `target: vscode`
and is compatible with GitHub Copilot cloud agents for future escalation.

### 5.1 Orchestrator Agent

```markdown
---
name: Loom Orchestrator
description: Drives the Loom FSM end-to-end. Call loom_next_step, execute the
	returned step using GitHub MCP tools, then checkpoint.
target: vscode
tools:
	- loom/loom_next_step
	- loom/loom_checkpoint
	- loom/loom_heartbeat
	- loom/loom_get_state
	- loom/loom_abort
	- github/github-mcp-server/default
handoffs:
	- label: Evaluate gate
		agent: loom-gate
		prompt: "Evaluate whether PR ${pr_number} is safe to merge. Return PASS or FAIL."
	- label: Debug CI failure
		agent: loom-debug
		prompt: "CI failed on PR ${pr_number} (run ${run_id}). Post a debug comment."
	- label: Pause for human
		agent: ask
		prompt: "Loom has paused. ${reason}"
---

Follow the Loom MCP Operator contract in .github/skills/loom-mcp-loop.md.
```

### 5.2 Gate Agent

```markdown
---
name: Loom Gate
description: Read-only gate evaluator. Returns {"verdict":"PASS"|"FAIL","reason":"..."}.
	Never makes writes.
target: vscode
tools:
	- search/codebase
	- github/github-mcp-server/pull_request_read
	- github/github-mcp-server/get_commit
	- loom/loom_get_state
---

You are a pure evaluator. Given a PR number, check:
1. All required CI checks are green.
2. At least one approved review exists.
3. The PR is not a draft.
4. No dependency in loom://dependencies is still open.
5. No merge conflicts.

Return exactly: {"verdict":"PASS","reason":"…"} or {"verdict":"FAIL","reason":"…"}.
Make no GitHub writes. Call no tools that could have side effects.
```

### 5.3 Debug Agent

```markdown
---
name: Loom Debug
description: Posts a structured debug comment on a failing PR CI run.
target: vscode
tools:
	- github/github-mcp-server/pull_request_read
	- github/github-mcp-server/get_commit
	- github/github-mcp-server/add_issue_comment
	- search/codebase
---

Given a PR number and a failing check run ID:
1. Read the check run annotations.
2. Identify the root cause.
3. Post a structured comment on the PR following the debug comment template in
	 .github/squad_prompts/WORKFLOW_GITHUB.md.
4. Return {"action":"commented","comment_id":…}.

Do not modify files. Do not create new issues.
```

---

## 6. Workflow Decomposition

The monolithic `WORKFLOW_GITHUB.md` playbook maps onto a set of prompt files
and agent handoffs, one per FSM transition:

| FSM Transition | Mechanism | File |
|---|---|---|
| IDLE → SCANNING | Orchestrator invokes `loom_next_step` | `loom-orchestrator.agent.md` |
| SCANNING → ISSUE_CREATED | Orchestrator calls GitHub MCP `issue_write` | `loom-orchestrator.agent.md` |
| ISSUE_CREATED → AWAITING_PR | Orchestrator calls `assign_copilot_to_issue` | `loom-orchestrator.agent.md` |
| AWAITING_PR → AWAITING_READY | Orchestrator polls `list_pull_requests` via heartbeat Task | `loom-orchestrator.agent.md` |
| AWAITING_READY → AWAITING_CI | Orchestrator calls `update_pull_request` (draft → ready) | `loom-orchestrator.agent.md` |
| AWAITING_CI → REVIEWING | Gate subagent returns PASS | `loom-gate.agent.md` (subagent) |
| AWAITING_CI → DEBUGGING | Gate subagent returns FAIL | `loom-debug.agent.md` (handoff) |
| DEBUGGING → AWAITING_CI | Debug comment posted; retry loop | `loom-debug.agent.md` → orchestrator handoff |
| REVIEWING → MERGING | Orchestrator calls `request_copilot_review`, polls `reviews_approved` | `loom-orchestrator.agent.md` |
| MERGING → SCANNING | Orchestrator calls `merge_pull_request`, advances phase | `loom-orchestrator.agent.md` |
| Any → PAUSED | Budget exhausted; MCP elicitation | `loom-orchestrator.agent.md` |

---

## 7. Expanded Vision: Parallel Epic Execution

The original Loom ran one FSM, one epic phase at a time. With background agents
and Git worktree isolation (v1.107), Loom v2 can run phases in parallel:

```
Orchestrator (local session)
├── Background Agent A (worktree-e2-us1): working on US-2.1
├── Background Agent B (worktree-e2-us2): working on US-2.2
└── Background Agent C (worktree-e2-us3): working on US-2.3
```

Each background agent:
1. Is spawned via `Continue in → Background agent` with the user story as context.
2. Runs in an isolated Git worktree (`worktree-e2-us1`, etc.).
3. Reports back to the orchestrator via Loom MCP checkpoints.
4. Opens a PR against the main branch when complete.

The orchestrator's FSM advances the dependency DAG: US-2.4 is not started until
US-2.1, US-2.2, and US-2.3 are all merged.

This multiplies throughput by the number of independent user stories in a phase,
limited only by API rate limits and available Copilot seats.

---

## 8. What Loom Still Owns

Not everything moves to VS Code's agent framework. Loom's Go binary retains:

| Concern | Why Loom owns it |
|---|---|
| FSM correctness | Deterministic state machine with Go unit tests. Not appropriate in a prompt. |
| SQLite checkpoint persistence | Crash recovery, cross-session continuity. File-based, no VS Code dependency. |
| Dependency DAG evaluation | Pure graph algorithm. Testable without LLM. |
| Retry budget accounting | Stateful counter across sessions. Per-operation deduplication key. |
| Structured logging | `log/slog` JSON. The human operator's window into all activity. |
| MCP server | Stdio server, registered in `.vscode/mcp.json`. Stable contract surface. |
| `loom start/status/pause/resume/log` CLI | Human operator commands. No VS Code required. |

The LLM handles: parsing GitHub API responses, writing issue bodies and
comments, choosing the right tool for each step, recovering gracefully from
ambiguous states. The Go binary handles: remembering what was done, refusing
duplicate actions, and providing the next deterministic step.

---

## 9. Design Values — Amended

The five values from v1 (`01-vision.md`) are preserved. Two are amended.

### 1. Plumbing vs. Intelligence (unchanged)

Loom's Go binary is pure plumbing. The agents are the intelligence. The new
custom agent files are part of "intelligence" — they are declarative definitions
of agent behavior, not embedded in the binary.

### 2. Locally Testable (unchanged)

FSM, dependency DAG, and SQLite remain pure Go packages with no external deps.

### 3. Stable APIs Only (extended)

Custom agent files (`.github/agents/*.agent.md`) and MCP resources are
stable, versioned surfaces. MCP tool names follow the qualified naming scheme
(`loom/loom_next_step`). Handoff targets reference agents by filename, not
by internal VS Code IDs.

### 4. Observable by Default (extended)

`loom://state` and `loom://log` are now MCP resources, surfacing the same
structured JSON log as the CLI, but discoverable from within any agent session
without switching to a terminal.

### 5. Graceful Degradation (extended)

`PAUSED` state now triggers an MCP elicitation with structured choices,
replacing the bare "Loom has paused, run `loom resume`" message. Operators
can act from within the chat UI. The Go binary still owns the PAUSED state;
the elicitation is how it surfaces to the operator.

---

## 10. New Capability: Run-Loom Session Trace

Loom v2 should keep a persistent session trace tab for every `/run-loom`
execution. The goal is not a dashboard for vanity metrics; it is an operator
artifact for post-mortem analysis, reproducibility, and debugging.

The trace tab is the human-readable companion to the structured JSON log. It
should remain openable during the session and reviewable after the session,
with enough context to answer:

- Which Loom build executed this run?
- What FSM states were traversed, and why?
- Which GitHub issues and pull requests were involved?
- How did their states evolve over time?
- Which retries, pauses, elicitations, and operator interventions occurred?

### 10.1 Operator Problem Statement

Today, Loom exposes machine-readable state and logs, but a failed or surprising
`/run-loom` session still requires reconstructing the narrative by correlating
CLI output, MCP tool activity, and GitHub state by hand. That is too expensive
for debugging intermittent failures, validating the FSM, and understanding
cross-boundary behavior between Loom and GitHub.

### 10.2 Primary Users

- Loom operators running autonomous `/run-loom` sessions
- Maintainers debugging FSM or MCP integration regressions
- Release engineers validating behavior across Loom binary versions
- Developers performing post-mortem analysis after a stalled or incorrect run

### 10.3 Required Contents of the Trace Tab

Every session trace must include a stable header with:

- Loom binary version
- Loom release tag
- Session identifier
- Repository owner/name
- Start time, end time, and final session outcome

The body of the tab must provide:

1. A chronological event timeline of every meaningful Loom event.
2. An FSM transition ledger with `from_state`, `to_state`, timestamp, and
	 transition reason.
3. A GitHub entity ledger that tracks issues and pull requests involved in the
	 session and records how their states changed over time.
4. Explicit markers for retries, exhausted budgets, elicitation prompts,
	 operator responses, and pause/resume boundaries.
5. Enough source identifiers to correlate the human-readable trace with the
	 structured log and persisted SQLite checkpoints.

### 10.4 GitHub State Transcription

For each tracked issue and pull request, Loom should maintain a compact state
form inside the trace tab. The purpose is to show evolution, not just the final
snapshot.

Minimum tracked fields:

- GitHub entity type and number
- Title / short label
- First-seen timestamp
- Current state and prior state
- Draft/ready status for pull requests
- Review state summary
- CI/check summary
- Mergeability / merged state
- Closing reason or terminal outcome

Each state change should be appended as a new trace entry so an operator can
replay the session without querying GitHub again.

### 10.5 Success Criteria

This capability is successful when:

- A maintainer can reconstruct a `/run-loom` session from a single tab without
	switching between terminal logs, GitHub pages, and the database.
- Every FSM transition visible in SQLite also appears in the trace tab with a
	human-readable reason.
- The header always identifies the exact Loom build via version and release
	tag.
- Issue and PR evolution is visible as a sequence of state changes, not only as
	final snapshots.
- A failed session can be analyzed after the fact even if GitHub state has
	since changed.

### 10.6 Constraints and Open Questions

Constraints:

- The trace must be append-only or otherwise auditable; post-hoc mutation
	weakens post-mortem value.
- It must not become the source of truth for orchestration state; SQLite and
	the FSM remain authoritative.
- It must scale to long sessions without becoming unreadable or excessively
	expensive to maintain.

Open questions:

- Should the tab be backed by a dedicated MCP resource such as
	`loom://session/<id>` or generated as a Markdown/virtual document?
- Should GitHub state be recorded as full snapshots, field-level diffs, or a
	hybrid model?
- How long should session traces be retained, and should Loom export them as
	release/debug artifacts?

---

## 11. Implementation Roadmap

Priority order follows the gap analysis recommendations, now mapped to the
new primitives.

| Priority | Deliverable | Closes Gap |
|---|---|---|
| P0 | `.loom/dependencies.yaml` schema + Loom MCP resource `loom://dependencies` | Gap 3 |
| P0 | Convert `loom-mcp-operator.md` to `.github/agents/loom-orchestrator.agent.md` with handoffs | Gap 2, 5 |
| P1 | `loom-gate.agent.md` (subagent, read-only tools, structured verdict) | Gap 2 |
| P1 | `loom-debug.agent.md` (debug comment on failing CI) | Gap 5 |
| P1 | MCP Task wrapping for `loom_heartbeat` polling calls | Gap 1 |
| P2 | MCP elicitation schema for budget-exhaustion choices | Gap 5 |
| P2 | `loom://state` and `loom://log` MCP resources | Gap 4 (observability) |
| P2 | Session trace resource/tab with header metadata, FSM transitions, and GitHub issue/PR state evolution | Gap 4 (post-mortem observability) |
| P2 | MCP server instructions with phase summary + dependency digest | Gap 3 |
| P3 | Background agent spawning for parallel user story execution | Throughput |
| P3 | Git worktree isolation per background session | Gap 4 (concurrency) |
| P3 | Org-level Loom MCP server in private registry | Gap 6 |
| P4 | `loom://dependencies` YAML schema enforced by Loom CLI on push | Gap 3 (quality) |

---

## 12. What Has Not Changed

The following remain exactly as specified in `01-vision.md`:

- Go as the implementation language
- SQLite + `modernc.org/sqlite` for state persistence
- `google/go-github` for GitHub REST client
- `log/slog` for structured JSON logging
- Five MCP tool names and their semantics
- FSM state names and transition table
- WORKFLOW_GITHUB.md as the ground-truth workflow specification
- `loom start`, `loom status`, `loom pause`, `loom resume`, `loom log` CLI surface

The changes in v2 are **additive**: new agent definition files, new MCP
resources, new elicitation payloads, and a new dependency YAML schema. No
existing interface is removed or renamed.

---

## 13. Relationship to Epics

The existing epic/user-story hierarchy maps cleanly to the P0–P4 roadmap:

| Epic | P-level | New v2 Work |
|---|---|---|
| E1 — Project Foundation | Done | `.loom/dependencies.yaml` schema (P0) |
| E2 — State Machine | Done | MCP Task wrapping for poll calls (P1) |
| E3 — GitHub Client | Done | No new work |
| E4 — MCP Server | Active | MCP resources (P2), elicitation (P2), session trace surface (P2), server instructions (P2) |
| E5 — CLI | Active | No new work |
| E6 — Session Management | Planned | Background agent spawning (P3), worktree isolation (P3) |
| E7 — Checkpointing | Planned | `loom://state` resource (P2), session correlation identifiers for trace replay (P2) |
| E8 — Integration Hardening | Planned | Org MCP registry (P3), tool eligibility policy (P3) |

New epics warranted by v2:

- **E9 — Agent Definitions**: The `.github/agents/` custom agent files for
	orchestrator, gate, debug. Handoff wiring. Subagent invocation patterns.
- **E10 — Parallel Execution**: Background agent spawning, Git worktree
	lifecycle, orchestrator DAG-aware scheduling.
