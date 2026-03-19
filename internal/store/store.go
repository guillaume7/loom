// Package store defines the persistence layer for Loom workflow checkpoints.
//
// Implementations must be safe for concurrent use and must survive process
// restarts (i.e. write to durable storage).
package store

import (
	"context"
	"errors"
	"time"
)

// ErrDuplicateOperationKey indicates an action write attempted to reuse an
// existing idempotency key.
var ErrDuplicateOperationKey = errors.New("duplicate operation key")

// ErrActionNotFound indicates no action exists for the requested operation key.
var ErrActionNotFound = errors.New("action not found")

// Checkpoint is a point-in-time snapshot of the Loom workflow that is written
// to durable storage after every state transition.
type Checkpoint struct {
	// StoryID identifies the user story this checkpoint belongs to.
	// The empty string keeps v1 sequential behavior.
	StoryID string
	// State is the serialised FSM state at the time of the checkpoint.
	State string
	// Phase is the current epic phase number being processed.
	Phase int
	// PRNumber is the GitHub pull-request number associated with the current
	// phase, or zero if no PR has been opened yet.
	PRNumber int
	// IssueNumber is the GitHub issue number associated with the current
	// phase, or zero if no issue has been created yet.
	IssueNumber int
	// RetryCount is the number of times the current step has been retried.
	RetryCount int
	// UpdatedAt is the wall-clock time at which the checkpoint was last written.
	// WriteCheckpoint sets this to time.Now() when the caller leaves it zero.
	UpdatedAt time.Time
}

// Action is a single append-only action log entry.
type Action struct {
	ID           int64
	SessionID    string
	OperationKey string
	StateBefore  string
	StateAfter   string
	Event        string
	Detail       string
	CreatedAt    time.Time
}

// SessionTrace records metadata about a single run-loom session.
type SessionTrace struct {
	// SessionID is the MCP session identifier that uniquely names this trace.
	SessionID string
	// LoomVer is the Loom binary version string (e.g. "0.1.0" or "dev").
	LoomVer string
	// Repository is the GitHub target in "owner/repo" form.
	Repository string
	// StartedAt is the wall-clock time the session was opened.
	StartedAt time.Time
	// EndedAt is the wall-clock time the session was closed. Zero when still running.
	EndedAt time.Time
	// Outcome is the terminal disposition: "in_progress", "complete", "paused",
	// "aborted", or "stalled". Empty until the trace is closed.
	Outcome string
}

// TraceEvent is an append-only entry in a session's event ledger.
type TraceEvent struct {
	ID int64
	// SessionID links the event back to its SessionTrace.
	SessionID string
	// Seq is a monotonically increasing sequence number within the session.
	Seq int
	// Kind classifies the event: "transition", "intervention", "github", "system".
	Kind string
	// FromState is the FSM state before the transition (empty for non-FSM events).
	FromState string
	// ToState is the FSM state after the transition (empty for non-FSM events).
	ToState string
	// Event is the FSM event name or intervention type.
	Event string
	// Reason is a human-readable description of why this event occurred.
	Reason string
	// PRNumber is the associated pull-request number, or zero.
	PRNumber int
	// IssueNumber is the associated issue number, or zero.
	IssueNumber int
	// CreatedAt is the wall-clock time of the event.
	CreatedAt time.Time
}

// Store persists and retrieves Loom workflow checkpoints.
type Store interface {
	// ReadCheckpoint returns the most recent persisted Checkpoint.
	// It returns an empty Checkpoint (not an error) when no checkpoint exists yet.
	ReadCheckpoint(ctx context.Context) (Checkpoint, error)

	// WriteCheckpoint persists cp, overwriting any existing checkpoint.
	WriteCheckpoint(ctx context.Context, cp Checkpoint) error

	// WriteAction appends an action log entry.
	WriteAction(ctx context.Context, action Action) error

	// WriteCheckpointAndAction atomically persists a checkpoint update and
	// appends an action log entry in a single transaction. It returns
	// ErrDuplicateOperationKey if action.OperationKey already exists in the log.
	WriteCheckpointAndAction(ctx context.Context, cp Checkpoint, action Action) error

	// ReadActionByOperationKey returns the action log entry for a single
	// idempotency key.
	ReadActionByOperationKey(ctx context.Context, operationKey string) (Action, error)

	// ReadActions returns the most recent action log entries, ordered newest
	// first. A limit of zero returns an empty, non-nil slice.
	ReadActions(ctx context.Context, limit int) ([]Action, error)

	// DeleteAll removes all persisted workflow state from the store, including
	// checkpoints and action log entries.
	DeleteAll(ctx context.Context) error

	// Close releases any resources held by the store (e.g. the database
	// connection). Callers must call Close when they are done with the store.
	Close() error

	// ── Session trace (E9) ────────────────────────────────────────────────

	// OpenSessionTrace initialises a new session trace record. Calling
	// OpenSessionTrace twice with the same SessionID is a no-op (idempotent).
	OpenSessionTrace(ctx context.Context, trace SessionTrace) error

	// AppendTraceEvent appends an event to an existing session trace. Seq is
	// assigned by the store implementation; the caller's Seq field is ignored.
	AppendTraceEvent(ctx context.Context, ev TraceEvent) error

	// CloseSessionTrace sets the EndedAt timestamp and Outcome for the given
	// session. It is idempotent: re-closing a trace with the same outcome is
	// silently accepted.
	CloseSessionTrace(ctx context.Context, sessionID, outcome string) error

	// ReadSessionTrace returns the trace header and all events for sessionID.
	// It returns an empty SessionTrace and a nil slice when sessionID is unknown.
	ReadSessionTrace(ctx context.Context, sessionID string) (SessionTrace, []TraceEvent, error)

	// ListSessionTraces returns trace headers ordered newest-first. A limit of
	// zero or less returns an empty, non-nil slice.
	ListSessionTraces(ctx context.Context, limit int) ([]SessionTrace, error)
}
