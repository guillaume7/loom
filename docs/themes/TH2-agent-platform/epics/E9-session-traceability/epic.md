# E9 — Run-Loom Session Traceability

## Summary

Provide a persistent, append-only session trace for every `/run-loom` execution
that exposes stable run metadata, a chronological event narrative, FSM
transition reasons, GitHub issue and PR state evolution, and operator
intervention markers in one human-readable session surface.

## User Stories

- [x] TH2.E9.US1 — Session trace header and append-only event model
- [x] TH2.E9.US2 — FSM transition and operator intervention ledger
- [x] TH2.E9.US3 — GitHub issue and PR evolution ledger
- [x] TH2.E9.US4 — Human-readable session trace resource or tab
- [x] TH2.E9.US5 — Session index, discoverability, and retention

## Acceptance Criteria

- [x] Every `/run-loom` session creates a durable trace with Loom version,
      session ID, repository, start time, end time, and terminal outcome.
- [x] Every meaningful Loom transition appends a timestamped trace entry with
      `from_state`, `to_state`, event, reason, and correlation identifiers.
- [x] GitHub issues and pull requests touched by the session appear in a ledger
      that records state evolution over time.
- [x] Retries, budget exhaustion, elicitation prompts, operator responses, and
      pause/resume boundaries are explicitly represented.
- [x] Operators can open the trace during execution and review it after
      completion from a dedicated session surface (`loom://trace`).
- [x] Retained traces remain append-only and never become the orchestration
      source of truth.

## Implementation Notes

- `session_trace` and `trace_event` tables added to the SQLite store.
- `store.Store` interface extended with `OpenSessionTrace`, `AppendTraceEvent`,
  `CloseSessionTrace`, `ReadSessionTrace`, and `ListSessionTraces`.
- `internal/mcp/trace.go` provides server-side `openTrace` / `appendTrace` /
  `closeTrace` helpers that swallow errors to never interrupt the workflow.
- `loom://trace` MCP resource: Markdown human-readable session trace.
- `loom://trace/index` MCP resource: JSON index of the last 50 sessions.
- `cmd/loom mcp` passes a fresh UUID trace session ID, the Loom version, and
  the `owner/repo` string to `mcp.NewServer` via `WithTraceSessionID`,
  `WithLoomVersion`, and `WithRepository` options.
