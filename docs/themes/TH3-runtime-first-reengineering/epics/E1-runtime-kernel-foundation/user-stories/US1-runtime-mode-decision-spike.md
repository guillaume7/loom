---
id: TH3.E1.US1
title: "Runtime mode decision spike"
type: spike
priority: high
size: S
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "The spike compares resumable runner, local daemon, and hybrid runtime modes against VP3 constraints"
  - AC2: "A recommended baseline runtime mode is recorded with rationale and rejected alternatives"
  - AC3: "The impact on existing CLI, MCP, and operator workflows is explicitly identified"
depends-on: []
---

# TH3.E1.US1 — Runtime Mode Decision Spike

**As a** Loom architect, **I want** a clear runtime mode decision, **so that** TH3 implementation does not drift between incompatible control models.

## Acceptance Criteria

- [ ] AC1: The spike compares resumable runner, local daemon, and hybrid runtime modes against VP3 constraints
- [ ] AC2: A recommended baseline runtime mode is recorded with rationale and rejected alternatives
- [ ] AC3: The impact on existing CLI, MCP, and operator workflows is explicitly identified

## BDD Scenarios

### Scenario: Evaluate runtime modes against local-first constraints
- **Given** VP3 requires local development and debugging without a hosted control plane
- **When** the runtime modes are compared
- **Then** the comparison records how each mode satisfies or violates that constraint

### Scenario: Select a baseline mode for TH3 delivery
- **Given** the comparison is complete
- **When** the spike concludes
- **Then** one baseline mode is recommended for TH3 planning
- **And** the rejected modes are recorded with reasons

### Scenario: Identify migration impact on current operator flows
- **Given** Loom already has CLI and MCP entry points
- **When** the runtime mode is selected
- **Then** the expected impact on those flows is listed explicitly

## Outcome

- Spike report: [runtime-mode-decision.md](../runtime-mode-decision.md)
