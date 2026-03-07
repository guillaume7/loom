## Summary

<!-- What was implemented in this PR and why. One paragraph max. -->

## Stories Closed

Closes #<!-- issue number -->

## Changes

<!-- List the key changed files and what changed in each. -->

- `internal/fsm/` — 
- `internal/github/` — 
- `internal/mcp/` — 
- `internal/store/` — 

## CI Results

All gates must be green before requesting review.

- [ ] `go vet ./...` ✅
- [ ] `golangci-lint run` ✅
- [ ] `go test ./... -race -cover` ✅ — coverage: __%
- [ ] `go build ./cmd/loom` ✅

## Architecture

<!-- For any interface or package boundary change: name the abstraction and why it is designed this way. -->

| Decision | Rationale |
|---|---|
|  |  |

## Loom-Specific Checklist

- [ ] No global mutable state introduced
- [ ] No `panic` in library code — errors returned
- [ ] Interfaces used for testability (GitHub client, store, clock)
- [ ] FSM state transitions covered by unit tests
- [ ] Retry budgets respected — no unbounded loops
- [ ] Checkpoint written before any blocking operation
- [ ] No undocumented GitHub API endpoints used
- [ ] `loom_heartbeat` called during all long-running waits

## Notes for Reviewer

<!-- Anything the reviewer should pay particular attention to, or known limitations. -->
