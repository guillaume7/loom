---
id: TH3.E2.US1
title: "Wake-up queue and timers"
type: standard
priority: high
size: M
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "The runtime persists due wake-ups with deduplication keys and due times"
  - AC2: "Wake-up records support timer-driven reassessment of waiting states without active session heartbeats"
  - AC3: "Operators can inspect pending wake-ups through runtime-facing diagnostics"
depends-on: [TH3.E1.US2]
---

# TH3.E2.US1 — Wake-Up Queue and Timers

**As a** Loom runtime, **I want** a durable wake-up queue, **so that** waiting states can be revisited at the right time without relying on chat-session liveness.

## Acceptance Criteria

- [ ] AC1: The runtime persists due wake-ups with deduplication keys and due times
- [ ] AC2: Wake-up records support timer-driven reassessment of waiting states without active session heartbeats
- [ ] AC3: Operators can inspect pending wake-ups through runtime-facing diagnostics

## BDD Scenarios

### Scenario: Waiting state schedules a wake-up
- **Given** Loom enters a waiting state such as CI or review
- **When** it needs to check again later
- **Then** a wake-up record is persisted with a due time and deduplication key

### Scenario: Due wake-up triggers reassessment
- **Given** a wake-up record is due
- **When** the runtime scheduler claims it
- **Then** Loom reassesses the waiting state without needing a prior session heartbeat

### Scenario: Operator inspects pending wake-ups
- **Given** wake-up records exist
- **When** the operator inspects runtime diagnostics
- **Then** the pending wake-ups and their due times are visible
