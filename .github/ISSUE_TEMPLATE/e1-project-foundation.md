name: "E1 — Project Foundation"
about: Create a compilable, linted, CI-green Go module scaffold with the configurations every later epic depends on.
title: "E1: Project Foundation"
labels: ["epic", "E1", "TH1"]
---

## Goal

Create a compilable, linted, CI-green Go module scaffold with all configurations that every subsequent epic depends on.

## User Stories

- [ ] US-1.1 — Initialise Go module with approved dependencies
- [ ] US-1.2 — Add package skeleton (`cmd/loom/`, `internal/fsm/`, `internal/github/`, `internal/mcp/`, `internal/store/`, `internal/config/`)
- [ ] US-1.3 — Configure `.golangci.yml` with required linters
- [ ] US-1.4 — Confirm CI pipeline green on `main`

## Acceptance Criteria

- [ ] `go build ./cmd/loom` exits 0
- [ ] `go vet ./...` exits 0
- [ ] `golangci-lint run ./...` exits 0
- [ ] `go test ./...` exits 0 (empty test suite passes)
- [ ] CI pipeline runs and passes on every push to `main`
- [ ] Cross-compilation succeeds: linux/amd64, darwin/arm64, windows/amd64
