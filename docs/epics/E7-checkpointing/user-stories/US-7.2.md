# US-7.2 — Implement `WriteCheckpoint` with upsert semantics

## Epic
E7: Checkpointing

## Goal
Implement `Store.WriteCheckpoint(cp Checkpoint) error` using `INSERT OR REPLACE` so writing the same checkpoint twice is idempotent and the latest value always wins.

## Acceptance Criteria

```
Given an open `:memory:` store
When `WriteCheckpoint(cp)` is called with a valid `Checkpoint`
Then the row is inserted and `ReadCheckpoint()` returns the same data
  And no error is returned
```

```
Given `WriteCheckpoint(cp)` has already been called once
When `WriteCheckpoint(cp)` is called again with the same `id`
Then the row is updated (not duplicated)
  And `ReadCheckpoint()` returns the latest values
```

```
Given a `Checkpoint` with a non-nil `RetryCounts` map
When `WriteCheckpoint(cp)` is called
Then the map is serialised to JSON and stored in the `retry_counts` column
  And `ReadCheckpoint()` deserialises it back to the original map
```

## Tasks

1. [ ] Write `sqlite_write_test.go` with insert, upsert, and retry-counts-round-trip cases (write tests first)
2. [ ] Define `Checkpoint` struct in `internal/store/sqlite.go` with all required fields
3. [ ] Implement `WriteCheckpoint` using `INSERT OR REPLACE INTO checkpoints ...`
4. [ ] Serialise `RetryCounts map[string]int` to JSON for storage
5. [ ] Run `go test ./internal/store/... -race` and confirm green

## Dependencies
- US-7.1

## Size Estimate
S
