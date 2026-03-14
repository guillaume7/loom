# US-2.2 — Implement the transition table with guard conditions

## Epic
E2: State Machine

## Assigned Agent

**[Architect](../../../../.github/agents/architect.md)** — apply [`loom-architecture`](../../../../.github/skills/loom-architecture.md) · [`go-standards`](../../../../.github/skills/go-standards.md).

**[Backend Developer](../../../../.github/agents/backend-developer.md)** — apply [`loom-architecture`](../../../../.github/skills/loom-architecture.md) · [`tdd-workflow`](../../../../.github/skills/tdd-workflow.md) · [`go-standards`](../../../../.github/skills/go-standards.md).


## Goal
Implement `Machine.Transition(event Event) error` in `internal/fsm/machine.go` backed by a complete, data-driven transition table covering every row in analysis.md §5.2.

## Acceptance Criteria

```
Given the FSM is in state IDLE
When the `start` event is received
Then the FSM state becomes SCANNING
  And no error is returned
```

```
Given the FSM is in state AWAITING_CI
When the `ci_green` event is received
Then the FSM state becomes REVIEWING
  And no error is returned
```

```
Given the FSM is in state AWAITING_CI
When the `ci_red` event is received
Then the FSM state becomes DEBUGGING
  And no error is returned
```

```
Given the FSM is in any state
When an event is received that has no entry in the transition table for the current state
Then an error is returned describing the invalid transition
  And the FSM state is unchanged
```

```
Given the FSM is in state MERGING
When the `merged` event is received with an epic-boundary flag set
Then the FSM state becomes REFACTORING
```

## Tasks

1. [ ] Write `machine_test.go` with a table-driven test covering all transitions in analysis.md §5.2 (write test first)
2. [ ] Define `type Machine struct` in `machine.go` with a `State` field and a transition map
3. [ ] Implement `NewMachine(initialState State) *Machine`
4. [ ] Implement `Machine.Transition(event Event) error` using the transition table
5. [ ] Add guard condition for epic-boundary flag on the MERGING→REFACTORING vs MERGING→SCANNING fork
6. [ ] Run `go test ./internal/fsm/... -race -count=1` and confirm green

## Dependencies
- US-2.1

## Size Estimate
M
