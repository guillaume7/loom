# US-1.2 — Add package skeleton

## Epic
E1: Project Foundation

## Goal
Create all required package directories with minimal stub files so every internal package path is importable and `go build ./...` succeeds.

## Acceptance Criteria

```
Given the Go module is initialised (US-1.1)
When `go build ./...` is executed
Then every package under `cmd/loom/`, `internal/fsm/`, `internal/github/`, `internal/mcp/`, `internal/store/`, `internal/config/` compiles without error
  And `cmd/loom/main.go` contains a runnable `main()` that exits 0
```

```
Given the package skeleton is committed
When `go vet ./...` is executed
Then it exits 0 with no diagnostics
```

```
Given the package skeleton is committed
When `go test ./...` is executed
Then it exits 0 (empty test files are acceptable at this stage)
```

## Tasks

1. [ ] Write a compilation smoke-test in each package that fails if the package does not exist (write test first)
2. [ ] Create `cmd/loom/main.go` with a `main()` stub using `cobra.Command`
3. [ ] Create `internal/fsm/machine.go`, `states.go`, `events.go` with package declarations only
4. [ ] Create `internal/github/client.go`, `types.go` with package declarations only
5. [ ] Create `internal/mcp/server.go`, `tools.go` with package declarations only
6. [ ] Create `internal/store/sqlite.go` with package declaration only
7. [ ] Create `internal/config/config.go` with package declaration only
8. [ ] Run `go build ./...` and `go vet ./...` and confirm both exit 0

## Dependencies
- US-1.1

## Size Estimate
S
