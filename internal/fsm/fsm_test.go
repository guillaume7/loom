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

		// AWAITING_READY
		{"AWAITING_READY+pr_ready→AWAITING_CI", setupAwaitingReady, fsm.EventPRReady, fsm.StateAwaitingCI},

		// AWAITING_CI
		{"AWAITING_CI+ci_green→REVIEWING", setupAwaitingCI, fsm.EventCIGreen, fsm.StateReviewing},
		{"AWAITING_CI+ci_red→DEBUGGING", setupAwaitingCI, fsm.EventCIRed, fsm.StateDebugging},

		// REVIEWING
		{"REVIEWING+review_approved→MERGING", setupReviewing, fsm.EventReviewApproved, fsm.StateMerging},
		{"REVIEWING+review_changes_requested→ADDRESSING_FEEDBACK", setupReviewing, fsm.EventReviewChangesRequested, fsm.StateAddressingFeedback},

		// DEBUGGING
		{"DEBUGGING+fix_pushed→AWAITING_CI", setupDebugging, fsm.EventFixPushed, fsm.StateAwaitingCI},

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

// ---------------------------------------------------------------------------
// AWAITING_PR — timeout retry budget
// ---------------------------------------------------------------------------

func TestMachine_AwaitingPR_TimeoutRetry_WithinBudget(t *testing.T) {
	cfg := fsm.DefaultConfig()
	cfg.MaxRetriesAwaitingPR = 3
	m := fsm.NewMachine(cfg)
	mustTransition(t, m, setupAwaitingPR...)

	for i := 0; i < cfg.MaxRetriesAwaitingPR; i++ {
		got, err := m.Transition(fsm.EventTimeout)
		assert.NoError(t, err, "retry %d should not error", i+1)
		assert.Equal(t, fsm.StateAwaitingPR, got, "retry %d should stay in AWAITING_PR", i+1)
	}
}

func TestMachine_AwaitingPR_TimeoutExhausted_GoesToPaused(t *testing.T) {
	cfg := fsm.DefaultConfig()
	cfg.MaxRetriesAwaitingPR = 3
	m := fsm.NewMachine(cfg)
	mustTransition(t, m, setupAwaitingPR...)

	for i := 0; i < cfg.MaxRetriesAwaitingPR; i++ {
		mustTransition(t, m, fsm.EventTimeout)
	}

	got, err := m.Transition(fsm.EventTimeout)
	assert.NoError(t, err)
	assert.Equal(t, fsm.StatePaused, got)
}

// ---------------------------------------------------------------------------
// AWAITING_READY — timeout retry budget (force-promote)
// ---------------------------------------------------------------------------

func TestMachine_AwaitingReady_TimeoutRetry_WithinBudget(t *testing.T) {
	cfg := fsm.DefaultConfig()
	cfg.MaxRetriesAwaitingReady = 2
	m := fsm.NewMachine(cfg)
	mustTransition(t, m, setupAwaitingReady...)

	for i := 0; i < cfg.MaxRetriesAwaitingReady; i++ {
		got, err := m.Transition(fsm.EventTimeout)
		assert.NoError(t, err, "retry %d should not error", i+1)
		assert.Equal(t, fsm.StateAwaitingReady, got, "retry %d should stay in AWAITING_READY", i+1)
	}
}

func TestMachine_AwaitingReady_TimeoutExhausted_ForcePromotes(t *testing.T) {
	cfg := fsm.DefaultConfig()
	cfg.MaxRetriesAwaitingReady = 2
	m := fsm.NewMachine(cfg)
	mustTransition(t, m, setupAwaitingReady...)

	for i := 0; i < cfg.MaxRetriesAwaitingReady; i++ {
		mustTransition(t, m, fsm.EventTimeout)
	}

	// Budget exhausted → force-promote to AWAITING_CI (not PAUSED).
	got, err := m.Transition(fsm.EventTimeout)
	assert.NoError(t, err)
	assert.Equal(t, fsm.StateAwaitingCI, got)
}

// ---------------------------------------------------------------------------
// AWAITING_CI — timeout retry budget
// ---------------------------------------------------------------------------

func TestMachine_AwaitingCI_TimeoutRetry_WithinBudget(t *testing.T) {
	cfg := fsm.DefaultConfig()
	cfg.MaxRetriesAwaitingCI = 3
	m := fsm.NewMachine(cfg)
	mustTransition(t, m, setupAwaitingCI...)

	for i := 0; i < cfg.MaxRetriesAwaitingCI; i++ {
		got, err := m.Transition(fsm.EventTimeout)
		assert.NoError(t, err, "retry %d should not error", i+1)
		assert.Equal(t, fsm.StateAwaitingCI, got, "retry %d should stay in AWAITING_CI", i+1)
	}
}

func TestMachine_AwaitingCI_TimeoutExhausted_GoesToPaused(t *testing.T) {
	cfg := fsm.DefaultConfig()
	cfg.MaxRetriesAwaitingCI = 3
	m := fsm.NewMachine(cfg)
	mustTransition(t, m, setupAwaitingCI...)

	for i := 0; i < cfg.MaxRetriesAwaitingCI; i++ {
		mustTransition(t, m, fsm.EventTimeout)
	}

	got, err := m.Transition(fsm.EventTimeout)
	assert.NoError(t, err)
	assert.Equal(t, fsm.StatePaused, got)
}

// ---------------------------------------------------------------------------
// Debug cycle budget (AWAITING_CI → DEBUGGING loop)
// ---------------------------------------------------------------------------

func TestMachine_DebugCycles_WithinBudget(t *testing.T) {
	cfg := fsm.DefaultConfig()
	cfg.MaxDebugCycles = 2
	m := fsm.NewMachine(cfg)
	mustTransition(t, m, setupAwaitingCI...)

	for cycle := 0; cycle < cfg.MaxDebugCycles; cycle++ {
		got, err := m.Transition(fsm.EventCIRed)
		assert.NoError(t, err, "cycle %d ci_red should not error", cycle+1)
		assert.Equal(t, fsm.StateDebugging, got)

		got, err = m.Transition(fsm.EventFixPushed)
		assert.NoError(t, err, "cycle %d fix_pushed should not error", cycle+1)
		assert.Equal(t, fsm.StateAwaitingCI, got)
	}
}

func TestMachine_DebugCycles_Exhausted_GoesToPaused(t *testing.T) {
	cfg := fsm.DefaultConfig()
	cfg.MaxDebugCycles = 2
	m := fsm.NewMachine(cfg)
	mustTransition(t, m, setupAwaitingCI...)

	for cycle := 0; cycle < cfg.MaxDebugCycles; cycle++ {
		mustTransition(t, m, fsm.EventCIRed, fsm.EventFixPushed)
	}

	got, err := m.Transition(fsm.EventCIRed)
	assert.NoError(t, err)
	assert.Equal(t, fsm.StatePaused, got)
}

// ---------------------------------------------------------------------------
// Feedback cycle budget (REVIEWING → ADDRESSING_FEEDBACK loop)
// ---------------------------------------------------------------------------

func TestMachine_FeedbackCycles_WithinBudget(t *testing.T) {
	cfg := fsm.DefaultConfig()
	cfg.MaxFeedbackCycles = 2
	m := fsm.NewMachine(cfg)
	mustTransition(t, m, setupReviewing...)

	for cycle := 0; cycle < cfg.MaxFeedbackCycles; cycle++ {
		got, err := m.Transition(fsm.EventReviewChangesRequested)
		assert.NoError(t, err, "cycle %d changes_requested should not error", cycle+1)
		assert.Equal(t, fsm.StateAddressingFeedback, got)

		got, err = m.Transition(fsm.EventFeedbackAddressed)
		assert.NoError(t, err, "cycle %d feedback_addressed should not error", cycle+1)
		assert.Equal(t, fsm.StateAwaitingCI, got)

		mustTransition(t, m, fsm.EventCIGreen) // return to REVIEWING for next cycle
	}
}

func TestMachine_FeedbackCycles_Exhausted_GoesToPaused(t *testing.T) {
	cfg := fsm.DefaultConfig()
	cfg.MaxFeedbackCycles = 2
	m := fsm.NewMachine(cfg)
	mustTransition(t, m, setupReviewing...)

	for cycle := 0; cycle < cfg.MaxFeedbackCycles; cycle++ {
		mustTransition(t, m,
			fsm.EventReviewChangesRequested,
			fsm.EventFeedbackAddressed,
			fsm.EventCIGreen, // back to REVIEWING
		)
	}

	got, err := m.Transition(fsm.EventReviewChangesRequested)
	assert.NoError(t, err)
	assert.Equal(t, fsm.StatePaused, got)
}

// ---------------------------------------------------------------------------
// Custom Config is respected
// ---------------------------------------------------------------------------

func TestMachine_CustomConfig(t *testing.T) {
	cfg := fsm.Config{
		MaxRetriesAwaitingPR:    1,
		MaxRetriesAwaitingReady: 1,
		MaxRetriesAwaitingCI:    1,
		MaxDebugCycles:          1,
		MaxFeedbackCycles:       1,
	}
	m := fsm.NewMachine(cfg)
	mustTransition(t, m, setupAwaitingPR...)

	// First timeout is within budget.
	got, err := m.Transition(fsm.EventTimeout)
	assert.NoError(t, err)
	assert.Equal(t, fsm.StateAwaitingPR, got)

	// Second timeout exceeds MaxRetriesAwaitingPR=1 → PAUSED.
	got, err = m.Transition(fsm.EventTimeout)
	assert.NoError(t, err)
	assert.Equal(t, fsm.StatePaused, got)
}

// ---------------------------------------------------------------------------
// Counter reset on re-entry to gate states
// ---------------------------------------------------------------------------

// TestMachine_CounterReset_AwaitingCIRetries verifies that the AWAITING_CI
// timeout counter resets to zero whenever the machine re-enters that state,
// giving each visit a fresh budget.
func TestMachine_CounterReset_AwaitingCIRetries(t *testing.T) {
	cfg := fsm.Config{
		MaxRetriesAwaitingPR:    20,
		MaxRetriesAwaitingReady: 60,
		MaxRetriesAwaitingCI:    2, // small for clarity
		MaxDebugCycles:          5, // generous so debug loop doesn't interfere
		MaxFeedbackCycles:       5,
	}
	m := fsm.NewMachine(cfg)
	mustTransition(t, m, setupAwaitingCI...)

	// Consume the full AWAITING_CI timeout budget.
	for i := 0; i < cfg.MaxRetriesAwaitingCI; i++ {
		mustTransition(t, m, fsm.EventTimeout)
	}

	// Take the debug detour; re-entering AWAITING_CI must reset the counter.
	mustTransition(t, m, fsm.EventCIRed, fsm.EventFixPushed) // → AWAITING_CI (counter reset)

	// The full timeout budget should be available again.
	for i := 0; i < cfg.MaxRetriesAwaitingCI; i++ {
		got, err := m.Transition(fsm.EventTimeout)
		assert.NoError(t, err, "retry %d after re-entry should not error", i+1)
		assert.Equal(t, fsm.StateAwaitingCI, got)
	}
}

// TestMachine_CounterReset_ScanningResetsLoopCounters verifies that
// transitioning back to SCANNING resets the debug and feedback cycle counters,
// allowing a fresh set of cycles for the next workflow pass.
func TestMachine_CounterReset_ScanningResetsLoopCounters(t *testing.T) {
	cfg := fsm.Config{
		MaxRetriesAwaitingPR:    20,
		MaxRetriesAwaitingReady: 60,
		MaxRetriesAwaitingCI:    20,
		MaxDebugCycles:          1, // only 1 debug cycle per pass
		MaxFeedbackCycles:       5,
	}
	m := fsm.NewMachine(cfg)
	mustTransition(t, m, setupAwaitingCI...)

	// Consume the sole debug cycle allowed in this pass.
	mustTransition(t, m, fsm.EventCIRed, fsm.EventFixPushed) // debugCycles=1

	// Complete the phase and return to SCANNING (which resets debugCycles).
	mustTransition(t, m,
		fsm.EventCIGreen,
		fsm.EventReviewApproved,
		fsm.EventMerged, // → SCANNING; debugCycles reset to 0
	)

	// Begin a new phase from SCANNING.
	mustTransition(t, m,
		fsm.EventPhaseIdentified,
		fsm.EventCopilotAssigned,
		fsm.EventPROpened,
		fsm.EventPRReady, // → AWAITING_CI with fresh debugCycles=0
	)

	// The debug cycle budget must be fresh: ci_red should go to DEBUGGING.
	got, err := m.Transition(fsm.EventCIRed)
	assert.NoError(t, err)
	assert.Equal(t, fsm.StateDebugging, got, "debug cycle should be allowed again after SCANNING reset")
}

// ---------------------------------------------------------------------------
// Snapshot and Rollback
// ---------------------------------------------------------------------------

// TestMachine_TakeSnapshot_CapturesState verifies that TakeSnapshot returns a
// snapshot reflecting the current FSM state.
func TestMachine_TakeSnapshot_CapturesState(t *testing.T) {
	m := fsm.NewMachine(fsm.DefaultConfig())
	mustTransition(t, m, fsm.EventStart) // IDLE → SCANNING

	snap := m.TakeSnapshot()

	// Advance to a new state after the snapshot.
	mustTransition(t, m, fsm.EventPhaseIdentified) // → ISSUE_CREATED
	assert.Equal(t, fsm.StateIssueCreated, m.State())

	// Rollback restores to SCANNING.
	m.Rollback(snap)
	assert.Equal(t, fsm.StateScanning, m.State())
}

// TestMachine_Rollback_RestoresCounters verifies that Rollback restores
// retry counters so a failed-and-retried operation does not consume an extra
// budget unit.
func TestMachine_Rollback_RestoresCounters(t *testing.T) {
	cfg := fsm.DefaultConfig()
	cfg.MaxRetriesAwaitingPR = 2
	m := fsm.NewMachine(cfg)
	mustTransition(t, m, setupAwaitingPR...)

	// Consume one timeout (awaitingPRRetries becomes 1 after transition).
	snap := m.TakeSnapshot()
	_, err := m.Transition(fsm.EventTimeout)
	assert.NoError(t, err)

	// Rollback restores the counter to the pre-transition value.
	m.Rollback(snap)

	// After rollback we should be able to fire two more timeouts within budget
	// (same budget as if the first one never happened).
	for i := 0; i < cfg.MaxRetriesAwaitingPR; i++ {
		got, err := m.Transition(fsm.EventTimeout)
		assert.NoError(t, err, "retry %d should succeed after rollback", i+1)
		assert.Equal(t, fsm.StateAwaitingPR, got)
	}
	// Now budget is exhausted.
	got, err := m.Transition(fsm.EventTimeout)
	assert.NoError(t, err)
	assert.Equal(t, fsm.StatePaused, got)
}

// TestMachine_Rollback_AllowsRetry verifies that after a rollback the same
// event can be successfully fired again — the core property required for the
// same-process retry scenario in the MCP server.
func TestMachine_Rollback_AllowsRetry(t *testing.T) {
	m := fsm.NewMachine(fsm.DefaultConfig())

	snap := m.TakeSnapshot()
	got, err := m.Transition(fsm.EventStart)
	assert.NoError(t, err)
	assert.Equal(t, fsm.StateScanning, got)

	// Simulate a failed store write: roll back.
	m.Rollback(snap)
	assert.Equal(t, fsm.StateIdle, m.State(), "must be back at IDLE after rollback")

	// Retry the same event — must succeed.
	got, err = m.Transition(fsm.EventStart)
	assert.NoError(t, err)
	assert.Equal(t, fsm.StateScanning, got)
}

