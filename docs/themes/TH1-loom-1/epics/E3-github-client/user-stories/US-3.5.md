# US-3.5 — Implement tag/release creation

## Epic
E3: GitHub Client

## Assigned Agent

**[Backend Developer](../../../../.github/agents/backend-developer.md)** — apply [`loom-architecture`](../../../../.github/skills/loom-architecture.md) · [`go-standards`](../../../../.github/skills/go-standards.md) · [`tdd-workflow`](../../../../.github/skills/tdd-workflow.md).


## Goal
Provide working implementations of `CreateTag` and `CreateRelease` so Loom can publish the final release when all phases are complete.

## Acceptance Criteria

```
Given an `httptest.Server` returning 201 for `POST /repos/{owner}/{repo}/git/refs`
When `client.CreateTag(ctx, tagName, sha)` is called
Then no error is returned
  And the request body contains `"ref": "refs/tags/{tagName}"` and `"sha": "{sha}"`
```

```
Given an `httptest.Server` returning 201 for `POST /repos/{owner}/{repo}/releases`
When `client.CreateRelease(ctx, tagName, name, body)` is called
Then the returned `Release.ID` matches the fixture
  And no error is returned
```

```
Given an `httptest.Server` returning 422 (tag already exists)
When `client.CreateTag(ctx, tagName, sha)` is called
Then a non-nil error is returned with context `"creating tag: ..."`
```

## Tasks

1. [ ] Write `client_release_test.go` with httptest tests for CreateTag and CreateRelease (write tests first)
2. [ ] Implement `CreateTag` using `POST /repos/{owner}/{repo}/git/refs`
3. [ ] Implement `CreateRelease` using `POST /repos/{owner}/{repo}/releases`
4. [ ] Wrap all errors with context
5. [ ] Run `go test ./internal/github/... -race` and confirm green

## Dependencies
- US-3.1

## Size Estimate
S
