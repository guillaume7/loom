---
id: TH2.E8.US1
title: "Checkpoint table story_id column extension"
type: standard
priority: low
size: S
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "checkpoint table gains a story_id TEXT column via additive migration"
  - AC2: "Default value is empty string for backward compatibility with sequential mode"
  - AC3: "Multiple rows with different story_id values can coexist"
  - AC4: "Existing single-row behavior is preserved when story_id is empty"
depends-on: []
---

# TH2.E8.US1 — Checkpoint Table story_id Column Extension

**As a** Loom developer, **I want** the checkpoint table to support per-story rows, **so that** parallel execution can track multiple concurrent stories.

## Acceptance Criteria

- [ ] AC1: `checkpoint` table gains a `story_id TEXT` column via additive migration
- [ ] AC2: Default value is empty string for backward compatibility with sequential mode
- [ ] AC3: Multiple rows with different `story_id` values can coexist
- [ ] AC4: Existing single-row behavior is preserved when `story_id` is empty

## BDD Scenarios

### Scenario: Migration adds column
- **Given** a v1 checkpoint table without `story_id` column
- **When** `migrate()` runs
- **Then** the `story_id` column exists with default value `''`

### Scenario: Sequential mode unaffected
- **Given** a checkpoint with `story_id = ''`
- **When** `ReadCheckpoint()` is called (v1 API)
- **Then** the existing single-row checkpoint is returned

### Scenario: Multiple story checkpoints
- **Given** the extended checkpoint table
- **When** two checkpoints are written with `story_id = 'US-2.1'` and `story_id = 'US-2.2'`
- **Then** both rows exist and can be read independently

### Scenario: Idempotent migration
- **Given** the `story_id` column already exists
- **When** `migrate()` runs again
- **Then** no error occurs
