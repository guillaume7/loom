---
id: TH2.E9.US5
title: "Session index, discoverability, and retention"
type: standard
priority: medium
size: S
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "Loom exposes an index of retained session traces with session_id, repository, started_at, and outcome"
  - AC2: "Retention is bounded by age or count and removes whole expired sessions only"
  - AC3: "Retained traces remain unchanged from their original append-only record"
depends-on: [TH2.E9.US4]
---

# TH2.E9.US5 — Session Index, Discoverability, and Retention

**As a** developer debugging Loom, **I want** a discoverable retained session index with bounded retention, **so that** session traceability remains usable without unbounded growth.

## Acceptance Criteria

- [ ] AC1: Loom exposes an index of retained session traces with `session_id`, repository, `started_at`, and outcome
- [ ] AC2: Retention is bounded by age or count and removes whole expired sessions only
- [ ] AC3: Retained traces remain unchanged from their original append-only record

## BDD Scenarios

### Scenario: Retained sessions can be listed
- **Given** multiple retained session traces exist
- **When** the operator requests the session index
- **Then** Loom returns enough metadata to identify and open a specific session trace

### Scenario: Expired sessions are pruned by whole session
- **Given** retained traces exceed the configured retention threshold
- **When** retention pruning runs
- **Then** only expired or excess whole-session traces are removed
- **And** retained sessions are left intact

### Scenario: Retained trace contents stay immutable
- **Given** a retained session trace remains within the retention window
- **When** the operator opens the trace after pruning has run
- **Then** the trace contents are unchanged from the original append-only record