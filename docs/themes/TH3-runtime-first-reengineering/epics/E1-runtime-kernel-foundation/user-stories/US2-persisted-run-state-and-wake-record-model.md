---
id: TH3.E1.US2
title: "Persisted run state and wake record model"
type: standard
priority: high
size: M
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "The runtime data model defines durable records for wake schedules, external events, leases, and policy outcomes"
  - AC2: "Checkpoint state remains the source of truth and is not replaced by trace or replay artifacts"
  - AC3: "The schema evolution path is additive and compatible with existing SQLite state"
depends-on: [TH3.E1.US1]
---

# TH3.E1.US2 — Persisted Run State and Wake Record Model

**As a** Loom maintainer, **I want** durable runtime records around the checkpoint, **so that** the runtime can resume safely without relying on interactive session memory.

## Acceptance Criteria

- [ ] AC1: The runtime data model defines durable records for wake schedules, external events, leases, and policy outcomes
- [ ] AC2: Checkpoint state remains the source of truth and is not replaced by trace or replay artifacts
- [ ] AC3: The schema evolution path is additive and compatible with existing SQLite state

## BDD Scenarios

### Scenario: Add wake and lease records without replacing checkpoint truth
- **Given** the checkpoint already stores the authoritative workflow snapshot
- **When** runtime support tables are added
- **Then** those tables extend orchestration metadata only
- **And** checkpoint state remains authoritative

### Scenario: Persist external observations for later replay
- **Given** Loom observes GitHub or timer-driven inputs
- **When** those inputs are recorded
- **Then** they are stored as additive runtime records
- **And** they can be correlated with the session trace

### Scenario: Migrate existing state forward compatibly
- **Given** an existing `.loom/state.db` created before TH3
- **When** the new runtime schema is introduced
- **Then** the migration is additive
- **And** prior checkpoint and trace data remain readable
