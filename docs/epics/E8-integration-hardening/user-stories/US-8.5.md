# US-8.5 — Cross-compilation matrix verified in CI

## Epic
E8: Integration & Hardening

## Assigned Agent

**[DevOps / Release Manager](../../../../.github/agents/devops-release.md)** — apply [`git-branching-workflow`](../../../../.github/skills/git-branching-workflow.md).


## Goal
Add a CI job matrix that cross-compiles the `loom` binary for all 5 target platforms (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64) with `CGO_ENABLED=0` and confirms each binary is non-zero in size.

## Acceptance Criteria

```
Given the CI workflow includes a `cross-compile` matrix job
When a push is made to `main`
Then binaries are produced for: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64
  And each binary is larger than 0 bytes
  And no CGo is used in any build (`CGO_ENABLED=0`)
```

```
Given the `modernc.org/sqlite` pure-Go driver is used
When `CGO_ENABLED=0 go build ./cmd/loom` is run for all 5 platforms
Then all builds succeed without CGo linker errors
```

```
Given a cross-compiled linux/amd64 binary
When it is executed on a linux/amd64 runner with `--help`
Then it exits 0 and prints usage information
```

## Tasks

1. [ ] Write a CI check-script that exits non-zero if any binary is zero bytes (write test first)
2. [ ] Update `.github/workflows/ci.yml` with a `cross-compile` matrix for all 5 platform/arch combos
3. [ ] Set `CGO_ENABLED=0 GOOS=$OS GOARCH=$ARCH go build -o loom-$OS-$ARCH ./cmd/loom` in each matrix step
4. [ ] Upload built binaries as workflow artifacts
5. [ ] Add a smoke-test step on linux/amd64 that runs `./loom-linux-amd64 --help`
6. [ ] Push to `main` and confirm all 5 matrix jobs pass

## Dependencies
- US-8.1

## Size Estimate
S
