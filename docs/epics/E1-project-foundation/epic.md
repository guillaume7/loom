# E1: Project Foundation

## Goal

Create a compilable, linted, CI-green Go module scaffold with all configurations
that every subsequent epic depends on.

## Description

Before any Loom logic can be written, the repository needs:
- A valid `go.mod` with approved dependencies
- The standard package layout (`cmd/loom/`, `internal/*`)
- A passing CI pipeline (go vet, golangci-lint, go test, cross-compile)
- `.github/copilot/mcp.json` referencing Loom itself as an MCP server
- An empty `cmd/loom/main.go` that compiles

This epic is bootstrapped **manually** by the DevOps agent. All subsequent
epics run through `loom start`.

## User Stories

- [ ] US-1.1 — Initialise Go module with approved dependencies
- [ ] US-1.2 — Add package skeleton (`cmd/loom/`, `internal/fsm/`, `internal/github/`, `internal/mcp/`, `internal/store/`, `internal/config/`)
- [ ] US-1.3 — Configure `.golangci.yml` with required linters
- [ ] US-1.4 — Confirm CI pipeline green on `main`

## Dependencies

None — this is the first epic.

## Acceptance Criteria

- [ ] `go build ./cmd/loom` exits 0
- [ ] `go vet ./...` exits 0
- [ ] `golangci-lint run ./...` exits 0
- [ ] `go test ./...` exits 0 (empty test suite passes)
- [ ] CI pipeline runs and passes on every push to `main`
- [ ] Cross-compilation succeeds: linux/amd64, darwin/arm64, windows/amd64
