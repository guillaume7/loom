---
name: "E9 — Run-Loom Session Traceability"
about: Provide a persistent session trace surface that captures run metadata, FSM transitions, GitHub evolution, and operator interventions.
title: "E9: Run-Loom Session Traceability"
labels: ["epic", "E9", "TH2"]
---

## Goal

Provide a persistent, append-only session trace for every `/run-loom` execution that exposes stable run metadata, a chronological event narrative, FSM transition reasons, GitHub issue and PR state evolution, and operator intervention markers in one human-readable session surface.

## User Stories

- [ ] TH2.E9.US1 — Session trace header and append-only event model
- [ ] TH2.E9.US2 — FSM transition and operator intervention ledger
- [ ] TH2.E9.US3 — GitHub issue and PR evolution ledger
- [ ] TH2.E9.US4 — Human-readable session trace resource or tab
- [ ] TH2.E9.US5 — Session index, discoverability, and retention

## Acceptance Criteria

- [ ] Every `/run-loom` session creates a durable trace with Loom version, release tag, session ID, repository, start time, end time, and terminal outcome
- [ ] Every meaningful Loom transition appends a timestamped trace entry with `from_state`, `to_state`, reason, and correlation identifiers
- [ ] GitHub issues and pull requests touched by the session appear in a ledger that records state evolution over time
- [ ] Retries, budget exhaustion, elicitation prompts, operator responses, and pause/resume boundaries are explicitly represented
- [ ] Operators can open the trace during execution and review it after completion from a dedicated session surface
- [ ] Retained traces remain append-only or otherwise auditable and never become the orchestration source of truth
