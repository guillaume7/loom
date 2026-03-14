---
id: TH2.E5.US2
title: "Heartbeat polling wrapped as MCP Task"
type: standard
priority: high
size: M
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "loom_heartbeat in polling mode emits task/start with title including PR number"
  - AC2: "Progress events fire every polling interval with CI check status summary"
  - AC3: "task/done event carries the final polling result (all green / failure details)"
  - AC4: "Task ID is deterministic based on PR number for reconnect matching"
depends-on: [TH2.E5.US1]
---

# TH2.E5.US2 — Heartbeat Polling Wrapped as MCP Task

**As a** Loom orchestrator agent, **I want** `loom_heartbeat` CI polling to be a Task with progress events, **so that** I see real-time CI status and can reconnect without losing polling state.

## Acceptance Criteria

- [ ] AC1: `loom_heartbeat` in polling mode emits `task/start` with title including PR number
- [ ] AC2: Progress events fire every polling interval with CI check status summary
- [ ] AC3: `task/done` event carries the final polling result (all green / failure details)
- [ ] AC4: Task ID is deterministic based on PR number for reconnect matching

## BDD Scenarios

### Scenario: CI polling emits progress
- **Given** FSM is in state "AWAITING_CI" for PR #42 with 5 CI checks
- **When** `loom_heartbeat` starts polling
- **Then** a `task/start` event is emitted: `{"id":"loom-ci-poll-pr-42","title":"Watching CI for PR #42"}`
- **And** progress events fire every 30s: `{"progress":"3/5 checks green, waiting on 'build'"}`

### Scenario: CI passes — task done
- **Given** an active CI poll task for PR #42
- **When** all CI checks turn green
- **Then** a `task/done` event is emitted: `{"result":{"all_green":true}}`

### Scenario: CI fails — task done with failure details
- **Given** an active CI poll task for PR #42
- **When** a CI check fails with conclusion "failure"
- **Then** a `task/done` event is emitted: `{"result":{"all_green":false,"failed_checks":["build"]}}`

### Scenario: Reconnect to existing task
- **Given** a CI poll task "loom-ci-poll-pr-42" is active in the server
- **When** a client reconnects and queries task status
- **Then** the client receives the latest progress event
