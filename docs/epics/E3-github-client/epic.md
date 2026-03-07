# E3: GitHub Client

## Goal

Implement a minimal GitHub REST API wrapper that covers all operations needed
by the Loom FSM, tested entirely with `httptest` — no real GitHub token required.

## Description

The FSM's gate checks (poll for PR, poll CI, check review status) and actions
(create issue, assign @copilot, post comment, merge PR, create tag) all go
through this package.

The package must be driven by a `GitHubClient` interface, injectable from
outside (enabling mock substitution in MCP tests).

## User Stories

- [ ] US-3.1 — Define `GitHubClient` interface covering all required operations
- [ ] US-3.2 — Implement issue operations: create, add comment, close
- [ ] US-3.3 — Implement PR operations: list, get status, get check-runs, merge
- [ ] US-3.4 — Implement review operations: request review, get review status
- [ ] US-3.5 — Implement tag/release creation
- [ ] US-3.6 — Implement rate-limit handling (respect `X-RateLimit-Remaining`)
- [ ] US-3.7 — `httptest`-based tests for all operations using recorded fixtures

## Dependencies

- E1 (project scaffold)

## Acceptance Criteria

- [ ] `GitHubClient` interface defined in `internal/github/client.go`
- [ ] All operations have at least one success test and one error test
- [ ] No real GitHub API calls in tests
- [ ] Rate-limit header respected: client backs off exponentially when below 10% remaining
- [ ] All errors wrapped with context: `fmt.Errorf("creating issue: %w", err)`
- [ ] `go test ./internal/github/... -race` exits 0
