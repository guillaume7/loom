# US-3.7 — `httptest`-based tests for all operations using recorded fixtures

## Epic
E3: GitHub Client

## Goal
Provide a complete, self-contained test suite using `httptest.NewServer` and JSON fixture files so the full `GitHubClient` implementation can be exercised with no live GitHub token.

## Acceptance Criteria

```
Given all fixture JSON files are present under `internal/github/testdata/`
When `go test ./internal/github/... -race` is executed without any GitHub credentials
Then all tests pass
  And no outbound HTTP connections to `api.github.com` are made
```

```
Given each operation has a success fixture and an error fixture
When tests run
Then every public method on `Client` has at least one success test and one error test
```

```
Given the `-race` flag is set
When tests run concurrently
Then no data races are detected
```

## Tasks

1. [ ] Create `internal/github/testdata/` with JSON fixture files for every API endpoint (write fixtures first)
2. [ ] Write a shared `newTestClient(t, handler)` helper that starts an `httptest.Server` and returns a configured `Client`
3. [ ] Write success and error tests for each of: CreateIssue, AddComment, CloseIssue, ListPRs, GetPR, GetCheckRuns, MergePR, RequestReview, GetReviewStatus, CreateTag, CreateRelease
4. [ ] Write rate-limit tests using the helper
5. [ ] Verify no import of `api.github.com` in test files
6. [ ] Run `go test ./internal/github/... -race -count=1` and confirm all pass

## Dependencies
- US-3.2
- US-3.3
- US-3.4
- US-3.5
- US-3.6

## Size Estimate
M
