# US-3.1 — Define `GitHubClient` interface covering all required operations

## Epic
E3: GitHub Client

## Goal
Declare the `GitHubClient` interface in `internal/github/client.go` and all supporting types in `internal/github/types.go` so the rest of the codebase can depend on the interface without a concrete implementation.

## Acceptance Criteria

```
Given `internal/github/client.go` is compiled
When the package is imported
Then the `GitHubClient` interface is exported and contains methods for: CreateIssue, AddComment, CloseIssue, ListPRs, GetPR, GetCheckRuns, MergePR, RequestReview, GetReviewStatus, CreateTag, CreateRelease
```

```
Given a mock struct that implements `GitHubClient`
When it is assigned to a `GitHubClient`-typed variable
Then the compiler accepts the assignment without error
```

```
Given `internal/github/types.go`
When compiled
Then exported types `Issue`, `PR`, `CheckRun`, `Review`, `Tag`, `Release` are defined with all fields required by the FSM gate checks
```

## Tasks

1. [ ] Write `client_interface_test.go` with a compile-time interface-satisfaction check using a blank `_` assignment (write test first)
2. [ ] Define all domain types in `internal/github/types.go`
3. [ ] Define `GitHubClient` interface in `internal/github/client.go` with all 11 method signatures
4. [ ] Run `go build ./internal/github/...` and confirm it exits 0

## Dependencies
- US-1.2

## Size Estimate
S
