# US-3.6 — Implement rate-limit handling

## Epic
E3: GitHub Client

## Assigned Agent

**[Backend Developer](../../../../.github/agents/backend-developer.md)** — apply [`loom-architecture`](../../../../.github/skills/loom-architecture.md) · [`go-standards`](../../../../.github/skills/go-standards.md) · [`tdd-workflow`](../../../../.github/skills/tdd-workflow.md).


## Goal
Respect `X-RateLimit-Remaining` and `X-RateLimit-Reset` response headers so the client backs off exponentially when the GitHub API quota drops below 10% and never returns a hard 429 error to callers as an unexpected failure.

## Acceptance Criteria

```
Given an `httptest.Server` that sets `X-RateLimit-Remaining: 0` and `X-RateLimit-Reset: {unix+1s}`
When any client method is called
Then the client waits until after the reset time before retrying
  And the eventual retry succeeds
```

```
Given an `httptest.Server` that sets `X-RateLimit-Remaining: 5` and `X-RateLimit-Limit: 60` (below 10%)
When any client method is called
Then the client logs a warning with `slog.Warn` before proceeding
```

```
Given an `httptest.Server` that returns HTTP 429
When any client method is called
Then the client retries with exponential backoff up to 3 times
  And returns an error only after all retries are exhausted
```

## Tasks

1. [ ] Write `client_ratelimit_test.go` with httptest tests for all three rate-limit scenarios (write tests first)
2. [ ] Add a `checkRateLimit(resp *http.Response) error` helper that inspects headers
3. [ ] Call `checkRateLimit` after every API response and sleep if `Remaining == 0`
4. [ ] Implement exponential backoff for HTTP 429 responses with max 3 retries
5. [ ] Log rate-limit warnings via `slog.Warn`
6. [ ] Run `go test ./internal/github/... -race` and confirm green

## Dependencies
- US-3.2

## Size Estimate
S
