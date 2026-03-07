# Skill: Go Standards

Coding standards for all Go code in the Loom project.

---

## Module Setup

```
module github.com/guillaume7/loom

go 1.22
```

Use the latest stable Go toolchain. Verify with `go version`.

---

## Language Conventions

### Error Handling

```go
// ✔ Wrap errors with context
if err := store.Write(ctx, cp); err != nil {
    return fmt.Errorf("writing checkpoint at state %s: %w", state, err)
}

// ✘ Swallow errors
store.Write(ctx, cp) // never do this

// ✘ Panic in library code
func Transition(e Event) State {
    panic("unknown event") // never do this in internal/*
}

// ✔ Return sentinel errors
var ErrInvalidTransition = errors.New("invalid state transition")
```

### Interfaces for Testability

```go
// ✔ Define minimal interfaces at the point of use
type Store interface {
    ReadCheckpoint(ctx context.Context) (Checkpoint, error)
    WriteCheckpoint(ctx context.Context, cp Checkpoint) error
}

// ✔ Inject through constructor
func NewMCPServer(fsm *fsm.Machine, gh GitHubClient, store Store) *Server {
    return &Server{fsm: fsm, gh: gh, store: store}
}
```

### Context

```go
// ✔ Pass context as first argument to every function that does I/O
func (c *Client) CreateIssue(ctx context.Context, req CreateIssueRequest) (IssueNumber, error)

// ✘ Store context in struct
type Client struct { ctx context.Context } // never do this
```

### Naming Conventions

| Thing | Convention | Example |
|---|---|---|
| Files | `snake_case.go` | `machine.go`, `client_test.go` |
| Types / Interfaces | `PascalCase` | `GitHubClient`, `Checkpoint` |
| Variables / Functions | `camelCase` | `createIssue`, `retryBudget` |
| Constants | `PascalCase` or `UPPER_SNAKE` for sentinel values | `StateIdle`, `DefaultMaxRetries` |
| Unexported constants | `camelCase` | `defaultPollInterval` |
| Receiver names | Short, consistent | `m *Machine`, `c *Client` |
| Error variables | `ErrXxx` | `ErrInvalidTransition` |
| Test helpers | `must...` or `fake...` | `mustNewMachine()`, `fakeGitHub()` |

### Avoid

```go
// ✘ init() with side effects
func init() { db.Connect() }

// ✘ Package-level mutable variables
var currentState = StateIdle // use Machine struct instead

// ✘ Unused blank identifier assignments in production code
_ = doSomething() // document why if intentional

// ✘ time.Sleep in tests — use fake clocks
time.Sleep(100 * time.Millisecond)
```

---

## Struct Design

```go
// ✔ Use constructor functions, not struct literals for complex types
type Machine struct {
    state   State
    retries map[State]int
    config  Config
}

func New(cfg Config) *Machine {
    return &Machine{
        state:   StateIdle,
        retries: make(map[State]int),
        config:  cfg,
    }
}
```

---

## Testing

### Table-Driven Tests

```go
func TestTransition(t *testing.T) {
    tests := []struct {
        name      string
        from      State
        event     Event
        wantState State
        wantErr   bool
    }{
        {"idle start → scanning", StateIdle, EventStart, StateScanning, false},
        {"idle invalid → error", StateIdle, EventCIGreen, StateIdle, true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            m := fsm.New(fsm.DefaultConfig())
            m.ForceState(tt.from) // exported test helper
            got, err := m.Transition(tt.event)
            if tt.wantErr {
                require.Error(t, err)
                assert.Equal(t, tt.from, m.State()) // state unchanged on error
            } else {
                require.NoError(t, err)
                assert.Equal(t, tt.wantState, got)
            }
        })
    }
}
```

### HTTP Fixtures

```go
func TestCreateIssue(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        assert.Equal(t, "/repos/owner/repo/issues", r.URL.Path)
        w.WriteHeader(http.StatusCreated)
        json.NewEncoder(w).Encode(map[string]any{"number": 42})
    }))
    defer srv.Close()
    // inject httptest server URL into client
}
```

### No Real I/O in Unit Tests

- No real GitHub API calls in unit tests
- No real SQLite file in unit tests — use `":memory:"`
- No real `time.Sleep` — inject a `Clock` interface

---

## Linting

Use `golangci-lint` with at minimum:

```yaml
# .golangci.yml
linters:
  enable:
    - errcheck
    - govet
    - staticcheck
    - godot
    - gofmt
    - revive
    - exhaustive    # exhaustive switch on State/Event enums
```

Run: `golangci-lint run ./...`

---

## Logging

Use `log/slog` (stdlib, Go 1.21+):

```go
// Structured logging throughout
slog.Info("state transition",
    "from", prev,
    "event", event,
    "to", next,
    "phase", phase,
)

slog.Warn("retry budget low",
    "state", state,
    "retries", retries,
    "max", maxRetries,
)

slog.Error("GitHub API error",
    "op", "CreateIssue",
    "err", err,
)
```

Default log format: JSON (`slog.NewJSONHandler(os.Stderr, nil)`).
