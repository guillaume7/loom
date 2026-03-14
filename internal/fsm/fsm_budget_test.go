package fsm_test

import (
	"testing"

	"github.com/guillaume7/loom/internal/fsm"
	"github.com/stretchr/testify/assert"
)

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

	// Budget exhausted -> force-promote to AWAITING_CI (not PAUSED).
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
// Debug cycle budget (AWAITING_CI -> DEBUGGING loop)
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
// Feedback cycle budget (REVIEWING -> ADDRESSING_FEEDBACK loop)
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

		mustTransition(t, m, fsm.EventCIGreen)
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
			fsm.EventCIGreen,
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

	got, err := m.Transition(fsm.EventTimeout)
	assert.NoError(t, err)
	assert.Equal(t, fsm.StateAwaitingPR, got)

	got, err = m.Transition(fsm.EventTimeout)
	assert.NoError(t, err)
	assert.Equal(t, fsm.StatePaused, got)
}

// ---------------------------------------------------------------------------
// Counter reset on re-entry to gate states
// ---------------------------------------------------------------------------

func TestMachine_CounterReset_AwaitingCIRetries(t *testing.T) {
	cfg := fsm.Config{
		MaxRetriesAwaitingPR:    20,
		MaxRetriesAwaitingReady: 60,
		MaxRetriesAwaitingCI:    2,
		MaxDebugCycles:          5,
		MaxFeedbackCycles:       5,
	}
	m := fsm.NewMachine(cfg)
	mustTransition(t, m, setupAwaitingCI...)

	for i := 0; i < cfg.MaxRetriesAwaitingCI; i++ {
		mustTransition(t, m, fsm.EventTimeout)
	}

	mustTransition(t, m, fsm.EventCIRed, fsm.EventFixPushed)

	for i := 0; i < cfg.MaxRetriesAwaitingCI; i++ {
		got, err := m.Transition(fsm.EventTimeout)
		assert.NoError(t, err, "retry %d after re-entry should not error", i+1)
		assert.Equal(t, fsm.StateAwaitingCI, got)
	}
}

func TestMachine_CounterReset_ScanningResetsLoopCounters(t *testing.T) {
	cfg := fsm.Config{
		MaxRetriesAwaitingPR:    20,
		MaxRetriesAwaitingReady: 60,
		MaxRetriesAwaitingCI:    20,
		MaxDebugCycles:          1,
		MaxFeedbackCycles:       5,
	}
	m := fsm.NewMachine(cfg)
	mustTransition(t, m, setupAwaitingCI...)

	mustTransition(t, m, fsm.EventCIRed, fsm.EventFixPushed)

	mustTransition(t, m,
		fsm.EventCIGreen,
		fsm.EventReviewApproved,
		fsm.EventMerged,
	)

	mustTransition(t, m,
		fsm.EventPhaseIdentified,
		fsm.EventCopilotAssigned,
		fsm.EventPROpened,
		fsm.EventPRReady,
	)

	got, err := m.Transition(fsm.EventCIRed)
	assert.NoError(t, err)
	assert.Equal(t, fsm.StateDebugging, got, "debug cycle should be allowed again after SCANNING reset")
}

func TestMachine_CounterReset_SkipStoryResetsAwaitingCIRetries(t *testing.T) {
	cfg := fsm.Config{
		MaxRetriesAwaitingPR:    20,
		MaxRetriesAwaitingReady: 60,
		MaxRetriesAwaitingCI:    1,
		MaxDebugCycles:          5,
		MaxFeedbackCycles:       5,
	}
	m := fsm.NewMachine(cfg)
	mustTransition(t, m, setupAwaitingCI...)

	// Consume the in-state CI timeout budget once.
	got, err := m.Transition(fsm.EventTimeout)
	assert.NoError(t, err)
	assert.Equal(t, fsm.StateAwaitingCI, got)

	// Skip to SCANNING and return to AWAITING_CI for a new story.
	mustTransition(t, m, fsm.EventSkipStory)
	mustTransition(t, m,
		fsm.EventPhaseIdentified,
		fsm.EventCopilotAssigned,
		fsm.EventPROpened,
		fsm.EventPRReady,
	)

	// Budget must be fresh after skip->SCANNING transition.
	got, err = m.Transition(fsm.EventTimeout)
	assert.NoError(t, err)
	assert.Equal(t, fsm.StateAwaitingCI, got)
}
