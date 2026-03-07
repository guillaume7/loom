# US-6.4 — `loom resume` re-opens the session from the last checkpoint

## Epic
E6: Session Management

## Goal
Ensure `loom resume` reads the PAUSED checkpoint from the store, transitions the FSM back to the checkpointed pre-pause state, and re-enters the FSM action loop from that state.

## Acceptance Criteria

```
Given a PAUSED checkpoint exists with the pre-pause `state` field set to `"AWAITING_CI"`
When `loom resume` is executed
Then the FSM is initialised at AWAITING_CI (not IDLE)
  And the command prints `"Resuming from AWAITING_CI"` to stdout
  And the FSM loop continues polling as if it never stopped
```

```
Given no checkpoint exists
When `loom resume` is executed
Then it prints `"Nothing to resume"` to stdout
  And exits 0
```

```
Given a PAUSED checkpoint with `reason: "stall: ..."`
When `loom resume` is executed
Then the stall reason is logged at `slog.Info` level before resuming
```

## Tasks

1. [ ] Write `resume_session_test.go` with PAUSED-checkpoint, no-checkpoint, and stall-reason cases (write tests first)
2. [ ] Update `cmd_resume.go` to read the checkpoint's `state` field and initialise the FSM at that state
3. [ ] Log the stall reason if present before entering the loop
4. [ ] Reset the stall timer on resume so the 5-minute window restarts
5. [ ] Run `go test ./cmd/loom/... -race` and confirm green

## Dependencies
- US-6.3
- US-5.4

## Size Estimate
S
