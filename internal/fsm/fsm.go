// Package fsm implements the Loom finite-state machine.
//
// States are string constants so they are human-readable in logs and
// serialised checkpoints. The machine has zero external dependencies and is
// fully unit-testable in isolation.
package fsm

// State represents a node in the Loom workflow graph.
type State string

// Event represents a trigger that can cause the machine to move between states.
type Event string

// Well-known workflow states.
const (
	StateIdle     State = "IDLE"
	StateScanning State = "SCANNING"
	StateComplete State = "COMPLETE"
)
