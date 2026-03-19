---
name: "E5 — MCP Tasks"
about: Wrap long-running Loom polling as MCP Tasks with explicit lifecycle events and fallback behavior.
title: "E5: MCP Tasks"
labels: ["epic", "E5", "TH2"]
---

## Goal

Wrap long-running `loom_heartbeat` polling as MCP Tasks with explicit lifecycle events (start, progress, done), enabling client disconnect and reconnect without losing polling state.

## User Stories

- [ ] TH2.E5.US1 — MCP Task lifecycle event emission
- [ ] TH2.E5.US2 — Heartbeat polling wrapped as MCP Task
- [ ] TH2.E5.US3 — Client capability negotiation and fallback

## Acceptance Criteria

- [ ] `loom_heartbeat` emits `task/start`, `task/progress`, and `task/done` events
- [ ] Progress events fire every polling interval with status details
- [ ] Client can disconnect and reconnect without losing poll state
- [ ] Clients without Task support receive blocking call behavior (v1 compat)