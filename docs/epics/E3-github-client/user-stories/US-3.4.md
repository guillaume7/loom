# US-3.4 — Implement review operations: request review, get review status

## Epic
E3: GitHub Client

## Assigned Agent

**[Backend Developer](../../../../.github/agents/backend-developer.md)** — apply [`loom-architecture`](../../../../.github/skills/loom-architecture.md) · [`go-standards`](../../../../.github/skills/go-standards.md) · [`tdd-workflow`](../../../../.github/skills/tdd-workflow.md).


## Goal
Provide working implementations of `RequestReview` and `GetReviewStatus` so the FSM can trigger Copilot reviews and poll for their outcome.

## Acceptance Criteria

```
Given an `httptest.Server` returning 201 for `POST /repos/{owner}/{repo}/pulls/{number}/requested_reviewers`
When `client.RequestReview(ctx, prNumber, reviewer)` is called
Then no error is returned
```

```
Given an `httptest.Server` returning a reviews array with `state: "APPROVED"`
When `client.GetReviewStatus(ctx, prNumber)` is called
Then the returned status string is `"APPROVED"`
  And no error is returned
```

```
Given an `httptest.Server` returning a reviews array with `state: "CHANGES_REQUESTED"`
When `client.GetReviewStatus(ctx, prNumber)` is called
Then the returned status string is `"CHANGES_REQUESTED"`
```

```
Given an `httptest.Server` returning an empty reviews array
When `client.GetReviewStatus(ctx, prNumber)` is called
Then the returned status string is `"PENDING"`
  And no error is returned
```

## Tasks

1. [ ] Write `client_review_test.go` with httptest tests for all three review-status scenarios (write tests first)
2. [ ] Implement `RequestReview` using `POST /repos/{owner}/{repo}/pulls/{number}/requested_reviewers`
3. [ ] Implement `GetReviewStatus` using `GET /repos/{owner}/{repo}/pulls/{number}/reviews`, returning the latest non-PENDING state
4. [ ] Wrap all errors with context
5. [ ] Run `go test ./internal/github/... -race` and confirm green

## Dependencies
- US-3.1

## Size Estimate
S
