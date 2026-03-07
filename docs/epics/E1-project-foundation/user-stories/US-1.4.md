# US-1.4 — Confirm CI pipeline green on `main`

## Epic
E1: Project Foundation

## Goal
Add a GitHub Actions workflow that runs `go vet`, `golangci-lint`, `go test`, and cross-compilation on every push to `main` and every pull request.

## Acceptance Criteria

```
Given `.github/workflows/ci.yml` is committed
When a push is made to `main`
Then the CI workflow triggers automatically
  And all jobs (`vet`, `lint`, `test`, `cross-compile`) pass
  And the workflow exits 0
```

```
Given the CI workflow is running
When the `cross-compile` job executes
Then binaries are produced for linux/amd64, darwin/arm64, and windows/amd64 without error
```

```
Given `.github/copilot/mcp.json` does not yet exist
When the CI workflow runs
Then it exits 0 (mcp.json is not required at E1)
```

## Tasks

1. [ ] Write a minimal failing workflow to confirm CI triggers (test-first: expect red, then fix to green)
2. [ ] Create `.github/workflows/ci.yml` with jobs: `vet`, `lint`, `test`, `cross-compile`
3. [ ] Add `cross-compile` job matrix: `{os: linux, arch: amd64}`, `{os: darwin, arch: arm64}`, `{os: windows, arch: amd64}`
4. [ ] Add `golangci-lint-action` step pinned to a stable version
5. [ ] Create `.github/copilot/mcp.json` stub referencing `loom mcp` as the MCP command
6. [ ] Push to `main` and confirm all CI jobs show green

## Dependencies
- US-1.3

## Size Estimate
S
