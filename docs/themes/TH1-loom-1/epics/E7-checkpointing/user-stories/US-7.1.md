# US-7.1 — Define SQLite schema: `checkpoints` table

## Epic
E7: Checkpointing

## Assigned Agent

**[Architect](../../../../.github/agents/architect.md)** — apply [`loom-architecture`](../../../../.github/skills/loom-architecture.md) · [`go-standards`](../../../../.github/skills/go-standards.md).

**[Backend Developer](../../../../.github/agents/backend-developer.md)** — apply [`loom-architecture`](../../../../.github/skills/loom-architecture.md) · [`go-standards`](../../../../.github/skills/go-standards.md) · [`tdd-workflow`](../../../../.github/skills/tdd-workflow.md).


## Goal
Create the `checkpoints` table DDL and run it on first open so `internal/store/sqlite.go` has a stable schema every subsequent operation can rely on.

## Acceptance Criteria

```
Given an empty `:memory:` database
When `store.New(":memory:")` is called
Then the `checkpoints` table is created without error
  And the table has columns: `id`, `state`, `previous_state`, `phase`, `pr_number`, `issue_number`, `retry_counts` (JSON), `reason`, `updated_at`
```

```
Given `store.New(".loom/state.db")` is called
When the directory `.loom/` does not exist
Then the directory is created automatically
  And the database file is created at `.loom/state.db`
```

```
Given the schema migration has already been applied
When `store.New(":memory:")` is called a second time on the same database
Then it exits 0 (idempotent DDL using `CREATE TABLE IF NOT EXISTS`)
```

## Tasks

1. [ ] Write `sqlite_schema_test.go` asserting table existence and column list after `New()` (write tests first)
2. [ ] Define `const createTableSQL` in `internal/store/sqlite.go`
3. [ ] Implement `New(dsn string) (*Store, error)` that opens the database and runs the DDL
4. [ ] Create the parent directory for the DSN path if it does not exist (skip for `:memory:`)
5. [ ] Run `go test ./internal/store/... -race` and confirm green

## Dependencies
- US-4.3

## Size Estimate
S
