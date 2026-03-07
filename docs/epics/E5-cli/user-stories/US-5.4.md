# US-5.4 — `loom pause` / `loom resume` — graceful pause and continue

## Epic
E5: CLI

## Goal
Add `loom pause` and `loom resume` subcommands so a human operator can gracefully halt the running FSM loop and later resume it from the persisted state.

## Acceptance Criteria

```
Given `loom start` is running in a gate state
When `loom pause` is executed in a separate terminal
Then a PAUSED checkpoint is written to the store
  And `loom start` exits 0 within 5 seconds
  And a log line `"paused by operator"` is emitted
```

```
Given a PAUSED checkpoint exists in the store
When `loom resume` is executed
Then the process prints `"Resuming from <state>"` to stdout
  And the FSM loop re-enters at the checkpointed state
```

```
Given no PAUSED checkpoint exists
When `loom resume` is executed
Then it prints `"Nothing to resume"` and exits 0
```

## Tasks

1. [ ] Write `pause_resume_cmd_test.go` with pause-signal and no-checkpoint cases (write tests first)
2. [ ] Create `cmd/loom/cmd_pause.go` that writes a PAUSED checkpoint with reason `"paused by operator"`
3. [ ] Create `cmd/loom/cmd_resume.go` that calls `loom start` logic with an existing checkpoint
4. [ ] Register both subcommands on the root command
5. [ ] Run `go test ./cmd/loom/... -race` and confirm green

## Dependencies
- US-5.2

## Size Estimate
S
