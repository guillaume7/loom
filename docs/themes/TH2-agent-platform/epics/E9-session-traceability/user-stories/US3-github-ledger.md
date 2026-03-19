---
id: TH2.E9.US3
title: "GitHub issue and PR evolution ledger"
status: done
epic: E9
depends-on: [TH2.E9.US2]
---

# TH2.E9.US3 — GitHub issue and PR evolution ledger

## As a
Loom operator

## I want
every GitHub issue and pull request touched during a session to appear in a
dedicated ledger within the session trace

## So that
I can trace the lifecycle of each issue and PR from creation through merge
without having to cross-reference multiple data sources.

## Acceptance Criteria

- [ ] Every trace event records the `pr_number` and `issue_number` from the
      checkpoint at the time of the transition.
- [ ] The `loom://trace` resource renders a **GitHub Ledger** section listing
      each unique PR and issue number touched during the session.
- [ ] PR numbers and issue numbers are de-duplicated in the ledger (same issue
      appearing in multiple events is listed once).

## BDD Scenarios

```gherkin
Given a checkpoint transition where pr_number=42 and issue_number=7
When the operator reads loom://trace
Then the GitHub Ledger section contains "PR #42" and "Issue #7"

Given two checkpoints both referencing pr_number=42
When the operator reads loom://trace
Then PR #42 appears exactly once in the GitHub Ledger
```
