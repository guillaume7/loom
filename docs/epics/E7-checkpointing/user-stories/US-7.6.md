# US-7.6 — SQLite tests using `":memory:"` database

## Epic
E7: Checkpointing

## Goal
Ensure the entire `internal/store` test suite runs against an in-memory SQLite database so tests are fast, isolated, and require no filesystem access.

## Acceptance Criteria

```
Given all store tests use `store.New(":memory:")`
When `go test ./internal/store/... -race` is executed
Then no files are created on the filesystem
  And all tests pass in < 2 seconds
  And the `-race` detector reports no data races
```

```
Given two parallel tests each call `store.New(":memory:")`
When they run concurrently
Then each test operates on an independent database (no cross-contamination)
```

```
Given `modernc.org/sqlite` is the database driver
When tests compile
Then no CGo is required (`CGO_ENABLED=0 go test ./internal/store/...` exits 0)
```

## Tasks

1. [ ] Write `sqlite_memory_test.go` with a `newMemStore(t)` helper that returns a `*Store` backed by `:memory:` (write test helper first)
2. [ ] Migrate all existing store tests to use `newMemStore(t)` instead of any filesystem path
3. [ ] Add a parallel-isolation test that creates two stores and writes different data to each
4. [ ] Add a build-tag test asserting `CGO_ENABLED=0` compilation succeeds
5. [ ] Run `go test ./internal/store/... -race -count=1` and confirm all pass with no filesystem side-effects

## Dependencies
- US-7.3

## Size Estimate
M
