---
id: TH3.E2.US4
title: "Resume deduplication"
type: standard
priority: medium
size: M
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "Duplicate wake-ups or concurrent resume attempts against the same run are detected by stable deduplication keys or locks"
  - AC2: "A duplicate resume attempt does not re-execute the same external action twice"
  - AC3: "The runtime records when a wake-up was skipped as duplicate and why"
depends-on: [TH3.E2.US2, TH3.E2.US3]
---

# TH3.E2.US4 — Resume Deduplication

**As a** Loom operator, **I want** duplicate resumes to be suppressed safely, **so that** retries, events, and polling do not cause repeated side effects.

## Acceptance Criteria

- [ ] AC1: Duplicate wake-ups or concurrent resume attempts against the same run are detected by stable deduplication keys or locks
- [ ] AC2: A duplicate resume attempt does not re-execute the same external action twice
- [ ] AC3: The runtime records when a wake-up was skipped as duplicate and why

## BDD Scenarios

### Scenario: Poll and event target the same resume window
- **Given** both polling and an event indicate the same run should resume
- **When** Loom evaluates the queued work
- **Then** it executes the resume logic only once

### Scenario: Duplicate wake-up is skipped safely
- **Given** a matching wake-up has already been claimed or completed
- **When** another equivalent wake-up is encountered
- **Then** Loom skips it as duplicate
- **And** it records the skip reason

### Scenario: Duplicate suppression prevents repeated external side effects
- **Given** a resume path would trigger an external action
- **When** the duplicate path is suppressed
- **Then** the external action is not performed twice
