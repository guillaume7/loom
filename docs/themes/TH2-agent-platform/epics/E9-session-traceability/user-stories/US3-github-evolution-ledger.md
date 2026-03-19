---
id: TH2.E9.US3
title: "GitHub issue and PR evolution ledger"
type: standard
priority: medium
size: L
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "The trace records first-seen metadata for each issue and pull request touched by the session"
  - AC2: "Meaningful GitHub state changes append ledger entries with prior state, current state, and terminal outcome when applicable"
  - AC3: "PR ledger entries include draft_or_ready status, review summary, CI summary, mergeability, and merged state"
  - AC4: "Repeated polls with unchanged GitHub state do not append duplicate ledger entries"
depends-on: [TH2.E9.US1]
---

# TH2.E9.US3 — GitHub Issue and PR Evolution Ledger

**As a** release engineer, **I want** GitHub issue and pull request evolution transcribed into the session trace, **so that** I can replay cross-boundary behavior without re-querying GitHub.

## Acceptance Criteria

- [ ] AC1: The trace records first-seen metadata for each issue and pull request touched by the session
- [ ] AC2: Meaningful GitHub state changes append ledger entries with prior state, current state, and terminal outcome when applicable
- [ ] AC3: PR ledger entries include `draft_or_ready` status, review summary, CI summary, mergeability, and merged state
- [ ] AC4: Repeated polls with unchanged GitHub state do not append duplicate ledger entries

## BDD Scenarios

### Scenario: First-seen PR metadata is recorded
- **Given** Loom observes pull request 42 for the first time during a session
- **When** the PR is associated with the session
- **Then** the session trace appends a first-seen entry with PR number, title, and initial state metadata

### Scenario: PR state evolution is recorded as distinct changes
- **Given** a pull request moves from draft to ready and later receives approval
- **When** Loom polls GitHub after each change
- **Then** the session trace appends distinct ledger entries for the ready transition and the review change

### Scenario: Unchanged GitHub state is not duplicated
- **Given** Loom polls GitHub for an issue or pull request and the observed state is unchanged
- **When** transcription logic evaluates the snapshot
- **Then** no duplicate GitHub ledger entry is appended