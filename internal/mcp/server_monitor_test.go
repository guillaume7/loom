package mcp_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/guillaume7/loom/internal/fsm"
	"github.com/guillaume7/loom/internal/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --------------------------------------------------------------------------
// E6: Session Management — fake clock and stall-detection helpers
// --------------------------------------------------------------------------

// fakeClock is a controllable Clock for stall-detection tests.
// It never calls time.Sleep; callers advance the clock via Advance.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Advance moves the fake clock forward by d.
func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// newTestServerWithClock creates a Server backed by a real FSM, in-memory store,
// and the provided Clock. It returns the Server (for RunStallCheck calls) and
// the underlying MCPServer.
func newTestServerWithClock(t *testing.T, clk mcp.Clock) (*mcp.Server, *mcpserver.MCPServer) {
	t.Helper()
	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newMemStore()
	s := mcp.NewServer(machine, st, nil, mcp.WithClock(clk))
	return s, s.MCPServer()
}

// --------------------------------------------------------------------------
// E6: loom_heartbeat wait/retry_in_seconds tests (US-6.5)
// --------------------------------------------------------------------------

func TestLoomHeartbeat_GateState_ReturnsWaitTrue(t *testing.T) {
	_, mcpSvr := newTestServer(t)

	// Advance FSM: IDLE → SCANNING → ISSUE_CREATED → AWAITING_PR (gate state).
	callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": "start"})
	callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": "phase_identified"})
	callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": "copilot_assigned"})

	result := callTool(t, mcpSvr, "loom_heartbeat", nil)
	assert.False(t, result.IsError)

	var got mcp.HeartbeatResult
	require.NoError(t, json.Unmarshal([]byte(toolText(t, result)), &got))

	assert.Equal(t, "AWAITING_PR", got.State)
	assert.True(t, got.Wait, "expected Wait=true in gate state AWAITING_PR")
	assert.Equal(t, 30, got.RetryInSeconds)
}

func TestLoomHeartbeat_NonGateState_ReturnsWaitFalse(t *testing.T) {
	_, mcpSvr := newTestServer(t)

	// After start, SCANNING is not a gate state.
	callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": "start"})

	result := callTool(t, mcpSvr, "loom_heartbeat", nil)
	assert.False(t, result.IsError)

	var got mcp.HeartbeatResult
	require.NoError(t, json.Unmarshal([]byte(toolText(t, result)), &got))

	assert.Equal(t, "SCANNING", got.State)
	assert.False(t, got.Wait, "expected Wait=false in non-gate state SCANNING")
	assert.Equal(t, 0, got.RetryInSeconds)
}

// --------------------------------------------------------------------------
// E6: stall detection tests (US-6.2 / US-6.3) — fake clock, no time.Sleep
// --------------------------------------------------------------------------

func TestRunStallCheck_GateState_Stall_WritesPaused(t *testing.T) {
	clk := newFakeClock()
	s, mcpSvr := newTestServerWithClock(t, clk)

	// Advance FSM to gate state: IDLE → SCANNING → ISSUE_CREATED → AWAITING_PR.
	callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": "start"})
	callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": "phase_identified"})
	callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": "copilot_assigned"})

	// Advance fake clock past stall timeout (default 5 minutes).
	clk.Advance(6 * time.Minute)

	stalled := s.RunStallCheck(context.Background())
	assert.True(t, stalled, "expected stall to be detected after 6 minutes without checkpoint")

	// Verify the store was written with PAUSED.
	cp, err := s.Store().ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "PAUSED", cp.State)
}

func TestRunStallCheck_GateState_WithinTimeout_ReturnsFalse(t *testing.T) {
	clk := newFakeClock()
	s, mcpSvr := newTestServerWithClock(t, clk)

	// Advance FSM to gate state.
	callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": "start"})
	callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": "phase_identified"})
	callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": "copilot_assigned"})

	// Advance clock by only 4 minutes — within the 5-minute stall timeout.
	clk.Advance(4 * time.Minute)

	stalled := s.RunStallCheck(context.Background())
	assert.False(t, stalled, "expected no stall before timeout expires")

	// State should still be AWAITING_PR.
	cp, err := s.Store().ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "AWAITING_PR", cp.State)
}

func TestRunStallCheck_NonGateState_ReturnsFalse(t *testing.T) {
	clk := newFakeClock()
	s, mcpSvr := newTestServerWithClock(t, clk)

	// IDLE → SCANNING: SCANNING is not a gate state.
	callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": "start"})

	// Advance far past any stall timeout.
	clk.Advance(10 * time.Minute)

	stalled := s.RunStallCheck(context.Background())
	assert.False(t, stalled, "expected no stall in non-gate state SCANNING")
}

func TestRunStallCheck_CheckpointResetsStallTimer(t *testing.T) {
	clk := newFakeClock()
	s, mcpSvr := newTestServerWithClock(t, clk)

	// Advance FSM to gate state.
	callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": "start"})
	callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": "phase_identified"})
	callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": "copilot_assigned"})

	// Advance 4 minutes — not yet stalled.
	clk.Advance(4 * time.Minute)

	// A new checkpoint call resets the stall timer.
	// (timeout event keeps us in AWAITING_PR)
	callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": "timeout"})

	// Advance another 4 minutes from the checkpoint — still within timeout.
	clk.Advance(4 * time.Minute)

	stalled := s.RunStallCheck(context.Background())
	assert.False(t, stalled, "expected no stall: checkpoint refreshed the stall timer")
}

func TestRunStallCheck_TOCTOU_CheckpointArrivesBeforeLock_ReturnsFalse(t *testing.T) {
	// Regression test for the TOCTOU race: verify that RunStallCheck does not
	// abort when lastActivity is refreshed after the initial RLock read but
	// before the write lock is acquired.
	//
	// We simulate this by: advancing the clock past the stall timeout, then
	// calling a checkpoint (which updates lastActivity to "now"), and only
	// then calling RunStallCheck. Because RunStallCheck re-verifies lastActivity
	// under the write lock, it must not abort when the activity timestamp is fresh.
	clk := newFakeClock()
	s, mcpSvr := newTestServerWithClock(t, clk)

	// Advance FSM to gate state.
	callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": "start"})
	callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": "phase_identified"})
	callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": "copilot_assigned"})

	// Advance clock past stall timeout.
	clk.Advance(6 * time.Minute)

	// A checkpoint arrives (timeout keeps us in AWAITING_PR) — this resets
	// lastActivity to the current fake-clock time (6 minutes in).
	callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": "timeout"})

	// RunStallCheck now reads lastActivity = clock-now (elapsed since last
	// activity = 0), so no stall should fire even though 6 minutes have passed
	// since the initial gate entry.
	stalled := s.RunStallCheck(context.Background())
	assert.False(t, stalled, "expected no stall after checkpoint reset lastActivity")

	// FSM must still be AWAITING_PR, not PAUSED.
	cp, err := s.Store().ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "AWAITING_PR", cp.State)
}

// --------------------------------------------------------------------------
// E8: Coverage — stateInstruction all-states and WithMonitorConfig
// --------------------------------------------------------------------------

// TestLoomNextStep_AllStates drives the FSM through all reachable states and
// asserts that loom_next_step returns a non-empty instruction for each one.
// This covers the stateInstruction switch in server.go.
func TestLoomNextStep_AllStates(t *testing.T) {
	// Helper: advance FSM to a specific state, then call loom_next_step.
	checkInstruction := func(t *testing.T, transitions []string, wantState string) {
		t.Helper()
		_, mcpSvr := newTestServer(t)
		for _, action := range transitions {
			r := callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": action})
			assert.False(t, r.IsError, "unexpected error on action %q: %s", action, toolText(t, r))
		}
		result := callTool(t, mcpSvr, "loom_next_step", nil)
		assert.False(t, result.IsError)
		var got NextStepResult
		require.NoError(t, json.Unmarshal([]byte(toolText(t, result)), &got))
		assert.Equal(t, wantState, got.State)
		assert.NotEmpty(t, got.Instruction, "instruction must be non-empty for state %s", wantState)
	}

	// Advance to each state using the minimal transition sequence.
	checkInstruction(t, nil, "IDLE")
	checkInstruction(t, []string{"start"}, "SCANNING")
	checkInstruction(t, []string{"start", "phase_identified"}, "ISSUE_CREATED")
	checkInstruction(t, []string{"start", "phase_identified", "copilot_assigned"}, "AWAITING_PR")
	checkInstruction(t,
		[]string{"start", "phase_identified", "copilot_assigned", "pr_opened"}, "AWAITING_READY")
	checkInstruction(t,
		[]string{"start", "phase_identified", "copilot_assigned", "pr_opened", "pr_ready"}, "AWAITING_CI")
	checkInstruction(t,
		[]string{"start", "phase_identified", "copilot_assigned", "pr_opened", "pr_ready", "ci_green"}, "REVIEWING")
	checkInstruction(t,
		[]string{"start", "phase_identified", "copilot_assigned", "pr_opened", "pr_ready", "ci_red"}, "DEBUGGING")
	checkInstruction(t,
		[]string{
			"start", "phase_identified", "copilot_assigned", "pr_opened", "pr_ready",
			"ci_green", "review_changes_requested",
		}, "ADDRESSING_FEEDBACK")
	checkInstruction(t,
		[]string{
			"start", "phase_identified", "copilot_assigned", "pr_opened", "pr_ready",
			"ci_green", "review_approved",
		}, "MERGING")
	checkInstruction(t,
		[]string{
			"start", "phase_identified", "copilot_assigned", "pr_opened", "pr_ready",
			"ci_green", "review_approved", "merged_epic_boundary",
		}, "REFACTORING")
	checkInstruction(t, []string{"start", "all_phases_done"}, "COMPLETE")
	checkInstruction(t, []string{"abort"}, "PAUSED")
}

// NextStepResult mirrors mcp.NextStepResult for JSON unmarshalling in tests
// without importing the production type.
type NextStepResult = mcp.NextStepResult

// TestWithMonitorConfig_AppliesConfig verifies that WithMonitorConfig sets
// the monitor configuration on the Server (covering the option function).
func TestWithMonitorConfig_AppliesConfig(t *testing.T) {
	clk := newFakeClock()
	cfg := mcp.MonitorConfig{
		StallTimeout:      1 * time.Second,
		HeartbeatInterval: 2 * time.Second,
		TickInterval:      500 * time.Millisecond,
	}
	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newMemStore()
	s := mcp.NewServer(machine, st, nil, mcp.WithClock(clk), mcp.WithMonitorConfig(cfg))
	mcpSvr := s.MCPServer()

	// Drive to gate state.
	callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": "start"})
	callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": "phase_identified"})
	callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": "copilot_assigned"})

	// Custom stall timeout is 1 second; advance 2 seconds — must stall.
	clk.Advance(2 * time.Second)
	stalled := s.RunStallCheck(context.Background())
	assert.True(t, stalled, "expected stall with custom 1-second StallTimeout")
}
