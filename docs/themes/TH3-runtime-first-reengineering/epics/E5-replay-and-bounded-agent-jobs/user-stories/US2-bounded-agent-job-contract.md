---
id: TH3.E5.US2
title: "Bounded agent job contract"
type: standard
priority: high
size: M
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "Agent work is described as a bounded job with clear input, deadline, and expected output contract"
  - AC2: "The runtime remains the owner of orchestration state before, during, and after an agent job"
  - AC3: "Agent jobs can be retried or replaced without redefining the authoritative workflow state"
depends-on: [TH3.E1.US3, TH3.E4.US1]
---

# TH3.E5.US2 — Bounded Agent Job Contract

**As a** Loom architect, **I want** bounded agent job contracts, **so that** agents contribute work without becoming the control plane.

## Acceptance Criteria

- [ ] AC1: Agent work is described as a bounded job with clear input, deadline, and expected output contract
- [ ] AC2: The runtime remains the owner of orchestration state before, during, and after an agent job
- [ ] AC3: Agent jobs can be retried or replaced without redefining the authoritative workflow state

## BDD Scenarios

### Scenario: Runtime dispatches a bounded job with explicit contract
- **Given** Loom needs an agent to perform a bounded task
- **When** it dispatches the job
- **Then** the job includes explicit input, deadline, and expected output

### Scenario: Runtime state remains authoritative during job execution
- **Given** an agent job is in progress
- **When** the runtime inspects the run state
- **Then** the authoritative workflow state still belongs to the runtime

### Scenario: Job can be retried without rewriting history
- **Given** an agent job fails or times out
- **When** Loom decides to retry or replace it
- **Then** the new attempt is recorded as another bounded job
- **And** prior authoritative state remains intact
