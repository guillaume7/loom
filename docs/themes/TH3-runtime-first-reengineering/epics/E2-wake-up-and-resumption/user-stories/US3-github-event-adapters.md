---
id: TH3.E2.US3
title: "GitHub event adapters"
type: standard
priority: medium
size: M
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "The runtime defines an adapter model for GitHub events that can enqueue resumptions without bypassing checkpoint truth"
  - AC2: "Event payloads are stored as external observations rather than direct state mutations"
  - AC3: "Polling remains available as fallback when event delivery is unavailable or incomplete"
depends-on: [TH3.E2.US1]
---

# TH3.E2.US3 — GitHub Event Adapters

**As a** Loom architect, **I want** optional GitHub event adapters, **so that** Loom can wake faster when signals are available without making event delivery a hard dependency.

## Acceptance Criteria

- [ ] AC1: The runtime defines an adapter model for GitHub events that can enqueue resumptions without bypassing checkpoint truth
- [ ] AC2: Event payloads are stored as external observations rather than direct state mutations
- [ ] AC3: Polling remains available as fallback when event delivery is unavailable or incomplete

## BDD Scenarios

### Scenario: Event enqueues a wake-up instead of mutating state directly
- **Given** a relevant GitHub event is observed
- **When** Loom processes it
- **Then** the event is recorded as an observation
- **And** any resulting wake-up is scheduled through the runtime queue

### Scenario: Event payload is available for later replay
- **Given** Loom receives a GitHub event
- **When** it stores the observation
- **Then** the payload can be replayed later to explain a runtime decision

### Scenario: Missing events fall back to polling
- **Given** event delivery is delayed or absent
- **When** the runtime still needs to progress
- **Then** polling remains the authoritative fallback path
