---
id: TH2.E2.US1
title: "Action log table migration"
type: standard
priority: high
size: S
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "action_log table is created via CREATE TABLE IF NOT EXISTS in the migrate function"
  - AC2: "Table schema matches data-model.md: id, session_id, operation_key (unique), state_before, state_after, event, detail, created_at"
  - AC3: "Migration is additive — existing checkpoint table is unaffected"
  - AC4: "Running migrate twice is idempotent"
depends-on: []
---

# TH2.E2.US1 — Action Log Table Migration

**As a** Loom developer, **I want** the SQLite database to include an `action_log` table, **so that** every action taken by Loom can be recorded for idempotency and audit.

## Acceptance Criteria

- [ ] AC1: `action_log` table is created via `CREATE TABLE IF NOT EXISTS` in the migrate function
- [ ] AC2: Table schema matches data-model.md: id, session_id, operation_key (unique), state_before, state_after, event, detail, created_at
- [ ] AC3: Migration is additive — existing checkpoint table is unaffected
- [ ] AC4: Running migrate twice is idempotent

## BDD Scenarios

### Scenario: Fresh database gets action_log table
- **Given** a new SQLite database with no tables
- **When** `migrate()` is called
- **Then** both `checkpoint` and `action_log` tables exist
- **And** `action_log` has a unique index on `operation_key`

### Scenario: Existing database gains action_log table
- **Given** a SQLite database with only the `checkpoint` table (v1 schema)
- **When** `migrate()` is called
- **Then** the `action_log` table is created
- **And** the `checkpoint` table data is preserved

### Scenario: Idempotent migration
- **Given** a database where `migrate()` has already been called
- **When** `migrate()` is called again
- **Then** no error occurs
- **And** table structure is unchanged
