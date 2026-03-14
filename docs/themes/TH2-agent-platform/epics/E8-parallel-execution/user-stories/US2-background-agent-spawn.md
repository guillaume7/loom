---
id: TH2.E8.US2
title: "Background agent spawning via code chat CLI"
type: standard
priority: low
size: M
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "Loom can spawn a background agent session via code chat -m loom-orchestrator command"
  - AC2: "Spawned session receives the user story context as the initial prompt"
  - AC3: "Process exit code is captured and reported"
  - AC4: "If code CLI is not available on PATH, a clear error is returned"
depends-on: [TH2.E8.US1]
---

# TH2.E8.US2 — Background Agent Spawning via code chat CLI

**As a** Loom orchestrator, **I want** to spawn background agent sessions for independent stories, **so that** multiple stories can be worked on in parallel.

## Acceptance Criteria

- [ ] AC1: Loom can spawn a background agent session via `code chat -m loom-orchestrator` command
- [ ] AC2: Spawned session receives the user story context as the initial prompt
- [ ] AC3: Process exit code is captured and reported
- [ ] AC4: If `code` CLI is not available on PATH, a clear error is returned

## BDD Scenarios

### Scenario: Spawn background agent
- **Given** a user story "US-2.1" is unblocked
- **When** the orchestrator spawns a background agent for US-2.1
- **Then** a `code chat -m loom-orchestrator --worktree worktree-us-2.1 "Implement US-2.1"` process is started

### Scenario: code CLI not found
- **Given** `code` is not on the system PATH
- **When** the orchestrator attempts to spawn a background agent
- **Then** an error is returned: "code CLI not found on PATH"

### Scenario: Agent process exits with error
- **Given** a spawned background agent for US-2.1
- **When** the agent process exits with code 1
- **Then** the orchestrator is notified of the failure
- **And** the story status can be updated accordingly
