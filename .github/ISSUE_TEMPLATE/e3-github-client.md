name: "E3 — GitHub Client"
about: Implement the minimal GitHub REST API wrapper Loom needs, fully tested with httptest.
title: "E3: GitHub Client"
labels: ["epic", "E3", "TH1"]
---

## Goal

Implement a minimal GitHub REST API wrapper that covers all operations needed by the Loom FSM, tested entirely with `httptest` — no real GitHub token required.

## User Stories

- [ ] US-3.1 — Define `GitHubClient` interface covering all required operations
- [ ] US-3.2 — Implement issue operations: create, add comment, close
- [ ] US-3.3 — Implement PR operations: list, get status, get check-runs, merge
- [ ] US-3.4 — Implement review operations: request review, get review status
- [ ] US-3.5 — Implement tag/release creation
- [ ] US-3.6 — Implement rate-limit handling (respect `X-RateLimit-Remaining`)
- [ ] US-3.7 — `httptest`-based tests for all operations using recorded fixtures

## Acceptance Criteria

- [ ] `GitHubClient` interface defined in `internal/github/client.go`
- [ ] All operations have at least one success test and one error test
- [ ] No real GitHub API calls in tests
- [ ] Rate-limit header respected: client backs off exponentially when below 10% remaining
- [ ] All errors wrapped with context: `fmt.Errorf("creating issue: %w", err)`
- [ ] `go test ./internal/github/... -race` exits 0
