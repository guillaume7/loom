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

// ErrRuntimeLeaseNotFound indicates no runtime lease exists for the requested
// lease key.
var ErrRuntimeLeaseNotFound = errors.New("runtime lease not found")

// Checkpoint is a point-in-time snapshot of the Loom workflow that is written
// to durable storage after every state transition.
type Checkpoint struct {
	// StoryID identifies the user story this checkpoint belongs to.
	// The empty string keeps v1 sequential behavior.
	StoryID string
	// State is the serialised FSM state at the time of the checkpoint.
	State string
	// ResumeState is the workflow state to restore when a PAUSED checkpoint is
	// resumed. It is empty for all non-paused checkpoints.
	ResumeState string
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

// WakeSchedule records a durable runtime wake-up request alongside the
// authoritative checkpoint.
type WakeSchedule struct {
	ID        int64
	SessionID string
	WakeKind  string
	DueAt     time.Time
	DedupeKey string
	Payload   string
	ClaimedAt time.Time
	CreatedAt time.Time
}

// ExternalEvent records a GitHub or timer observation for later replay and
// audit correlation.
type ExternalEvent struct {
	ID            int64
	SessionID     string
	EventSource   string
	EventKind     string
	ExternalID    string
	CorrelationID string
	Payload       string
	ObservedAt    time.Time
}

// RuntimeLease records active ownership of a runtime scope such as a run or PR.
type RuntimeLease struct {
	LeaseKey  string
	HolderID  string
	Scope     string
	ExpiresAt time.Time
	CreatedAt time.Time
	RenewedAt time.Time
}

// PolicyDecision records a persisted deterministic runtime outcome.
type PolicyDecision struct {
	ID            int64
	SessionID     string
	CorrelationID string
	DecisionKind  string
	Verdict       string
	InputHash     string
	Detail        string
	CreatedAt     time.Time
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

	// UpsertWakeSchedule persists or replaces a wake-up record using its
	// deduplication key.
	UpsertWakeSchedule(ctx context.Context, wake WakeSchedule) error

	// ReadWakeSchedules returns wake-up records ordered by due time then id.
	ReadWakeSchedules(ctx context.Context, sessionID string, limit int) ([]WakeSchedule, error)

	// WriteExternalEvent appends a durable observation used for replay or audit.
	WriteExternalEvent(ctx context.Context, event ExternalEvent) error

	// ReadExternalEvents returns the most recent observations ordered newest
	// first. A limit of zero returns an empty, non-nil slice.
	ReadExternalEvents(ctx context.Context, sessionID string, limit int) ([]ExternalEvent, error)

	// UpsertRuntimeLease persists or replaces the current lease for a key.
	UpsertRuntimeLease(ctx context.Context, lease RuntimeLease) error

	// ReadRuntimeLease returns the persisted lease for a single key.
	ReadRuntimeLease(ctx context.Context, leaseKey string) (RuntimeLease, error)

	// WritePolicyDecision appends a persisted runtime decision record.
	WritePolicyDecision(ctx context.Context, decision PolicyDecision) error

	// ReadPolicyDecisions returns the most recent decision records ordered newest
	// first. A limit of zero returns an empty, non-nil slice.
	ReadPolicyDecisions(ctx context.Context, sessionID string, limit int) ([]PolicyDecision, error)

	// DeleteAll removes all persisted workflow state from the store, including
	// checkpoints and action log entries.
	DeleteAll(ctx context.Context) error

	// Close releases any resources held by the store (e.g. the database
	// connection). Callers must call Close when they are done with the store.
	Close() error
}
