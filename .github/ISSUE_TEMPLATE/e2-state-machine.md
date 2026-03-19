name: "E2 — State Machine"
about: Implement the complete 13-state Loom FSM with retry budgets, valid transitions, and exhaustive tests.
title: "E2: State Machine"
labels: ["epic", "E2", "TH1"]
---

## Goal

Implement the complete 13-state FSM with all valid transitions, retry budgets, and the PAUSED escape state — with 100% branch coverage and zero external dependencies.

## User Stories

- [ ] US-2.1 — Define all State and Event constants with metadata
- [ ] US-2.2 — Implement the transition table with guard conditions
- [ ] US-2.3 — Implement retry budgets per gate state
- [ ] US-2.4 — Implement PAUSED escape state (triggered on budget exhaustion)
- [ ] US-2.5 — Full table-driven test matrix for all valid transitions
- [ ] US-2.6 — Full table-driven test matrix for all invalid transitions

## Acceptance Criteria

- [ ] All 13 states defined: `IDLE`, `SCANNING`, `ISSUE_CREATED`, `AWAITING_PR`, `AWAITING_READY`, `AWAITING_CI`, `REVIEWING`, `DEBUGGING`, `ADDRESSING_FEEDBACK`, `MERGING`, `REFACTORING`, `COMPLETE`, `PAUSED`
- [ ] `Machine.Transition(event)` returns an error for invalid transitions
- [ ] State is never mutated outside `Machine.Transition()`
- [ ] Each gate state (`AWAITING_PR`, `AWAITING_READY`, `AWAITING_CI`) has a configurable retry budget
- [ ] On retry exhaustion: state transitions to `PAUSED`
- [ ] `internal/fsm` imports zero packages outside the Go stdlib
- [ ] `go test ./internal/fsm/... -race -cover` shows 100% statement coverage
