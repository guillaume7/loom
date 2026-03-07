# US-5.5 — `loom reset` — clear state with confirmation prompt

## Epic
E5: CLI

## Goal
Add the `loom reset` subcommand that deletes all checkpoint rows from the store after prompting the user for confirmation, preventing accidental state loss.

## Acceptance Criteria

```
Given a checkpoint exists in the store
When `loom reset` is executed and the user types `y` at the prompt
Then all rows are deleted from the store
  And the command prints `"State cleared."` and exits 0
```

```
Given a checkpoint exists in the store
When `loom reset` is executed and the user types `N` at the prompt
Then no rows are deleted
  And the command prints `"Aborted."` and exits 0
```

```
Given `loom reset` is executed with `--force` flag
When executed
Then no confirmation prompt is shown and state is cleared immediately
```

## Tasks

1. [ ] Write `reset_cmd_test.go` with confirm-yes, confirm-no, and force-flag cases (write tests first)
2. [ ] Create `cmd/loom/cmd_reset.go` with a `resetCmd` cobra.Command
3. [ ] Print `"Are you sure? [y/N] "` prompt and read from stdin
4. [ ] Call `store.DeleteAll()` only on `y` or `--force`
5. [ ] Register `resetCmd` on the root command
6. [ ] Run `go test ./cmd/loom/... -race` and confirm green

## Dependencies
- US-5.7

## Size Estimate
S
