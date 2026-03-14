---
id: TH2.E6.US1
title: "Elicitation schema definition and emission"
type: standard
priority: medium
size: M
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "Elicitation JSON schema is defined with action enum: skip, reassign, pause_epic"
  - AC2: "Budget exhaustion triggers elicitation emission instead of immediate PAUSED"
  - AC3: "Elicitation includes title, description, and enumDescriptions for each choice"
  - AC4: "Elicitation is emitted via MCP protocol to the connected client"
depends-on: []
---

# TH2.E6.US1 — Elicitation Schema Definition and Emission

**As a** Loom operator, **I want** structured elicitation prompts when retry budgets are exhausted, **so that** I can choose recovery actions from the chat UI without switching to the terminal.

## Acceptance Criteria

- [ ] AC1: Elicitation JSON schema is defined with action enum: `skip`, `reassign`, `pause_epic`
- [ ] AC2: Budget exhaustion triggers elicitation emission instead of immediate `PAUSED`
- [ ] AC3: Elicitation includes title, description, and `enumDescriptions` for each choice
- [ ] AC4: Elicitation is emitted via MCP protocol to the connected client

## BDD Scenarios

### Scenario: Budget exhaustion triggers elicitation
- **Given** FSM retry_count reaches the configured budget limit for CI checks
- **When** the budget is exhausted
- **Then** an elicitation is emitted with title "PR #42 — CI budget exhausted"
- **And** the schema offers choices: skip, reassign, pause_epic

### Scenario: Elicitation schema is well-formed
- **Given** a budget exhaustion event
- **When** the elicitation JSON is inspected
- **Then** it contains `"type":"elicitation"`, `"title"`, `"description"`, and `"schema"` fields
- **And** the schema has `"action":{"type":"string","enum":["skip","reassign","pause_epic"]}`

### Scenario: Elicitation blocks until operator responds
- **Given** an elicitation has been emitted
- **When** the operator has not yet responded
- **Then** the FSM does NOT transition to PAUSED
- **And** the workflow waits for the operator's choice

### Scenario: Multiple budget exhaustions produce separate elicitations
- **Given** two different PRs exhaust their budgets
- **When** elicitations are emitted
- **Then** each has a distinct title referencing its PR number
