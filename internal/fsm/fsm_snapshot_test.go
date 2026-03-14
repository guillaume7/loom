package fsm_test

import (
	"testing"

	"github.com/guillaume7/loom/internal/fsm"
	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// Snapshot and Rollback
// ---------------------------------------------------------------------------

func TestMachine_TakeSnapshot_CapturesState(t *testing.T) {
	m := fsm.NewMachine(fsm.DefaultConfig())
	mustTransition(t, m, fsm.EventStart)

	snap := m.TakeSnapshot()

	mustTransition(t, m, fsm.EventPhaseIdentified)
	assert.Equal(t, fsm.StateIssueCreated, m.State())

	m.Rollback(snap)
	assert.Equal(t, fsm.StateScanning, m.State())
}

func TestMachine_Rollback_RestoresCounters(t *testing.T) {
	cfg := fsm.DefaultConfig()
	cfg.MaxRetriesAwaitingPR = 2
	m := fsm.NewMachine(cfg)
	mustTransition(t, m, setupAwaitingPR...)

	snap := m.TakeSnapshot()
	_, err := m.Transition(fsm.EventTimeout)
	assert.NoError(t, err)

	m.Rollback(snap)

	for i := 0; i < cfg.MaxRetriesAwaitingPR; i++ {
		got, err := m.Transition(fsm.EventTimeout)
		assert.NoError(t, err, "retry %d should succeed after rollback", i+1)
		assert.Equal(t, fsm.StateAwaitingPR, got)
	}
	got, err := m.Transition(fsm.EventTimeout)
	assert.NoError(t, err)
	assert.Equal(t, fsm.StatePaused, got)
}

func TestMachine_Rollback_AllowsRetry(t *testing.T) {
	m := fsm.NewMachine(fsm.DefaultConfig())

	snap := m.TakeSnapshot()
	got, err := m.Transition(fsm.EventStart)
	assert.NoError(t, err)
	assert.Equal(t, fsm.StateScanning, got)

	m.Rollback(snap)
	assert.Equal(t, fsm.StateIdle, m.State(), "must be back at IDLE after rollback")

	got, err = m.Transition(fsm.EventStart)
	assert.NoError(t, err)
	assert.Equal(t, fsm.StateScanning, got)
}
