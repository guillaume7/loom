# US-5.2 — `loom start` — begin from IDLE or resume from checkpoint

## Epic
E5: CLI

## Assigned Agent

**[Backend Developer](../../../../.github/agents/backend-developer.md)** — apply [`loom-architecture`](../../../../.github/skills/loom-architecture.md) · [`go-standards`](../../../../.github/skills/go-standards.md) · [`tdd-workflow`](../../../../.github/skills/tdd-workflow.md).


## Goal
Add the `loom start` subcommand that reads any existing checkpoint, prints the current state, and enters the FSM-driven action loop — or resumes from the persisted state if a prior checkpoint exists.

## Acceptance Criteria

```
Given no checkpoint exists in the store
When `loom start` is executed
Then the FSM starts in state IDLE
  And the command prints `"Starting from IDLE"` to stdout
  And the FSM loop begins
```

```
Given a checkpoint exists in the store with state AWAITING_CI
When `loom start` is executed
Then the command prints `"Resuming from AWAITING_CI"` to stdout
  And the FSM loop resumes from AWAITING_CI without re-executing prior steps
```

```
Given `loom start` is running
When Ctrl-C is pressed
Then the current state is written to the store as a PAUSED checkpoint
  And the process exits 0
```

## Tasks

1. [ ] Write `start_cmd_test.go` with IDLE-start and checkpoint-resume cases using a mock store (write tests first)
2. [ ] Create `cmd/loom/cmd_start.go` with a `startCmd` cobra.Command
3. [ ] Read checkpoint from store on startup and choose initial FSM state
4. [ ] Print start/resume message to stdout before entering the loop
5. [ ] Implement a `signal.NotifyContext` loop that writes a PAUSED checkpoint on SIGINT
6. [ ] Register `startCmd` on the root command
7. [ ] Run `go test ./cmd/loom/... -race` and confirm green

## Dependencies
- US-5.1
- US-5.7

## Size Estimate
M
