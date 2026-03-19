---
id: TH2.E9.US1
title: "Session trace header and append-only event model"
type: standard
priority: medium
size: M
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "Starting /run-loom creates a durable session trace with loom_version, release_tag, session_id, repo_owner, repo_name, and started_at"
  - AC2: "Completing or pausing a session records ended_at and terminal outcome without rewriting prior entries"
  - AC3: "Trace events are append-only and carry stable sequence numbers and correlation identifiers"
depends-on: []
---

# TH2.E9.US1 — Session Trace Header and Append-Only Event Model

**As a** Loom operator, **I want** each `/run-loom` execution to create a durable append-only trace with stable run metadata, **so that** every session has a canonical audit artifact.

## Acceptance Criteria

- [ ] AC1: Starting `/run-loom` creates a durable session trace with `loom_version`, `release_tag`, `session_id`, `repo_owner`, `repo_name`, and `started_at`
- [ ] AC2: Completing or pausing a session records `ended_at` and terminal outcome without rewriting prior entries
- [ ] AC3: Trace events are append-only and carry stable sequence numbers and correlation identifiers

## BDD Scenarios

### Scenario: New session initializes trace header
- **Given** Loom starts a new `/run-loom` execution
- **When** the session is initialized
- **Then** a durable trace header is created with version, release tag, session ID, repository, and start time

### Scenario: Terminal metadata is recorded at session end
- **Given** an active session trace exists
- **When** the session completes or pauses
- **Then** the trace header records the end time and terminal outcome
- **And** prior event entries remain unchanged

### Scenario: Resumed work appends to the same session trace
- **Given** a paused session trace already exists
- **When** Loom resumes work for that session
- **Then** new events append to the same session trace
- **And** sequence numbers continue monotonically