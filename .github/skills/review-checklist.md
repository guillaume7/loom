# Skill: Code Review Checklist

A structured checklist for every PR in the Loom project.

---

## Automated Gates (must pass before review begins)

- [ ] `go vet ./...` exits 0
- [ ] `golangci-lint run ./...` exits 0
- [ ] `go test ./... -race -cover` exits 0 (all tests pass, thresholds met)
- [ ] `go build ./cmd/loom` exits 0

---

## Architecture

- [ ] `internal/fsm` has zero imports from `internal/github`, `internal/mcp`, or `internal/store`
- [ ] All cross-package dependencies use interfaces, not concrete types
- [ ] New external dependencies have an ADR entry
- [ ] Package APIs match the Architect's interface contracts
- [ ] No global mutable variables introduced

---

## FSM Correctness

- [ ] Every new state has tests for all valid in-bound transitions
- [ ] Every new state has a test for an invalid transition attempt
- [ ] Retry budget is enforced — state transitions to PAUSED on exhaustion
- [ ] PAUSED state is always reachable from every gate-timeout scenario
- [ ] State is never mutated outside `Machine.Transition()`

---

## Tests

- [ ] Every new code branch has at least one test covering it
- [ ] Table-driven tests used where input/output can be parameterised
- [ ] Test names are descriptive: `TestMachine_TransitionsToPausedOnRetryExhaustion`
- [ ] No `time.Sleep` in tests — fake clocks used
- [ ] No real GitHub API calls in unit tests — `httptest` used
- [ ] No real SQLite file in unit tests — `":memory:"` or in-memory mock
- [ ] Regression tests reference the issue they prevent: `// REGRESSION: ...`

---

## Code Quality

- [ ] No `panic` in `internal/*` — errors returned
- [ ] No swallowed errors: `_ = someFunc()` must have a comment if intentional
- [ ] Errors wrapped with context: `fmt.Errorf("doing X: %w", err)`
- [ ] No magic numbers — named constants in `config.go`
- [ ] No functions longer than ~50 lines
- [ ] No commented-out code
- [ ] No `TODO` comments not tracked as issues
- [ ] `context.Context` is first argument for all I/O functions

---

## MCP Tool Surface

- [ ] All tool input schemas are defined as typed structs (not `map[string]any`)
- [ ] All tool outputs are JSON-serialisable structs
- [ ] Every tool call is logged with `slog.Info`
- [ ] Tool handlers respect `context.Context` cancellation

---

## Logging

- [ ] New significant operations logged with `slog`
- [ ] Log entries use structured key-value pairs (not fmt.Sprintf messages)
- [ ] No sensitive data (tokens, passwords) in log output

---

## Feedback Labels

| Label | Meaning | Blocks merge? |
|---|---|---|
| `BLOCKER` | Must fix before merge | Yes |
| `SUGGESTION` | Improvement; author decides | No |
| `QUESTION` | Reviewer needs clarification | Pauses review |
| `NIT` | Trivial style preference | No |

---

## Approval

Approve when **all** of:

- All automated gates pass
- All acceptance criteria from the linked user story are implemented and tested
- No BLOCKER comments remain unresolved
- Code is readable without needing to ask the author
