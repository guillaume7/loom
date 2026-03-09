package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/guillaume7/loom/internal/fsm"
	"github.com/guillaume7/loom/internal/mcp"
	"github.com/guillaume7/loom/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStallDetection verifies that RunStallCheck transitions the FSM to PAUSED
// when no checkpoint has been received for longer than StallTimeout, and that
// a resumed FSM (new Server re-initialized to the pre-stall state) operates
// correctly afterwards (US-8.4).
func TestStallDetection(t *testing.T) {
	clk := newFakeClock()

	// Use a tiny stall timeout to make the test fast.
	stallCfg := mcp.MonitorConfig{
		StallTimeout:      10 * time.Second,
		HeartbeatInterval: 60 * time.Second,
		TickInterval:      10 * time.Second,
	}

	machine := fsm.NewMachine(fsm.DefaultConfig())
	st, err := store.New(":memory:")
	require.NoError(t, err)
	defer st.Close()

	server := mcp.NewServer(machine, st, nil,
		mcp.WithClock(clk),
		mcp.WithMonitorConfig(stallCfg),
	)
	mcpSvr := server.MCPServer()
	ctx := context.Background()

	// Drive FSM to AWAITING_PR — a gate state where stall detection is active.
	checkpoint(t, mcpSvr, "start")               // IDLE → SCANNING
	checkpoint(t, mcpSvr, "phase_identified")     // SCANNING → ISSUE_CREATED
	checkpoint(t, mcpSvr, "copilot_assigned")     // ISSUE_CREATED → AWAITING_PR

	step := nextStep(t, mcpSvr)
	require.Equal(t, "AWAITING_PR", step.State, "FSM must be in AWAITING_PR before stall test")

	// ── Stall not yet triggered (within timeout) ──────────────────────────

	clk.Advance(9 * time.Second) // just inside 10-second threshold
	stalled := server.RunStallCheck(ctx)
	assert.False(t, stalled, "expected no stall before timeout expires")

	// FSM must still be AWAITING_PR.
	step = nextStep(t, mcpSvr)
	assert.Equal(t, "AWAITING_PR", step.State)

	// ── Stall triggered (past timeout) ───────────────────────────────────

	clk.Advance(2 * time.Second) // now 11 seconds since last checkpoint
	stalled = server.RunStallCheck(ctx)
	assert.True(t, stalled, "expected stall to be detected after 11 seconds")

	// FSM must now be PAUSED.
	step = nextStep(t, mcpSvr)
	assert.Equal(t, "PAUSED", step.State, "FSM must be PAUSED after stall")

	// Store must persist PAUSED.
	cp, err := st.ReadCheckpoint(ctx)
	require.NoError(t, err)
	assert.Equal(t, "PAUSED", cp.State, "store checkpoint must be PAUSED after stall")

	// ── Resume: create new Server re-initialized to AWAITING_PR ──────────

	// Simulate loom resume: new FSM driven back to the gate state.
	machine2 := fsm.NewMachine(fsm.DefaultConfig())
	st2, err := store.New(":memory:")
	require.NoError(t, err)
	defer st2.Close()

	server2 := mcp.NewServer(machine2, st2, nil)
	mcpSvr2 := server2.MCPServer()

	// Replay the transitions to get back to AWAITING_PR.
	checkpoint(t, mcpSvr2, "start")
	checkpoint(t, mcpSvr2, "phase_identified")
	checkpoint(t, mcpSvr2, "copilot_assigned")

	step2 := nextStep(t, mcpSvr2)
	assert.Equal(t, "AWAITING_PR", step2.State,
		"resumed FSM must be back in AWAITING_PR, not PAUSED")

	// A further stall check with no clock advance must not fire.
	stalled2 := server2.RunStallCheck(ctx)
	assert.False(t, stalled2, "no stall expected immediately after resume")
}

// TestStallDetection_CheckpointResetsTimer verifies that receiving a
// loom_checkpoint call resets the stall timer so a stall is not declared
// even when the total elapsed time exceeds StallTimeout.
func TestStallDetection_CheckpointResetsTimer(t *testing.T) {
	clk := newFakeClock()
	stallCfg := mcp.MonitorConfig{
		StallTimeout:      10 * time.Second,
		HeartbeatInterval: 60 * time.Second,
		TickInterval:      10 * time.Second,
	}

	machine := fsm.NewMachine(fsm.DefaultConfig())
	st, err := store.New(":memory:")
	require.NoError(t, err)
	defer st.Close()

	server := mcp.NewServer(machine, st, nil,
		mcp.WithClock(clk),
		mcp.WithMonitorConfig(stallCfg),
	)
	mcpSvr := server.MCPServer()
	ctx := context.Background()

	// Drive FSM to AWAITING_PR.
	checkpoint(t, mcpSvr, "start")
	checkpoint(t, mcpSvr, "phase_identified")
	checkpoint(t, mcpSvr, "copilot_assigned")

	// Advance 8 seconds — not yet stalled.
	clk.Advance(8 * time.Second)

	// A timeout event keeps us in AWAITING_PR and resets lastActivity.
	checkpoint(t, mcpSvr, "timeout") // AWAITING_PR → AWAITING_PR

	// Advance another 8 seconds from the new checkpoint.
	// Total elapsed since original entry = 16s, but since last checkpoint = 8s.
	clk.Advance(8 * time.Second)

	stalled := server.RunStallCheck(ctx)
	assert.False(t, stalled, "expected no stall: checkpoint reset the stall timer")
}
