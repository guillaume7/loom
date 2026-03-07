# US-1.3 — Configure `.golangci.yml` with required linters

## Epic
E1: Project Foundation

## Goal
Add a `.golangci.yml` that enforces the project's required linters so CI can gate every PR on lint quality.

## Acceptance Criteria

```
Given `.golangci.yml` is present at the repository root
When `golangci-lint run ./...` is executed against the package skeleton
Then it exits 0
  And the following linters are enabled: `errcheck`, `govet`, `staticcheck`, `revive`, `gofmt`, `misspell`, `unused`
```

```
Given a Go source file with an unchecked error is introduced
When `golangci-lint run ./...` is executed
Then it exits non-zero and reports an `errcheck` violation
```

```
Given a Go source file with a formatting issue is introduced
When `golangci-lint run ./...` is executed
Then it exits non-zero and reports a `gofmt` violation
```

## Tasks

1. [ ] Write a test stub file with a deliberate `errcheck` violation and verify lint fails (test-first validation)
2. [ ] Create `.golangci.yml` with `linters.enable` listing all required linters
3. [ ] Set `run.timeout: 5m` and `issues.max-same-issues: 0` in the config
4. [ ] Remove the deliberate violation stub and confirm `golangci-lint run ./...` exits 0
5. [ ] Document the linter list in a comment block at the top of `.golangci.yml`

## Dependencies
- US-1.2

## Size Estimate
S
