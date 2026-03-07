# US-5.6 — `loom log` — stream structured JSON log

## Epic
E5: CLI

## Assigned Agent

**[Backend Developer](../../../../.github/agents/backend-developer.md)** — apply [`loom-architecture`](../../../../.github/skills/loom-architecture.md) · [`go-standards`](../../../../.github/skills/go-standards.md) · [`tdd-workflow`](../../../../.github/skills/tdd-workflow.md).


## Goal
Add the `loom log` subcommand that tails and streams the structured JSON log file to stdout, supporting both historical output and live-follow mode.

## Acceptance Criteria

```
Given structured JSON log entries exist in the log file
When `loom log` is executed
Then all existing log entries are written to stdout as newline-delimited JSON
  And the command exits 0 after printing all lines
```

```
Given `loom log --follow` is executed
When new log entries are appended to the log file
Then new entries appear on stdout in real time
  And the command blocks until interrupted
```

```
Given `loom log -n 5` is executed
When the log file has more than 5 entries
Then only the last 5 entries are printed
```

## Tasks

1. [ ] Write `log_cmd_test.go` with historical, empty-file, and `-n` flag cases (write tests first)
2. [ ] Create `cmd/loom/cmd_log.go` with a `logCmd` cobra.Command
3. [ ] Implement historical log read with `-n` flag for tail behaviour
4. [ ] Implement `--follow` mode using `fsnotify` or periodic polling
5. [ ] Register `logCmd` on the root command
6. [ ] Run `go test ./cmd/loom/... -race` and confirm green

## Dependencies
- US-5.7

## Size Estimate
S
