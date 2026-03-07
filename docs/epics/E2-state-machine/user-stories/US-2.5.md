# US-2.5 — Full table-driven test matrix for all valid transitions

## Epic
E2: State Machine

## Goal
Achieve 100% branch coverage on `machine.go` by exhaustively testing every valid (from-state, event, expected-to-state) triple defined in analysis.md §5.2.

## Acceptance Criteria

```
Given the complete transition table in analysis.md §5.2
When `go test ./internal/fsm/... -race -cover` is executed
Then statement coverage for `machine.go` is 100%
  And every valid (from, event, to) triple has a corresponding test row
```

```
Given a table-driven test iterating over all valid transitions
When each row is executed independently with a freshly constructed Machine
Then `Transition()` returns nil error
  And `Machine.State()` equals the expected destination state
```

## Tasks

1. [ ] Enumerate all valid (from, event, to) triples from analysis.md §5.2 in a `validTransitions` slice (write test data first)
2. [ ] Write `TestValidTransitions` iterating the slice, constructing a fresh Machine per row, and asserting state and nil error
3. [ ] Include the epic-boundary guard test rows: MERGING+epic→REFACTORING and MERGING+non-epic→SCANNING
4. [ ] Include all retry-budget exhaustion rows: AWAITING_PR/AWAITING_CI at max budget → PAUSED
5. [ ] Run `go test ./internal/fsm/... -race -cover` and confirm 100% statement coverage

## Dependencies
- US-2.4

## Size Estimate
M
