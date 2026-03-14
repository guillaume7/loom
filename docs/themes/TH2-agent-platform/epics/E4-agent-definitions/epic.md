# E4 — Agent Definitions

> Theme: [TH2 — Native Agent Platform](../../README.md)
> ADR: [ADR-002](../../../../ADRs/ADR-002-multi-agent-orchestration.md)
> Priority: P0/P1

## Goal

Create four custom agent definition files in `.github/agents/` that implement
the multi-agent orchestration model: orchestrator (full tools + handoffs),
gate (read-only subagent), debug (read + comment), and merge (merge-only).

## Stories

| Story | Title | Size | Depends On |
|-------|-------|------|------------|
| US1 | Orchestrator agent definition | M | — |
| US2 | Gate agent definition | S | — |
| US3 | Debug agent definition | S | — |
| US4 | Merge agent definition | S | — |

## Acceptance

Epic is done when:
- All four `.agent.md` files exist in `.github/agents/`
- Orchestrator has handoff wiring to gate, debug, and merge
- Gate is read-only with structured PASS/FAIL return
- Debug can only read and comment
- Merge can only execute merge operations
- All files have `target: vscode` and are valid custom agent format
