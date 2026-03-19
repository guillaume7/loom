# E9 — Run-Loom Session Traceability

> Theme: [TH2 — Native Agent Platform](../../README.md)
> ADR: [ADR-007](../../../../ADRs/ADR-007-session-trace-resource-and-storage.md)
> Priority: P2

## Goal

Provide a persistent, append-only session trace for every `/run-loom`
execution that exposes stable run metadata, a chronological event narrative,
FSM transition reasons, GitHub issue and PR state evolution, and operator
intervention markers in one human-readable session surface.

## Dependencies

- **E2** (Action Log & Idempotency) — append-only storage and correlation patterns
- **E3** (MCP Resources) — resource registration and operator read surfaces
- **E5** (MCP Tasks) — long-running task lifecycle events must appear in traces
- **E6** (MCP Elicitation) — operator prompts and responses must be captured

## Stories

| Story | Title | Size | Depends On |
| ------- | ------- | ------ | ------------ |
| US1 | Session trace header and append-only event model | M | - |
| US2 | FSM transition and operator intervention ledger | M | US1 |
| US3 | GitHub issue and PR evolution ledger | L | US1 |
| US4 | Human-readable session trace resource or tab | M | US2, US3 |
| US5 | Session index, discoverability, and retention | S | US4 |

## Acceptance

Epic is done when:

- Every `/run-loom` session creates a durable trace with Loom version, release tag, session ID, repository, start time, end time, and terminal outcome
- Every meaningful Loom transition appends a timestamped trace entry with `from_state`, `to_state`, reason, and correlation identifiers
- GitHub issues and pull requests touched by the session appear in a ledger that records state evolution over time
- Retries, budget exhaustion, elicitation prompts, operator responses, and pause/resume boundaries are explicitly represented
- Operators can open the trace during execution and review it after completion from a dedicated session surface
- Retained traces remain append-only or otherwise auditable and never become the orchestration source of truth
