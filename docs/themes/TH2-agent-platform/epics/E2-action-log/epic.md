# E2 — Action Log & Idempotency

> Theme: [TH2 — Native Agent Platform](../../README.md)
> ADR: [ADR-003](../../../../ADRs/ADR-003-mcp-resources.md)
> Priority: P0

## Goal

Add an `action_log` table to SQLite for idempotency enforcement and structured
audit trail. Extend the store layer with `WriteAction` / `ReadActions` methods
and integrate with MCP tool handlers and CLI.

## Stories

| Story | Title | Size | Depends On |
|-------|-------|------|------------|
| US1 | Action log table migration | S | — |
| US2 | WriteAction and ReadActions store methods | M | US1 |
| US3 | Idempotency check in MCP tool handlers | M | US2 |
| US4 | loom log CLI shows action history | S | US2 |

## Acceptance

Epic is done when:
- `action_log` table is created via additive migration
- `WriteAction` inserts rows with unique `operation_key`
- Duplicate `operation_key` inserts are rejected gracefully
- `ReadActions(limit)` returns the most recent actions
- `loom log` displays action history from the database
- MCP tools check for existing operation keys before executing writes
