# E5 — MCP Tasks

> Theme: [TH2 — Native Agent Platform](../../README.md)
> ADR: [ADR-004](../../../../ADRs/ADR-004-mcp-tasks-and-elicitation.md)
> Priority: P1

## Goal

Wrap long-running `loom_heartbeat` polling as MCP Tasks with explicit lifecycle
events (start/progress/done), enabling client disconnect/reconnect without
losing polling state.

## Stories

| Story | Title | Size | Depends On |
|-------|-------|------|------------|
| US1 | Task lifecycle event emission | M | — |
| US2 | Heartbeat polling as MCP Task | M | US1 |
| US3 | Client capability negotiation and fallback | M | US1 |

## Acceptance

Epic is done when:
- `loom_heartbeat` emits `task/start`, `task/progress`, `task/done` events
- Progress events fire every polling interval with status details
- Client can disconnect and reconnect without losing poll state
- Clients without Task support receive blocking call behavior (v1 compat)
