---
id: TH2.E6.US2
title: "skip_story FSM event and transition"
type: standard
priority: medium
size: M
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "New skip_story event is added to the FSM event set"
  - AC2: "skip_story from AWAITING_CI or DEBUGGING transitions to SCANNING"
  - AC3: "Phase counter advances to next story on skip_story"
  - AC4: "Skipped story is recorded in checkpoint detail"
depends-on: []
---

# TH2.E6.US2 — skip_story FSM Event and Transition

**As a** Loom FSM developer, **I want** a `skip_story` event that advances past a failing story, **so that** the operator can unblock the workflow via elicitation.

## Acceptance Criteria

- [ ] AC1: New `skip_story` event is added to the FSM event set
- [ ] AC2: `skip_story` from `AWAITING_CI` or `DEBUGGING` transitions to `SCANNING`
- [ ] AC3: Phase counter advances to next story on `skip_story`
- [ ] AC4: Skipped story is recorded in checkpoint detail

## BDD Scenarios

### Scenario: Skip from AWAITING_CI
- **Given** FSM is in state "AWAITING_CI"
- **When** event `skip_story` is fired
- **Then** FSM transitions to "SCANNING"
- **And** the phase counter advances

### Scenario: Skip from DEBUGGING
- **Given** FSM is in state "DEBUGGING"
- **When** event `skip_story` is fired
- **Then** FSM transitions to "SCANNING"

### Scenario: Skip is invalid from other states
- **Given** FSM is in state "MERGING"
- **When** event `skip_story` is fired
- **Then** the transition is rejected with an error

### Scenario: Skip resets retry counter
- **Given** FSM is in state "AWAITING_CI" with retry_count 5
- **When** event `skip_story` is fired
- **Then** retry_count is reset to 0 in the new SCANNING state

### Scenario: Event constant exists
- **Given** the `internal/fsm` package
- **When** the event constants are inspected
- **Then** `SkipStory` is a valid `Event` constant with a non-empty `String()` value
