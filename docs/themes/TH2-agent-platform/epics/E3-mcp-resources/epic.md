# E3 — MCP Resources

> Theme: [TH2 — Native Agent Platform](../../README.md)
> ADR: [ADR-003](../../../../ADRs/ADR-003-mcp-resources.md)
> Priority: P2

## Goal

Register three MCP resources (`loom://dependencies`, `loom://state`,
`loom://log`) and add server instructions to the MCP server. Resources give
agents read-only access to Loom internals without tool calls.

## Dependencies

- **E1** (Dependency Graph) — `loom://dependencies` serves the parsed graph
- **E2** (Action Log) — `loom://log` reads from the action_log table

## Stories

| Story | Title | Size | Depends On |
|-------|-------|------|------------|
| US1 | Resource registration framework | S | — |
| US2 | loom://dependencies resource | M | US1 |
| US3 | loom://state resource | M | US1 |
| US4 | loom://log resource | M | US1 |
| US5 | MCP server instructions | S | US1 |

## Acceptance

Epic is done when:
- MCP server responds to `resources/list` with three registered resources
- `loom://dependencies` returns YAML from `.loom/dependencies.yaml`
- `loom://state` returns JSON with FSM state, PR, retry counts
- `loom://log` returns NDJSON of recent actions
- Server instructions include phase summary and dependency digest
