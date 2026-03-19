---
id: TH2.E9.US1
title: "Session trace header and append-only event model"
status: done
epic: E9
depends-on: []
---

# TH2.E9.US1 — Session trace header and append-only event model

## As a
Loom operator

## I want
every `/run-loom` session to automatically create a durable trace record that
captures stable header metadata

## So that
I can correlate runtime behaviour with a specific Loom release, repository, and
time window even after the process has exited.

## Acceptance Criteria

- [ ] A `session_trace` table is created in the SQLite database alongside the
      existing `checkpoint` and `action_log` tables.
- [ ] Each trace row stores: `session_id`, `loom_ver`, `repository`,
      `started_at`, `ended_at`, and `outcome`.
- [ ] `OpenSessionTrace` is idempotent: calling it twice with the same
      `session_id` does not create duplicate rows.
- [ ] `Serve()` opens the trace before accepting MCP messages and closes it
      with the terminal outcome when the server exits.
- [ ] `CloseSessionTrace` sets `ended_at` and transitions `outcome` from
      `in_progress` to the final value (`complete`, `paused`, `aborted`, or
      `stalled`).

## BDD Scenarios

```gherkin
Given a fresh SQLite database
When the MCP server starts via loom mcp
Then a session_trace row exists with outcome = "in_progress"

Given a running session
When loom mcp exits with FSM in COMPLETE state
Then the session_trace row has outcome = "complete" and a non-null ended_at
```
