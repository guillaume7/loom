# US-3.3 — Implement PR operations: list, get status, get check-runs, merge

## Epic
E3: GitHub Client

## Assigned Agent

**[Backend Developer](../../../../.github/agents/backend-developer.md)** — apply [`loom-architecture`](../../../../.github/skills/loom-architecture.md) · [`go-standards`](../../../../.github/skills/go-standards.md) · [`tdd-workflow`](../../../../.github/skills/tdd-workflow.md).


## Goal
Provide working implementations of `ListPRs`, `GetPR`, `GetCheckRuns`, and `MergePR` so the FSM gate checks can poll pull-request and CI state.

## Acceptance Criteria

```
Given an `httptest.Server` returning a JSON array for `GET /repos/{owner}/{repo}/pulls`
When `client.ListPRs(ctx, branch)` is called
Then the returned slice contains one `PR` per fixture entry
  And each `PR.Number`, `PR.Draft`, and `PR.HeadSHA` are populated
```

```
Given an `httptest.Server` returning a PR object with `draft: false`
When `client.GetPR(ctx, prNumber)` is called
Then `PR.Draft` is false and no error is returned
```

```
Given an `httptest.Server` returning check-runs with `conclusion: "success"`
When `client.GetCheckRuns(ctx, sha)` is called
Then all returned `CheckRun.Conclusion` values equal "success"
```

```
Given an `httptest.Server` returning 200 for `PUT /repos/{owner}/{repo}/pulls/{number}/merge`
When `client.MergePR(ctx, prNumber, commitMessage)` is called
Then no error is returned
```

## Tasks

1. [ ] Write `client_pr_test.go` with httptest tests for all four operations (write tests first)
2. [ ] Implement `ListPRs` using `GET /repos/{owner}/{repo}/pulls?head={branch}&state=open`
3. [ ] Implement `GetPR` using `GET /repos/{owner}/{repo}/pulls/{number}`
4. [ ] Implement `GetCheckRuns` using `GET /repos/{owner}/{repo}/commits/{sha}/check-runs`
5. [ ] Implement `MergePR` using `PUT /repos/{owner}/{repo}/pulls/{number}/merge`
6. [ ] Wrap all errors with context
7. [ ] Run `go test ./internal/github/... -race` and confirm green

## Dependencies
- US-3.1

## Size Estimate
M
