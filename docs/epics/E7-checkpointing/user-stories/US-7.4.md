# US-7.4 — `loom start` reads checkpoint on startup, resumes from persisted state

## Epic
E7: Checkpointing

## Assigned Agent

**[Backend Developer](../../../../.github/agents/backend-developer.md)** — apply [`loom-architecture`](../../../../.github/skills/loom-architecture.md) · [`go-standards`](../../../../.github/skills/go-standards.md) · [`tdd-workflow`](../../../../.github/skills/tdd-workflow.md).


## Goal
Wire the SQLite store into the `loom start` command so that on every startup the FSM is initialised from the latest persisted checkpoint rather than always starting from IDLE.

## Acceptance Criteria

```
Given a checkpoint with `state: "AWAITING_CI"` exists in `.loom/state.db`
When `loom start` is executed
Then the FSM starts in AWAITING_CI (not IDLE)
  And the command prints `"Resuming from AWAITING_CI"` to stdout
```

```
Given no checkpoint exists
When `loom start` is executed
Then the FSM starts in IDLE
  And the command prints `"Starting from IDLE"` to stdout
```

```
Given `loom start` runs and the FSM advances from AWAITING_CI to REVIEWING
When the process is killed and restarted
Then on restart the FSM begins at REVIEWING (the last written checkpoint)
```

## Tasks

1. [ ] Write `start_resume_test.go` using a mock store to inject a pre-existing checkpoint (write test first)
2. [ ] Update `cmd_start.go` to call `store.ReadCheckpoint()` before constructing the FSM
3. [ ] Pass the checkpoint state as the initial state to `fsm.NewMachine(initialState)`
4. [ ] Pass checkpoint `Phase`, `PRNumber`, and `RetryCounts` to the FSM via the Config
5. [ ] Run `go test ./cmd/loom/... -race` and confirm green

## Dependencies
- US-7.3
- US-5.2

## Size Estimate
M
