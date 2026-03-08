// Package store defines the persistence layer for Loom workflow checkpoints.
//
// Implementations must be safe for concurrent use and must survive process
// restarts (i.e. write to durable storage).
package store

import "context"

// Checkpoint is a point-in-time snapshot of the Loom workflow that is written
// to durable storage after every state transition.
type Checkpoint struct {
	// State is the serialised FSM state at the time of the checkpoint.
	State string
	// Phase is the current epic phase number being processed.
	Phase int
}

// Store persists and retrieves Loom workflow checkpoints.
type Store interface {
	// ReadCheckpoint returns the most recent persisted Checkpoint.
	// It returns an empty Checkpoint (not an error) when no checkpoint exists yet.
	ReadCheckpoint(ctx context.Context) (Checkpoint, error)

	// WriteCheckpoint persists cp, overwriting any existing checkpoint.
	WriteCheckpoint(ctx context.Context, cp Checkpoint) error
}
