# US-4.6 — Implement `loom_abort` tool

## Epic
E4: MCP Server

## Goal
Implement the `loom_abort` tool handler so it immediately transitions the FSM to PAUSED, persists the checkpoint with the abort reason, and returns a summary to the session.

## Acceptance Criteria

```
Given the FSM is in any non-PAUSED state
When the session calls `loom_abort` with `{"reason": "user requested stop"}`
Then the FSM transitions to PAUSED
  And a checkpoint is written to the store with state PAUSED and the provided reason
  And the response contains `"state": "PAUSED"` and `"reason": "user requested stop"`
```

```
Given the FSM is already in state PAUSED
When `loom_abort` is called
Then the response contains `"state": "PAUSED"` (idempotent)
  And no error is returned
```

```
Given `loom_abort` is called without a reason field
When the handler processes it
Then the reason defaults to `"abort requested"` in the stored checkpoint
```

## Tasks

1. [ ] Write `tools_abort_test.go` with non-PAUSED, already-PAUSED, and no-reason cases (write tests first)
2. [ ] Define `AbortRequest` and `AbortResponse` structs
3. [ ] Implement `handleAbort` firing `EventAbort` on the Machine and writing the checkpoint
4. [ ] Default reason to `"abort requested"` when the field is empty
5. [ ] Wire handler into server registration
6. [ ] Run `go test ./internal/mcp/... -race` and confirm green

## Dependencies
- US-4.1

## Size Estimate
S
