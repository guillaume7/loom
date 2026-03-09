// Package store defines the persistence layer for Loom workflow checkpoints.
//
// Implementations must be safe for concurrent use and must survive process
// restarts (i.e. write to durable storage).
package store

import (
	"context"
	"time"
)

// Checkpoint is a point-in-time snapshot of the Loom workflow that is written
// to durable storage after every state transition.
type Checkpoint struct {
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

// Store persists and retrieves Loom workflow checkpoints.
type Store interface {
	// ReadCheckpoint returns the most recent persisted Checkpoint.
	// It returns an empty Checkpoint (not an error) when no checkpoint exists yet.
	ReadCheckpoint(ctx context.Context) (Checkpoint, error)

	// WriteCheckpoint persists cp, overwriting any existing checkpoint.
	WriteCheckpoint(ctx context.Context, cp Checkpoint) error

	// DeleteAll removes all persisted checkpoints from the store.
	DeleteAll(ctx context.Context) error

	// Close releases any resources held by the store (e.g. the database
	// connection). Callers must call Close when they are done with the store.
	Close() error
}
