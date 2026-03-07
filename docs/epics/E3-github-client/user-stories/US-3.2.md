# US-3.2 — Implement issue operations: create, add comment, close

## Epic
E3: GitHub Client

## Assigned Agent

**[Backend Developer](../../../../.github/agents/backend-developer.md)** — apply [`loom-architecture`](../../../../.github/skills/loom-architecture.md) · [`go-standards`](../../../../.github/skills/go-standards.md) · [`tdd-workflow`](../../../../.github/skills/tdd-workflow.md).


## Goal
Provide working implementations of `CreateIssue`, `AddComment`, and `CloseIssue` in `internal/github/client.go` that call the GitHub REST API and correctly handle success and API-error responses.

## Acceptance Criteria

```
Given an `httptest.Server` returning a 201 response for `POST /repos/{owner}/{repo}/issues`
When `client.CreateIssue(ctx, title, body, labels)` is called
Then the returned `Issue.Number` matches the fixture response
  And no error is returned
```

```
Given an `httptest.Server` returning a 422 response
When `client.CreateIssue(ctx, ...)` is called
Then a non-nil error is returned containing the HTTP status code
  And the error is wrapped with context: `"creating issue: ..."`
```

```
Given an `httptest.Server` returning 201 for `POST /repos/{owner}/{repo}/issues/{number}/comments`
When `client.AddComment(ctx, issueNumber, body)` is called
Then no error is returned
```

```
Given an `httptest.Server` returning 200 for `PATCH /repos/{owner}/{repo}/issues/{number}` with `state=closed`
When `client.CloseIssue(ctx, issueNumber)` is called
Then no error is returned
```

## Tasks

1. [ ] Write `client_issue_test.go` with httptest table-driven tests for CreateIssue success, CreateIssue error, AddComment, CloseIssue (write tests first)
2. [ ] Create `internal/github/client.go` concrete `Client` struct with `baseURL`, `token`, `owner`, `repo` fields
3. [ ] Implement `CreateIssue` using `net/http` with JSON body and Bearer token header
4. [ ] Implement `AddComment` using `POST /issues/{number}/comments`
5. [ ] Implement `CloseIssue` using `PATCH /issues/{number}` with `{"state":"closed"}`
6. [ ] Wrap all errors with `fmt.Errorf("<operation>: %w", err)`
7. [ ] Run `go test ./internal/github/... -race` and confirm green

## Dependencies
- US-3.1

## Size Estimate
M
