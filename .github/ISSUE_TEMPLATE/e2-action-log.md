---
name: "E2 — Action Log & Idempotency"
about: Add durable action history and idempotency enforcement for Loom tool handlers and operator log output.
title: "E2: Action Log & Idempotency"
labels: ["epic", "E2", "TH2"]
---

## Goal

Add an `action_log` table to SQLite for idempotency enforcement and structured audit trail.

## User Stories

- [ ] TH2.E2.US1 — Action log table migration
- [ ] TH2.E2.US2 — WriteAction and ReadActions store methods
- [ ] TH2.E2.US3 — Idempotency check in MCP tool handlers
- [ ] TH2.E2.US4 — loom log CLI shows action history

## Acceptance Criteria

- [ ] `action_log` table is created via additive migration
- [ ] `WriteAction` inserts rows with unique `operation_key`
- [ ] Duplicate `operation_key` inserts are rejected gracefully
- [ ] `ReadActions(limit)` returns the most recent actions
- [ ] `loom log` displays action history from the database
- [ ] MCP tools check for existing operation keys before executing writes