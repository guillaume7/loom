# US-7.3 — Implement `ReadCheckpoint` returning zero-value when empty

## Epic
E7: Checkpointing

## Assigned Agent

**[Backend Developer](../../../../.github/agents/backend-developer.md)** — apply [`loom-architecture`](../../../../.github/skills/loom-architecture.md) · [`go-standards`](../../../../.github/skills/go-standards.md) · [`tdd-workflow`](../../../../.github/skills/tdd-workflow.md).


## Goal
Implement `Store.ReadCheckpoint() (Checkpoint, error)` so it returns the most recent checkpoint row, or a zero-value `Checkpoint{}` with a nil error when the table is empty.

## Acceptance Criteria

```
Given an empty `:memory:` store
When `ReadCheckpoint()` is called
Then a zero-value `Checkpoint{}` is returned
  And the error is nil
```

```
Given two checkpoints have been written with different `updated_at` values
When `ReadCheckpoint()` is called
Then the checkpoint with the latest `updated_at` is returned
```

```
Given a checkpoint with `retry_counts` stored as JSON
When `ReadCheckpoint()` is called
Then the returned `Checkpoint.RetryCounts` map equals the original map
```

## Tasks

1. [ ] Write `sqlite_read_test.go` with empty-store, latest-row, and retry-counts cases (write tests first)
2. [ ] Implement `ReadCheckpoint` using `SELECT ... ORDER BY updated_at DESC LIMIT 1`
3. [ ] Return `Checkpoint{}` (not an error) when `sql.ErrNoRows` is returned
4. [ ] Deserialise `retry_counts` JSON back to `map[string]int`
5. [ ] Run `go test ./internal/store/... -race` and confirm green

## Dependencies
- US-7.2

## Size Estimate
S
