---
id: TH2.E9.US4
title: "Human-readable session trace resource or tab"
type: standard
priority: medium
size: M
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "Loom exposes a dedicated session trace surface for a given session_id"
  - AC2: "The surface renders a stable header, chronological timeline, FSM ledger, and GitHub evolution ledger in one place"
  - AC3: "The surface remains available after session completion subject to retention policy"
depends-on: [TH2.E9.US2, TH2.E9.US3]
---

# TH2.E9.US4 — Human-Readable Session Trace Resource or Tab

**As a** Loom operator, **I want** one human-readable session trace surface, **so that** I can inspect a run live and after the fact without switching between terminal logs, GitHub pages, and SQLite.

## Acceptance Criteria

- [ ] AC1: Loom exposes a dedicated session trace surface for a given `session_id`
- [ ] AC2: The surface renders a stable header, chronological timeline, FSM ledger, and GitHub evolution ledger in one place
- [ ] AC3: The surface remains available after session completion subject to retention policy

## BDD Scenarios

### Scenario: Active session trace can be opened
- **Given** a `/run-loom` session is in progress
- **When** the operator opens the session trace for that `session_id`
- **Then** the header, timeline, and ledgers are readable from one surface

### Scenario: Completed session trace can be reopened
- **Given** a session has already completed
- **When** the operator reopens the session trace
- **Then** the final outcome and full narrative remain available

### Scenario: Unknown session returns clear not-found result
- **Given** no retained session exists for a requested `session_id`
- **When** the session trace surface is requested
- **Then** Loom returns a clear not-found response instead of an empty or misleading document