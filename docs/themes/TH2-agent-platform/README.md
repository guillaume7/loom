# TH2 — Native Agent Platform

> Vision Reference: [VP2 — The Native Agent Platform](../../vision_of_product/VP2-agent-platform/02-vision-agent-platform.md)

## Summary

Transform Loom from a single-session MCP tool into a multi-agent orchestration
platform, leveraging VS Code's custom agents, handoffs, subagents, MCP resources,
MCP Tasks, and MCP elicitations.

## Epics

| Epic | Name | Priority | Description |
|------|------|----------|-------------|
| E1 | Dependency Graph Engine | P0 | `internal/depgraph` package; `.loom/dependencies.yaml` schema |
| E2 | Action Log & Idempotency | P0 | `action_log` SQLite table; idempotency enforcement |
| E3 | MCP Resources | P2 | `loom://dependencies`, `loom://state`, `loom://log`; server instructions |
| E4 | Agent Definitions | P0/P1 | `.github/agents/` orchestrator, gate, debug, merge |
| E5 | MCP Tasks | P1 | Task lifecycle wrapping for heartbeat polling |
| E6 | MCP Elicitation | P2 | Structured elicitation on budget exhaustion |
| E7 | Security Hardening | P2 | Token validation, tool annotations, config permissions |
| E8 | Parallel Execution | P3 | Background agents, worktrees, DAG-aware scheduling |

## Dependency Graph

```
E1 (depgraph) ─────────┬──→ E3 (resources)
E2 (action log) ────────┘
E4 (agents)             (independent)
E5 (MCP tasks) ──→ E6 (elicitation)
E7 (security)           (independent)
E1 + E4 + E6 ──→ E8 (parallel execution)
```

## Implementation Waves

- **Wave 1** (parallel): E1, E2, E4, E5, E7
- **Wave 2** (after Wave 1 deps): E3 (needs E1+E2), E6 (needs E5)
- **Wave 3** (after E1+E4+E6): E8
