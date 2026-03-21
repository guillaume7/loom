---
id: TH3.E1.US4
title: "Pause and manual override controls"
type: standard
priority: medium
size: M
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "The runtime defines an explicit pause model that stops forward progress without losing recoverable state"
  - AC2: "Manual override actions are constrained, auditable, and distinct from routine runtime decisions"
  - AC3: "Resuming after pause preserves the same session and correlation context when appropriate"
depends-on: [TH3.E1.US3]
---

# TH3.E1.US4 — Pause and Manual Override Controls

**As a** Loom operator, **I want** explicit pause and override controls, **so that** I can intervene safely without corrupting the runtime's state model.

## Acceptance Criteria

- [ ] AC1: The runtime defines an explicit pause model that stops forward progress without losing recoverable state
- [ ] AC2: Manual override actions are constrained, auditable, and distinct from routine runtime decisions
- [ ] AC3: Resuming after pause preserves the same session and correlation context when appropriate

## BDD Scenarios

### Scenario: Pause stops new work but preserves recoverable state
- **Given** the runtime is mid-run
- **When** the operator pauses execution
- **Then** new work is not started
- **And** checkpoint, wake, and trace state remain recoverable

### Scenario: Manual override is recorded as operator intent
- **Given** the operator performs a manual override
- **When** Loom records that action
- **Then** the override is distinguishable from normal policy outcomes
- **And** it is visible in the trace and logs

### Scenario: Resume continues with preserved context
- **Given** a paused run has not been abandoned
- **When** the operator resumes it
- **Then** Loom continues with the same session context where appropriate
- **And** correlation identifiers remain consistent for later analysis
