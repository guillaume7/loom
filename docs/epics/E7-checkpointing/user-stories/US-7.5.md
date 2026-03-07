# US-7.5 — `loom reset` deletes all rows (with confirmation)

## Epic
E7: Checkpointing

## Goal
Implement `Store.DeleteAll() error` and wire it into `loom reset` so confirmed resets result in an empty `checkpoints` table.

## Acceptance Criteria

```
Given checkpoint rows exist in the store
When `store.DeleteAll()` is called
Then `SELECT COUNT(*) FROM checkpoints` returns 0
  And no error is returned
```

```
Given `loom reset` is executed and the user confirms with `y`
When `store.DeleteAll()` is called
Then `loom status` subsequently prints `"No active session"`
```

```
Given `loom reset` is executed and the user answers `N`
When the command exits
Then the checkpoint rows are unchanged
```

## Tasks

1. [ ] Write `sqlite_delete_test.go` asserting row count is 0 after `DeleteAll()` (write test first)
2. [ ] Implement `DeleteAll()` using `DELETE FROM checkpoints`
3. [ ] Update `cmd_reset.go` to call `store.DeleteAll()` after confirmation
4. [ ] Run `go test ./internal/store/... -race` and `go test ./cmd/loom/... -race` and confirm green

## Dependencies
- US-7.1
- US-5.5

## Size Estimate
S
