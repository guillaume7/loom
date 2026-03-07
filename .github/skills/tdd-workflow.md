# Skill: TDD Workflow (Go)

Test-Driven Development workflow for the Loom project. All agents implementing code must follow this process.

---

## The Red–Green–Refactor Loop

```
1. RED    — Write a failing test that describes the desired behaviour
2. GREEN  — Write the minimum code to make the test pass
3. REFACTOR — Improve the code without breaking the test
```

Never write implementation code before you have a failing test.

---

## Step-by-Step

### 1. Read the Acceptance Criteria

Before writing any test, read the user story's acceptance criteria in full.
Each criterion maps to one or more tests.

### 2. Write a Failing Test (RED)

```go
// Example: internal/fsm — before implementation exists
func TestMachine_TransitionScanningToIssueCreated(t *testing.T) {
    m := fsm.New(fsm.DefaultConfig())
    m.ForceState(fsm.StateScanning)
    got, err := m.Transition(fsm.EventPhaseIdentified)
    require.NoError(t, err)
    assert.Equal(t, fsm.StateIssueCreated, got)
}
```

Run: `go test ./internal/fsm/... -run TestMachine_TransitionScanningToIssueCreated -v`

It **must fail** (red).

### 3. Implement (GREEN)

Write the **minimum code** necessary to pass the test. Do not generalise yet.

### 4. Refactor

Once green: improve naming, extract helpers, remove duplication. Run again — must still pass.

### 5. Repeat

Add the next test for the next acceptance criterion.

---

## Test Naming

```go
// Format: Test{Type}_{Behaviour}
func TestMachine_TransitionsToPausedOnRetryExhaustion(t *testing.T)
func TestClient_CreateIssue_ReturnsIssueNumber(t *testing.T)
func TestClient_CreateIssue_ReturnsError_OnHTTP500(t *testing.T)
func TestStore_ReadCheckpoint_ReturnsZeroValue_WhenEmpty(t *testing.T)
```

For table-driven tests, the `name` field is the sub-test name:

```go
t.Run("scanning: phase identified → issue created", func(t *testing.T) { … })
```

---

## Test Structure (AAA)

```go
func TestMachine_RetryBudget(t *testing.T) {
    // Arrange
    m := fsm.New(fsm.Config{MaxRetries: map[fsm.State]int{fsm.StateAwaitingPR: 2}})
    m.ForceState(fsm.StateAwaitingPR)

    // Act: exhaust budget
    m.Transition(fsm.EventTimeout) // retry 1
    m.Transition(fsm.EventTimeout) // retry 2 — should trigger PAUSED

    // Assert
    assert.Equal(t, fsm.StatePaused, m.State())
}
```

---

## Go-Specific Test Patterns

### Fake Clock

```go
type Clock interface {
    Now() time.Time
    Sleep(d time.Duration)
    After(d time.Duration) <-chan time.Time
}

// In tests:
type fakeClock struct { t time.Time }
func (c *fakeClock) Now() time.Time           { return c.t }
func (c *fakeClock) Sleep(d time.Duration)    { c.t = c.t.Add(d) }
func (c *fakeClock) After(d time.Duration) <-chan time.Time {
    ch := make(chan time.Time, 1)
    ch <- c.t.Add(d)
    return ch
}
```

### In-Memory Store

```go
type memStore struct{ cp store.Checkpoint }
func (s *memStore) ReadCheckpoint(_ context.Context) (store.Checkpoint, error) {
    return s.cp, nil
}
func (s *memStore) WriteCheckpoint(_ context.Context, cp store.Checkpoint) error {
    s.cp = cp
    return nil
}
```

### Mock GitHub Client

```go
type mockGitHub struct {
    createIssueResp IssueNumber
    createIssueErr  error
}
func (m *mockGitHub) CreateIssue(_ context.Context, _ CreateIssueRequest) (IssueNumber, error) {
    return m.createIssueResp, m.createIssueErr
}
```

---

## Running Tests

```bash
# Run all tests once
go test ./...

# With race detector
go test ./... -race

# With coverage
go test ./... -race -coverprofile=coverage.out
go tool cover -func=coverage.out

# Run specific package
go test ./internal/fsm/... -v

# Run specific test
go test ./internal/fsm/... -run TestMachine_Transitions -v
```

---

## Regression Tests

When a bug is found:

1. Write a test that **fails** without the fix: `// REGRESSION: <description> (issue #N)`
2. Commit: `test: regression for <description>`
3. Fix the code
4. Confirm regression test now passes
5. Confirm no other tests regressed
