package fsm_test

import (
	"strings"
	"testing"

	"github.com/guillaume7/loom/internal/fsm"
	"github.com/stretchr/testify/assert"
)

// mustTransition fires a sequence of events and calls t.Fatal if any returns
// an error. Use only for test-setup code that is expected to succeed.
func mustTransition(t *testing.T, m *fsm.Machine, events ...fsm.Event) {
	t.Helper()
	for _, e := range events {
		if _, err := m.Transition(e); err != nil {
			t.Fatalf("setup transition failed for event %q: %v", e, err)
		}
	}
}

// ---------------------------------------------------------------------------
// String-value sanity checks
// ---------------------------------------------------------------------------

func TestState_StringValues(t *testing.T) {
	tests := []struct {
		state fsm.State
		want  string
	}{
		{fsm.StateIdle, "IDLE"},
		{fsm.StateScanning, "SCANNING"},
		{fsm.StateIssueCreated, "ISSUE_CREATED"},
		{fsm.StateAwaitingPR, "AWAITING_PR"},
		{fsm.StateAwaitingReady, "AWAITING_READY"},
		{fsm.StateAwaitingCI, "AWAITING_CI"},
		{fsm.StateReviewing, "REVIEWING"},
		{fsm.StateDebugging, "DEBUGGING"},
		{fsm.StateAddressingFeedback, "ADDRESSING_FEEDBACK"},
		{fsm.StateMerging, "MERGING"},
		{fsm.StateRefactoring, "REFACTORING"},
		{fsm.StateComplete, "COMPLETE"},
		{fsm.StatePaused, "PAUSED"},
	}
	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			assert.Equal(t, tt.want, string(tt.state))
		})
	}
}

func TestEvent_StringValues(t *testing.T) {
	tests := []struct {
		event fsm.Event
		want  string
	}{
		{fsm.EventStart, "start"},
		{fsm.EventPhaseIdentified, "phase_identified"},
		{fsm.EventAllPhasesDone, "all_phases_done"},
		{fsm.EventCopilotAssigned, "copilot_assigned"},
		{fsm.EventPROpened, "pr_opened"},
		{fsm.EventTimeout, "timeout"},
		{fsm.EventPRReady, "pr_ready"},
		{fsm.EventCIGreen, "ci_green"},
		{fsm.EventCIRed, "ci_red"},
		{fsm.EventReviewApproved, "review_approved"},
		{fsm.EventReviewChangesRequested, "review_changes_requested"},
		{fsm.EventFixPushed, "fix_pushed"},
		{fsm.EventFeedbackAddressed, "feedback_addressed"},
		{fsm.EventMerged, "merged"},
		{fsm.EventMergedEpicBoundary, "merged_epic_boundary"},
		{fsm.EventRefactorMerged, "refactor_merged"},
		{fsm.EventSkipStory, "skip_story"},
		{fsm.EventAbort, "abort"},
	}
	for _, tt := range tests {
		t.Run(string(tt.event), func(t *testing.T) {
			assert.Equal(t, tt.want, string(tt.event))
		})
	}
}

// ---------------------------------------------------------------------------
// DefaultConfig
// ---------------------------------------------------------------------------

func TestDefaultConfig(t *testing.T) {
	cfg := fsm.DefaultConfig()
	assert.Equal(t, 20, cfg.MaxRetriesAwaitingPR)
	assert.Equal(t, 60, cfg.MaxRetriesAwaitingReady)
	assert.Equal(t, 20, cfg.MaxRetriesAwaitingCI)
	assert.Equal(t, 3, cfg.MaxDebugCycles)
	assert.Equal(t, 5, cfg.MaxFeedbackCycles)
}

// ---------------------------------------------------------------------------
// NewMachine / State()
// ---------------------------------------------------------------------------

func TestNewMachine_InitialState(t *testing.T) {
	m := fsm.NewMachine(fsm.DefaultConfig())
	assert.Equal(t, fsm.StateIdle, m.State())
}

// ---------------------------------------------------------------------------
// Setup sequences used by multiple tests
// ---------------------------------------------------------------------------

var (
	setupScanning           = []fsm.Event{fsm.EventStart}
	setupIssueCreated       = []fsm.Event{fsm.EventStart, fsm.EventPhaseIdentified}
	setupAwaitingPR         = []fsm.Event{fsm.EventStart, fsm.EventPhaseIdentified, fsm.EventCopilotAssigned}
	setupAwaitingReady      = []fsm.Event{fsm.EventStart, fsm.EventPhaseIdentified, fsm.EventCopilotAssigned, fsm.EventPROpened}
	setupAwaitingCI         = []fsm.Event{fsm.EventStart, fsm.EventPhaseIdentified, fsm.EventCopilotAssigned, fsm.EventPROpened, fsm.EventPRReady}
	setupReviewing          = []fsm.Event{fsm.EventStart, fsm.EventPhaseIdentified, fsm.EventCopilotAssigned, fsm.EventPROpened, fsm.EventPRReady, fsm.EventCIGreen}
	setupDebugging          = []fsm.Event{fsm.EventStart, fsm.EventPhaseIdentified, fsm.EventCopilotAssigned, fsm.EventPROpened, fsm.EventPRReady, fsm.EventCIRed}
	setupAddressingFeedback = []fsm.Event{fsm.EventStart, fsm.EventPhaseIdentified, fsm.EventCopilotAssigned, fsm.EventPROpened, fsm.EventPRReady, fsm.EventCIGreen, fsm.EventReviewChangesRequested}
	setupMerging            = []fsm.Event{fsm.EventStart, fsm.EventPhaseIdentified, fsm.EventCopilotAssigned, fsm.EventPROpened, fsm.EventPRReady, fsm.EventCIGreen, fsm.EventReviewApproved}
	setupRefactoring        = []fsm.Event{fsm.EventStart, fsm.EventPhaseIdentified, fsm.EventCopilotAssigned, fsm.EventPROpened, fsm.EventPRReady, fsm.EventCIGreen, fsm.EventReviewApproved, fsm.EventMergedEpicBoundary}
)

// ---------------------------------------------------------------------------
// Valid transitions — happy path
// ---------------------------------------------------------------------------

func TestMachine_ValidTransitions(t *testing.T) {
	tests := []struct {
		name  string
		setup []fsm.Event
		event fsm.Event
		want  fsm.State
	}{
		// IDLE
		{"IDLE+start→SCANNING", nil, fsm.EventStart, fsm.StateScanning},

		// SCANNING
		{"SCANNING+phase_identified→ISSUE_CREATED", setupScanning, fsm.EventPhaseIdentified, fsm.StateIssueCreated},
		{"SCANNING+all_phases_done→COMPLETE", setupScanning, fsm.EventAllPhasesDone, fsm.StateComplete},

		// ISSUE_CREATED
		{"ISSUE_CREATED+copilot_assigned→AWAITING_PR", setupIssueCreated, fsm.EventCopilotAssigned, fsm.StateAwaitingPR},

		// AWAITING_PR
		{"AWAITING_PR+pr_opened→AWAITING_READY", setupAwaitingPR, fsm.EventPROpened, fsm.StateAwaitingReady},
		{"AWAITING_PR+skip_story→SCANNING", setupAwaitingPR, fsm.EventSkipStory, fsm.StateScanning},

		// AWAITING_READY
		{"AWAITING_READY+pr_ready→AWAITING_CI", setupAwaitingReady, fsm.EventPRReady, fsm.StateAwaitingCI},

		// AWAITING_CI
		{"AWAITING_CI+ci_green→REVIEWING", setupAwaitingCI, fsm.EventCIGreen, fsm.StateReviewing},
		{"AWAITING_CI+ci_red→DEBUGGING", setupAwaitingCI, fsm.EventCIRed, fsm.StateDebugging},
		{"AWAITING_CI+skip_story→SCANNING", setupAwaitingCI, fsm.EventSkipStory, fsm.StateScanning},

		// REVIEWING
		{"REVIEWING+review_approved→MERGING", setupReviewing, fsm.EventReviewApproved, fsm.StateMerging},
		{"REVIEWING+review_changes_requested→ADDRESSING_FEEDBACK", setupReviewing, fsm.EventReviewChangesRequested, fsm.StateAddressingFeedback},
		{"REVIEWING+skip_story→SCANNING", setupReviewing, fsm.EventSkipStory, fsm.StateScanning},

		// DEBUGGING
		{"DEBUGGING+fix_pushed→AWAITING_CI", setupDebugging, fsm.EventFixPushed, fsm.StateAwaitingCI},
		{"DEBUGGING+skip_story→SCANNING", setupDebugging, fsm.EventSkipStory, fsm.StateScanning},

		// ADDRESSING_FEEDBACK
		{"ADDRESSING_FEEDBACK+feedback_addressed→AWAITING_CI", setupAddressingFeedback, fsm.EventFeedbackAddressed, fsm.StateAwaitingCI},

		// MERGING
		{"MERGING+merged→SCANNING", setupMerging, fsm.EventMerged, fsm.StateScanning},
		{"MERGING+merged_epic_boundary→REFACTORING", setupMerging, fsm.EventMergedEpicBoundary, fsm.StateRefactoring},

		// REFACTORING
		{"REFACTORING+refactor_merged→SCANNING", setupRefactoring, fsm.EventRefactorMerged, fsm.StateScanning},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := fsm.NewMachine(fsm.DefaultConfig())
			mustTransition(t, m, tt.setup...)

			got, err := m.Transition(tt.event)

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.want, m.State())
		})
	}
}

// ---------------------------------------------------------------------------
// Invalid transitions — one representative invalid event per state
// ---------------------------------------------------------------------------

func TestMachine_InvalidTransitions(t *testing.T) {
	tests := []struct {
		name          string
		setup         []fsm.Event
		event         fsm.Event
		wantState     fsm.State // state must NOT change on error
		wantErrSubstr string
	}{
		{"IDLE+phase_identified→error", nil, fsm.EventPhaseIdentified, fsm.StateIdle, "IDLE"},
		{"SCANNING+copilot_assigned→error", setupScanning, fsm.EventCopilotAssigned, fsm.StateScanning, "SCANNING"},
		{"ISSUE_CREATED+pr_opened→error", setupIssueCreated, fsm.EventPROpened, fsm.StateIssueCreated, "ISSUE_CREATED"},
		{"AWAITING_PR+ci_green→error", setupAwaitingPR, fsm.EventCIGreen, fsm.StateAwaitingPR, "AWAITING_PR"},
		{"AWAITING_READY+ci_green→error", setupAwaitingReady, fsm.EventCIGreen, fsm.StateAwaitingReady, "AWAITING_READY"},
		{"AWAITING_CI+pr_opened→error", setupAwaitingCI, fsm.EventPROpened, fsm.StateAwaitingCI, "AWAITING_CI"},
		{"REVIEWING+ci_green→error", setupReviewing, fsm.EventCIGreen, fsm.StateReviewing, "REVIEWING"},
		{"DEBUGGING+ci_green→error", setupDebugging, fsm.EventCIGreen, fsm.StateDebugging, "DEBUGGING"},
		{"MERGING+skip_story→error", setupMerging, fsm.EventSkipStory, fsm.StateMerging, "MERGING"},
		{"ADDRESSING_FEEDBACK+ci_green→error", setupAddressingFeedback, fsm.EventCIGreen, fsm.StateAddressingFeedback, "ADDRESSING_FEEDBACK"},
		{"MERGING+ci_green→error", setupMerging, fsm.EventCIGreen, fsm.StateMerging, "MERGING"},
		{"REFACTORING+ci_green→error", setupRefactoring, fsm.EventCIGreen, fsm.StateRefactoring, "REFACTORING"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := fsm.NewMachine(fsm.DefaultConfig())
			mustTransition(t, m, tt.setup...)

			got, err := m.Transition(tt.event)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErrSubstr)
			assert.Equal(t, tt.wantState, got)
			assert.Equal(t, tt.wantState, m.State())
		})
	}
}

// TestMachine_TerminalStates verifies that COMPLETE and PAUSED reject all
// non-abort events.
func TestMachine_TerminalStates_RejectNonAbortEvents(t *testing.T) {
	t.Run("COMPLETE+start→error", func(t *testing.T) {
		m := fsm.NewMachine(fsm.DefaultConfig())
		mustTransition(t, m, setupScanning...)
		mustTransition(t, m, fsm.EventAllPhasesDone) // → COMPLETE

		got, err := m.Transition(fsm.EventStart)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "COMPLETE")
		assert.Equal(t, fsm.StateComplete, got)
	})

	t.Run("PAUSED+start→error", func(t *testing.T) {
		m := fsm.NewMachine(fsm.DefaultConfig())
		mustTransition(t, m, fsm.EventAbort) // → PAUSED

		got, err := m.Transition(fsm.EventStart)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "PAUSED")
		assert.Equal(t, fsm.StatePaused, got)
	})
}

// ---------------------------------------------------------------------------
// Error message quality
// ---------------------------------------------------------------------------

func TestMachine_ErrorMessages_AreDescriptive(t *testing.T) {
	m := fsm.NewMachine(fsm.DefaultConfig())

	_, err := m.Transition(fsm.EventCIGreen)

	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "IDLE"), "error must mention current state")
	assert.True(t, strings.Contains(err.Error(), "ci_green"), "error must mention the event")
}

// ---------------------------------------------------------------------------
// Abort — universal escape hatch
// ---------------------------------------------------------------------------

func TestMachine_Abort_FromVariousStates(t *testing.T) {
	cases := []struct {
		name  string
		setup []fsm.Event
	}{
		{"from IDLE", nil},
		{"from SCANNING", setupScanning},
		{"from ISSUE_CREATED", setupIssueCreated},
		{"from AWAITING_PR", setupAwaitingPR},
		{"from AWAITING_READY", setupAwaitingReady},
		{"from AWAITING_CI", setupAwaitingCI},
		{"from REVIEWING", setupReviewing},
		{"from DEBUGGING", setupDebugging},
		{"from ADDRESSING_FEEDBACK", setupAddressingFeedback},
		{"from MERGING", setupMerging},
		{"from REFACTORING", setupRefactoring},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			m := fsm.NewMachine(fsm.DefaultConfig())
			mustTransition(t, m, tt.setup...)

			got, err := m.Transition(fsm.EventAbort)

			assert.NoError(t, err)
			assert.Equal(t, fsm.StatePaused, got)
			assert.Equal(t, fsm.StatePaused, m.State())
		})
	}
}

func TestMachine_Abort_FromComplete(t *testing.T) {
	m := fsm.NewMachine(fsm.DefaultConfig())
	mustTransition(t, m, setupScanning...)
	mustTransition(t, m, fsm.EventAllPhasesDone) // → COMPLETE

	got, err := m.Transition(fsm.EventAbort)

	assert.NoError(t, err)
	assert.Equal(t, fsm.StatePaused, got)
}

func TestMachine_Abort_FromPaused_StaysPaused(t *testing.T) {
	m := fsm.NewMachine(fsm.DefaultConfig())
	mustTransition(t, m, fsm.EventAbort) // → PAUSED

	got, err := m.Transition(fsm.EventAbort) // second abort

	assert.NoError(t, err)
	assert.Equal(t, fsm.StatePaused, got)
}
