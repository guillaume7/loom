---
id: TH3.E3.US3
title: "Escalation and wait outcome taxonomy"
type: standard
priority: medium
size: M
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "The runtime defines named wait, retry, block, and escalate outcomes with clear semantics"
  - AC2: "Each outcome type maps to concrete runtime actions such as schedule wake-up, stop progress, or request operator intervention"
  - AC3: "Escalation conditions are explicit and are not hidden in free-form agent prompts"
depends-on: [TH3.E3.US1]
---

# TH3.E3.US3 — Escalation and Wait Outcome Taxonomy

**As a** Loom maintainer, **I want** a fixed taxonomy of runtime outcomes, **so that** waits and escalations are implemented consistently across gates.

## Acceptance Criteria

- [ ] AC1: The runtime defines named wait, retry, block, and escalate outcomes with clear semantics
- [ ] AC2: Each outcome type maps to concrete runtime actions such as schedule wake-up, stop progress, or request operator intervention
- [ ] AC3: Escalation conditions are explicit and are not hidden in free-form agent prompts

## BDD Scenarios

### Scenario: Wait outcome maps to a wake-up action
- **Given** a policy decision returns wait
- **When** the runtime handles that outcome
- **Then** it schedules the appropriate wake-up action

### Scenario: Escalate outcome requests operator attention explicitly
- **Given** a condition requires human judgement
- **When** the runtime returns escalate
- **Then** the operator-facing reason is explicit
- **And** the system does not continue automatically

### Scenario: Block outcome halts unsafe forward progress
- **Given** a blocking condition is detected
- **When** the runtime handles the decision
- **Then** forward progress stops until the block is resolved
