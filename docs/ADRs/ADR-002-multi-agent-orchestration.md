# ADR-002: Multi-Agent Orchestration via Custom Agent Definitions

## Status
Proposed

## Context

VP2 (§3, §5, §6) identifies that VS Code v1.101–v1.107 introduced custom agents,
handoffs, and isolated subagents. Loom v1 uses a single monolithic Copilot session
with all workflow logic embedded in one long prompt. This creates several problems:

1. **No separation of concerns** — the gate evaluator, debugger, and merge logic
   share one context window and one tool set.
2. **No tool constraint enforcement** — the session can call any available tool,
   including destructive ones, during read-only operations.
3. **Context window pressure** — long-running sessions accumulate context from
   all phases, reducing reasoning quality.
4. **No structured failure branching** — failure handling is embedded in prompt
   logic, not in a workflow graph.

VP2 §5 specifies four agent roles: orchestrator, gate, debug, and merge.
VP2 §6 maps every FSM transition to a specific agent and mechanism (handoff or
subagent invocation).

## Decision

Decompose the monolithic master session into **four custom agent definitions** in
`.github/agents/`, each with explicitly constrained tool sets and declared
handoff transitions.

### Agent Roster

| Agent File | Role | Tool Constraints | Invocation |
|---|---|---|---|
| `loom-orchestrator.agent.md` | Drives FSM loop end-to-end | All Loom + GitHub MCP tools | Entry point; hands off to others |
| `loom-gate.agent.md` | Evaluates merge readiness | Read-only tools only | Subagent (returns structured verdict) |
| `loom-debug.agent.md` | Posts debug analysis on CI failure | Read + comment tools | Handoff from orchestrator |
| `loom-merge.agent.md` | Executes merge after gate PASS | Merge tool only | Handoff from orchestrator |

### Handoff Graph

```
orchestrator ──[gate FAIL]──→ debug ──[commented]──→ orchestrator
orchestrator ──[gate PASS]──→ merge ──[merged]──→ orchestrator
orchestrator ──[budget exhausted]──→ ask (built-in) ──[human response]──→ orchestrator
```

### Subagent vs. Handoff

- **Gate** is invoked as a **subagent** (isolated context window, structured return value).
  The orchestrator does not inherit the gate's reasoning context.
- **Debug** and **merge** are invoked as **handoffs** (pre-filled prompt with PR context).
  They return control to the orchestrator when complete.

### VP2 Traceability

| VP2 Section | Requirement | How Addressed |
|---|---|---|
| §2 Gap 2 | Deterministic gate evaluator | `loom-gate.agent.md` with read-only tools |
| §2 Gap 5 | Failure-handling policy | Handoff branches (debug, ask) |
| §5.1 | Orchestrator agent definition | `loom-orchestrator.agent.md` |
| §5.2 | Gate agent definition | `loom-gate.agent.md` |
| §5.3 | Debug agent definition | `loom-debug.agent.md` |
| §6 | Workflow decomposition by FSM transition | Each transition mapped to an agent |

## Consequences

### Positive
- Each agent has the minimum tool set needed — reduces risk of accidental writes.
- Gate evaluation runs in isolation — reasoning does not pollute orchestrator context.
- Failure policy is a visible, auditable handoff graph — not hidden in prompts.
- Agent files are version-controlled and portable to GitHub cloud agents.

### Negative
- Requires VS Code v1.106+ with custom agent support. Older versions fall back to
  single-session mode (v1 behavior).
- Four files to maintain instead of one prompt. Mitigation: agent files are short
  (< 50 lines each); changes are infrequent.

### Risks
- VS Code custom agent API is still evolving (v1.106–v1.107). Breaking changes
  could require agent file updates. Mitigation: agent files are declarative
  Markdown with minimal schema surface.

## Alternatives Considered

### A. Keep single master session (v1 status quo)
- Pros: Simpler, no agent API dependency.
- Cons: No tool isolation, context window pressure, ad-hoc failure handling.
- Rejected because: VP2 requirements explicitly call for separated agents with
  constrained tools.

### B. Separate Go binaries per role
- Pros: Full isolation, no VS Code dependency.
- Cons: Massive over-engineering; duplicates MCP server logic; loses LLM reasoning.
- Rejected because: VS Code custom agents provide the same isolation declaratively.

### C. Single agent with dynamic tool filtering
- Pros: One file to maintain.
- Cons: No platform enforcement of tool constraints; still one context window.
- Rejected because: Subagent isolation (separate context windows) is a key VP2 requirement.
