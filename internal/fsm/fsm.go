// Package fsm implements the Loom finite-state machine.
//
// States and events are string constants so they are human-readable in logs
// and serialised checkpoints. The machine has zero external dependencies and
// is fully unit-testable in isolation.
//
// Usage:
//
//	m := fsm.NewMachine(fsm.DefaultConfig())
//	state, err := m.Transition(fsm.EventStart)
package fsm

import "fmt"

// State represents a node in the Loom workflow graph.
type State string

// All 13 well-known workflow states.
const (
	StateIdle               State = "IDLE"
	StateScanning           State = "SCANNING"
	StateIssueCreated       State = "ISSUE_CREATED"
	StateAwaitingPR         State = "AWAITING_PR"
	StateAwaitingReady      State = "AWAITING_READY"
	StateAwaitingCI         State = "AWAITING_CI"
	StateReviewing          State = "REVIEWING"
	StateDebugging          State = "DEBUGGING"
	StateAddressingFeedback State = "ADDRESSING_FEEDBACK"
	StateMerging            State = "MERGING"
	StateRefactoring        State = "REFACTORING"
	StateComplete           State = "COMPLETE"
	StatePaused             State = "PAUSED"
)

// Event represents a trigger that can cause the machine to move between states.
type Event string

// All well-known events.
const (
	EventStart                  Event = "start"
	EventPhaseIdentified        Event = "phase_identified"
	EventAllPhasesDone          Event = "all_phases_done"
	EventCopilotAssigned        Event = "copilot_assigned"
	EventPROpened               Event = "pr_opened"
	EventTimeout                Event = "timeout"
	EventPRReady                Event = "pr_ready"
	EventCIGreen                Event = "ci_green"
	EventCIRed                  Event = "ci_red"
	EventReviewApproved         Event = "review_approved"
	EventReviewChangesRequested Event = "review_changes_requested"
	EventFixPushed              Event = "fix_pushed"
	EventFeedbackAddressed      Event = "feedback_addressed"
	EventMerged                 Event = "merged"
	EventMergedEpicBoundary     Event = "merged_epic_boundary"
	EventRefactorMerged         Event = "refactor_merged"
	EventSkipStory              Event = "skip_story"
	EventReassign               Event = "reassign"
	EventAbort                  Event = "abort"
)

// Config holds retry budgets for each gate state.
// Use DefaultConfig to obtain production-spec defaults.
type Config struct {
	// MaxRetriesAwaitingPR is the number of timeout events tolerated in state
	// AWAITING_PR before the machine transitions to PAUSED.
	MaxRetriesAwaitingPR int

	// MaxRetriesAwaitingReady is the number of timeout events tolerated in
	// AWAITING_READY before the draft PR is force-promoted and the machine
	// transitions to AWAITING_CI.
	MaxRetriesAwaitingReady int

	// MaxRetriesAwaitingCI is the number of timeout events tolerated in
	// AWAITING_CI before the machine transitions to PAUSED.
	MaxRetriesAwaitingCI int

	// MaxDebugCycles is the maximum number of full AWAITING_CI→DEBUGGING
	// cycles (triggered by ci_red) before the machine transitions to PAUSED.
	MaxDebugCycles int

	// MaxFeedbackCycles is the maximum number of full
	// REVIEWING→ADDRESSING_FEEDBACK cycles (triggered by
	// review_changes_requested) before the machine transitions to PAUSED.
	MaxFeedbackCycles int
}

// DefaultConfig returns a Config with production-spec retry budgets as defined
// in docs/loom/analysis.md § 5.3.
func DefaultConfig() Config {
	return Config{
		MaxRetriesAwaitingPR:    20,
		MaxRetriesAwaitingReady: 60,
		MaxRetriesAwaitingCI:    20,
		MaxDebugCycles:          3,
		MaxFeedbackCycles:       5,
	}
}

// Machine is the Loom finite-state machine.
//
// Create with NewMachine; interact only through Transition and State.
// All fields are unexported; there is no global mutable state.
type Machine struct {
	state State
	cfg   Config

	// per-gate timeout retry counters; reset whenever the gate state is
	// (re-)entered from a different state.
	awaitingPRRetries    int
	awaitingReadyRetries int
	awaitingCIRetries    int

	// cross-state loop-cycle counters; reset when the machine returns to
	// SCANNING (end of one workflow pass).
	debugCycles    int
	feedbackCycles int
}

// NewMachine constructs a Machine starting in StateIdle with the given Config.
func NewMachine(cfg Config) *Machine {
	return &Machine{
		state: StateIdle,
		cfg:   cfg,
	}
}

// State returns the current state of the machine.
func (m *Machine) State() State {
	return m.state
}

// invalidTransition returns a descriptive error for an event that is not valid
// in the machine's current state.
func (m *Machine) invalidTransition(event Event) error {
	return fmt.Errorf("invalid transition: no transition from state %q on event %q", m.state, event)
}

// Transition fires event on the machine, mutates state, and returns the new
// state. It returns a descriptive error when the event is not valid in the
// current state. Transition never panics.
//
// The abort event is universally accepted from every state and always
// transitions the machine to PAUSED.
func (m *Machine) Transition(event Event) (State, error) {
	// abort is a global escape hatch valid from every state.
	if event == EventAbort {
		m.state = StatePaused
		return m.state, nil
	}

	var newState State

	switch m.state {

	case StateIdle:
		switch event {
		case EventStart:
			newState = StateScanning
		default:
			return m.state, m.invalidTransition(event)
		}

	case StateScanning:
		switch event {
		case EventPhaseIdentified:
			newState = StateIssueCreated
		case EventAllPhasesDone:
			newState = StateComplete
		default:
			return m.state, m.invalidTransition(event)
		}

	case StateIssueCreated:
		switch event {
		case EventCopilotAssigned:
			newState = StateAwaitingPR
		default:
			return m.state, m.invalidTransition(event)
		}

	case StateAwaitingPR:
		switch event {
		case EventPROpened:
			newState = StateAwaitingReady
		case EventReassign:
			newState = StateIssueCreated
		case EventSkipStory:
			newState = StateScanning
		case EventTimeout:
			m.awaitingPRRetries++
			if m.awaitingPRRetries > m.cfg.MaxRetriesAwaitingPR {
				newState = StatePaused
			} else {
				newState = StateAwaitingPR
			}
		default:
			return m.state, m.invalidTransition(event)
		}

	case StateAwaitingReady:
		switch event {
		case EventPRReady:
			newState = StateAwaitingCI
		case EventTimeout:
			m.awaitingReadyRetries++
			if m.awaitingReadyRetries > m.cfg.MaxRetriesAwaitingReady {
				// Budget exhausted: force-promote the draft PR and proceed.
				newState = StateAwaitingCI
			} else {
				newState = StateAwaitingReady
			}
		default:
			return m.state, m.invalidTransition(event)
		}

	case StateAwaitingCI:
		switch event {
		case EventCIGreen:
			newState = StateReviewing
		case EventReassign:
			newState = StateIssueCreated
		case EventCIRed:
			m.debugCycles++
			if m.debugCycles > m.cfg.MaxDebugCycles {
				newState = StatePaused
			} else {
				newState = StateDebugging
			}
		case EventSkipStory:
			newState = StateScanning
		case EventTimeout:
			m.awaitingCIRetries++
			if m.awaitingCIRetries > m.cfg.MaxRetriesAwaitingCI {
				newState = StatePaused
			} else {
				newState = StateAwaitingCI
			}
		default:
			return m.state, m.invalidTransition(event)
		}

	case StateReviewing:
		switch event {
		case EventReviewApproved:
			newState = StateMerging
		case EventReassign:
			newState = StateIssueCreated
		case EventSkipStory:
			newState = StateScanning
		case EventReviewChangesRequested:
			m.feedbackCycles++
			if m.feedbackCycles > m.cfg.MaxFeedbackCycles {
				newState = StatePaused
			} else {
				newState = StateAddressingFeedback
			}
		default:
			return m.state, m.invalidTransition(event)
		}

	case StateDebugging:
		switch event {
		case EventFixPushed:
			newState = StateAwaitingCI
		case EventReassign:
			newState = StateIssueCreated
		case EventSkipStory:
			newState = StateScanning
		default:
			return m.state, m.invalidTransition(event)
		}

	case StateAddressingFeedback:
		switch event {
		case EventFeedbackAddressed:
			newState = StateAwaitingCI
		case EventReassign:
			newState = StateIssueCreated
		default:
			return m.state, m.invalidTransition(event)
		}

	case StateMerging:
		switch event {
		case EventMerged:
			newState = StateScanning
		case EventMergedEpicBoundary:
			newState = StateRefactoring
		default:
			return m.state, m.invalidTransition(event)
		}

	case StateRefactoring:
		switch event {
		case EventRefactorMerged:
			newState = StateScanning
		default:
			return m.state, m.invalidTransition(event)
		}

	case StateComplete:
		return m.state, m.invalidTransition(event)

	case StatePaused:
		return m.state, m.invalidTransition(event)
	}

	// Reset per-state counters when entering a new state, so budgets are fresh
	// on re-entry (e.g. after a full workflow cycle or a debug detour).
	if newState != m.state {
		switch newState {
		case StateAwaitingPR:
			m.awaitingPRRetries = 0
		case StateAwaitingReady:
			m.awaitingReadyRetries = 0
		case StateAwaitingCI:
			m.awaitingCIRetries = 0
		case StateScanning:
			m.awaitingPRRetries = 0
			m.awaitingReadyRetries = 0
			m.awaitingCIRetries = 0
			m.debugCycles = 0
			m.feedbackCycles = 0
		}
	}

	m.state = newState
	return m.state, nil
}

// Snapshot captures all mutable state of a Machine so it can be fully
// restored by Rollback. It is an opaque value; callers must not inspect or
// modify its fields.
type Snapshot struct {
	state                State
	awaitingPRRetries    int
	awaitingReadyRetries int
	awaitingCIRetries    int
	debugCycles          int
	feedbackCycles       int
}

// TakeSnapshot returns an opaque snapshot of the machine's current mutable
// state (including all retry/cycle counters). Pass the returned value to
// Rollback if a subsequent operation needs to be undone.
func (m *Machine) TakeSnapshot() Snapshot {
	return Snapshot{
		state:                m.state,
		awaitingPRRetries:    m.awaitingPRRetries,
		awaitingReadyRetries: m.awaitingReadyRetries,
		awaitingCIRetries:    m.awaitingCIRetries,
		debugCycles:          m.debugCycles,
		feedbackCycles:       m.feedbackCycles,
	}
}

// Rollback restores the machine to the state captured by a prior TakeSnapshot
// call. It is intended to be called when a dependent operation (e.g. a store
// write) fails, so that a retry fires the same event from the correct starting
// state without leaving the machine permanently advanced.
func (m *Machine) Rollback(snap Snapshot) {
	m.state = snap.state
	m.awaitingPRRetries = snap.awaitingPRRetries
	m.awaitingReadyRetries = snap.awaitingReadyRetries
	m.awaitingCIRetries = snap.awaitingCIRetries
	m.debugCycles = snap.debugCycles
	m.feedbackCycles = snap.feedbackCycles
}
