# US-1.1 — Initialise Go module with approved dependencies

## Epic
E1: Project Foundation

## Goal
Create a valid `go.mod` and `go.sum` with all approved direct dependencies pinned so every subsequent epic can import them without modification.

## Acceptance Criteria

```
Given an empty repository with only a README
When `go mod init github.com/loom-io/loom` is run and approved dependencies are added
Then `go.mod` declares module `github.com/loom-io/loom` with Go 1.22 or later
  And direct dependencies include `spf13/cobra`, `mark3labs/mcp-go`, `google/go-github/v62`, `modernc.org/sqlite`, `pelletier/go-toml/v2`
  And `go mod tidy` exits 0 with no diff
  And `go build ./...` exits 0
```

```
Given the `go.sum` file is committed
When a developer clones the repository and runs `go mod download`
Then all modules are fetched without network errors
  And the checksum database verifies every entry
```

## Tasks

1. [ ] Write a test that imports each approved package and fails to compile without it (write test first)
2. [ ] Run `go mod init github.com/loom-io/loom` and set minimum Go version to 1.22
3. [ ] `go get` each approved dependency at the pinned version
4. [ ] Run `go mod tidy` and commit `go.mod` + `go.sum`
5. [ ] Verify `go build ./...` exits 0 on a clean checkout

## Dependencies
None

## Size Estimate
S
