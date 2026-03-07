# US-5.3 — `loom status` — print current state, phase, PR, last 20 log lines

## Epic
E5: CLI

## Assigned Agent

**[Backend Developer](../../../../.github/agents/backend-developer.md)** — apply [`loom-architecture`](../../../../.github/skills/loom-architecture.md) · [`go-standards`](../../../../.github/skills/go-standards.md) · [`tdd-workflow`](../../../../.github/skills/tdd-workflow.md).


## Goal
Add the `loom status` subcommand that reads the latest checkpoint from the store and prints a human-readable summary including state, phase, PR number, and the last 20 log lines.

## Acceptance Criteria

```
Given a checkpoint exists with state AWAITING_CI, phase 3, PR #42
When `loom status` is executed
Then it prints `State: AWAITING_CI`, `Phase: 3`, `PR: #42` to stdout
  And it exits 0
```

```
Given no checkpoint exists
When `loom status` is executed
Then it prints `"No active session"` to stdout
  And it exits 0 (not an error)
```

```
Given structured JSON log entries exist in the log file
When `loom status` is executed
Then the last 20 log lines are printed below the state summary
```

## Tasks

1. [ ] Write `status_cmd_test.go` with checkpoint-present and empty-store cases (write tests first)
2. [ ] Create `cmd/loom/cmd_status.go` with a `statusCmd` cobra.Command
3. [ ] Read latest checkpoint from store and format output
4. [ ] Read last 20 lines from the log file and append to output
5. [ ] Handle missing checkpoint with `"No active session"` message
6. [ ] Register `statusCmd` on the root command
7. [ ] Run `go test ./cmd/loom/... -race` and confirm green

## Dependencies
- US-5.7

## Size Estimate
S
