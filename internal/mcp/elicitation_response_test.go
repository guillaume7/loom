package mcp_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/guillaume7/loom/internal/fsm"
	"github.com/guillaume7/loom/internal/mcp"
	"github.com/guillaume7/loom/internal/store"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type reassignGitHubClientMock struct {
	closedIssues []int
}

func (m *reassignGitHubClientMock) Ping(context.Context) error { return nil }

func (m *reassignGitHubClientMock) CloseIssue(_ context.Context, issueNumber int) error {
	m.closedIssues = append(m.closedIssues, issueNumber)
	return nil
}

func prepareBudgetExhaustionElicitationForState(
	t *testing.T,
	mcpSvr *mcpserver.MCPServer,
	st *memStore,
	setupActions []string,
	checkpoint store.Checkpoint,
	budgetExhaustionAction string,
	sessionCaps map[string]interface{},
) *testSession {
	t.Helper()

	for _, action := range setupActions {
		result := callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": action})
		require.False(t, result.IsError, "failed to advance with action %q: %s", action, toolText(t, result))
	}

	require.NoError(t, st.WriteCheckpoint(context.Background(), checkpoint))

	sess := newTestSession(nextSessionID())
	require.NoError(t, mcpSvr.RegisterSession(context.Background(), sess))
	initializeSessionWithCapabilities(t, mcpSvr, sess, sessionCaps)

	checkpointResult := callToolOnSession(t, mcpSvr, sess, "loom_checkpoint", map[string]interface{}{"action": budgetExhaustionAction})
	require.False(t, checkpointResult.IsError)
	return sess
}

func prepareBudgetExhaustionElicitation(t *testing.T, mcpSvr *mcpserver.MCPServer, st *memStore, sessionCaps map[string]interface{}) *testSession {
	t.Helper()
	return prepareBudgetExhaustionElicitationForState(
		t,
		mcpSvr,
		st,
		[]string{"start", "phase_identified", "copilot_assigned", "pr_opened", "pr_ready"},
		store.Checkpoint{State: string(fsm.StateAwaitingCI), PRNumber: 42, Phase: 3},
		"timeout",
		sessionCaps,
	)
}

func prepareAwaitingPRElicitation(t *testing.T, mcpSvr *mcpserver.MCPServer, st *memStore, sessionCaps map[string]interface{}) *testSession {
	t.Helper()
	return prepareBudgetExhaustionElicitationForState(
		t,
		mcpSvr,
		st,
		[]string{"start", "phase_identified", "copilot_assigned"},
		store.Checkpoint{State: string(fsm.StateAwaitingPR), PRNumber: 42, Phase: 3},
		"timeout",
		sessionCaps,
	)
}

func prepareReviewingElicitation(t *testing.T, mcpSvr *mcpserver.MCPServer, st *memStore, sessionCaps map[string]interface{}) *testSession {
	t.Helper()
	return prepareBudgetExhaustionElicitationForState(
		t,
		mcpSvr,
		st,
		[]string{"start", "phase_identified", "copilot_assigned", "pr_opened", "pr_ready", "ci_green"},
		store.Checkpoint{State: string(fsm.StateReviewing), PRNumber: 42, Phase: 3},
		"review_changes_requested",
		sessionCaps,
	)
}

func TestLoomElicitationResponse_Skip_FiresSkipStoryEvent(t *testing.T) {
	cfg := fsm.DefaultConfig()
	cfg.MaxRetriesAwaitingCI = 0
	machine := fsm.NewMachine(cfg)
	st := newMemStore()
	s := mcp.NewServer(machine, st, nil)
	mcpSvr := s.MCPServer()

	sess := prepareBudgetExhaustionElicitation(t, mcpSvr, st, map[string]interface{}{
		"experimental": map[string]interface{}{
			"elicitation": true,
		},
	})

	result := callToolOnSession(t, mcpSvr, sess, "loom_elicitation_response", map[string]interface{}{"action": "skip"})
	require.False(t, result.IsError)

	var got mcp.ElicitationResponseResult
	require.NoError(t, json.Unmarshal([]byte(toolText(t, result)), &got))
	assert.Equal(t, "skip", got.Action)
	assert.Equal(t, "AWAITING_CI", got.PreviousState)
	assert.Equal(t, "SCANNING", got.NewState)

	cp, err := st.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "SCANNING", cp.State)
	assert.Equal(t, 4, cp.Phase)
	assert.Equal(t, 0, cp.RetryCount)
}

func TestLoomElicitationResponse_Skip_FromAwaitingPR_FiresSkipStoryEvent(t *testing.T) {
	cfg := fsm.DefaultConfig()
	cfg.MaxRetriesAwaitingPR = 0
	machine := fsm.NewMachine(cfg)
	st := newMemStore()
	s := mcp.NewServer(machine, st, nil)
	mcpSvr := s.MCPServer()

	sess := prepareAwaitingPRElicitation(t, mcpSvr, st, map[string]interface{}{
		"experimental": map[string]interface{}{
			"elicitation": true,
		},
	})

	result := callToolOnSession(t, mcpSvr, sess, "loom_elicitation_response", map[string]interface{}{"action": "skip"})
	require.False(t, result.IsError)

	var got mcp.ElicitationResponseResult
	require.NoError(t, json.Unmarshal([]byte(toolText(t, result)), &got))
	assert.Equal(t, "skip", got.Action)
	assert.Equal(t, "AWAITING_PR", got.PreviousState)
	assert.Equal(t, "SCANNING", got.NewState)

	cp, err := st.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "SCANNING", cp.State)
	assert.Equal(t, 4, cp.Phase)
	assert.Equal(t, 0, cp.RetryCount)
}

func TestLoomElicitationResponse_Skip_FromReviewing_FiresSkipStoryEvent(t *testing.T) {
	cfg := fsm.DefaultConfig()
	cfg.MaxFeedbackCycles = 0
	machine := fsm.NewMachine(cfg)
	st := newMemStore()
	s := mcp.NewServer(machine, st, nil)
	mcpSvr := s.MCPServer()

	sess := prepareReviewingElicitation(t, mcpSvr, st, map[string]interface{}{
		"experimental": map[string]interface{}{
			"elicitation": true,
		},
	})

	result := callToolOnSession(t, mcpSvr, sess, "loom_elicitation_response", map[string]interface{}{"action": "skip"})
	require.False(t, result.IsError)

	var got mcp.ElicitationResponseResult
	require.NoError(t, json.Unmarshal([]byte(toolText(t, result)), &got))
	assert.Equal(t, "skip", got.Action)
	assert.Equal(t, "REVIEWING", got.PreviousState)
	assert.Equal(t, "SCANNING", got.NewState)

	cp, err := st.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "SCANNING", cp.State)
	assert.Equal(t, 4, cp.Phase)
	assert.Equal(t, 0, cp.RetryCount)
}

func TestLoomElicitationResponse_Reassign_ClosesPRAndResetsToIssueCreated(t *testing.T) {
	cfg := fsm.DefaultConfig()
	cfg.MaxRetriesAwaitingCI = 0
	machine := fsm.NewMachine(cfg)
	st := newMemStore()
	gh := &reassignGitHubClientMock{}
	s := mcp.NewServer(machine, st, gh)
	mcpSvr := s.MCPServer()

	sess := prepareBudgetExhaustionElicitation(t, mcpSvr, st, map[string]interface{}{
		"experimental": map[string]interface{}{
			"elicitation": true,
		},
	})

	result := callToolOnSession(t, mcpSvr, sess, "loom_elicitation_response", map[string]interface{}{"action": "reassign"})
	require.False(t, result.IsError)

	var got mcp.ElicitationResponseResult
	require.NoError(t, json.Unmarshal([]byte(toolText(t, result)), &got))
	assert.Equal(t, "reassign", got.Action)
	assert.Equal(t, "ISSUE_CREATED", got.NewState)

	cp, err := st.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "ISSUE_CREATED", cp.State)
	assert.Equal(t, 0, cp.PRNumber)
	require.Len(t, gh.closedIssues, 1)
	assert.Equal(t, 42, gh.closedIssues[0])
}

func TestLoomElicitationResponse_PauseEpic_TransitionsPaused(t *testing.T) {
	cfg := fsm.DefaultConfig()
	cfg.MaxRetriesAwaitingCI = 0
	machine := fsm.NewMachine(cfg)
	st := newMemStore()
	s := mcp.NewServer(machine, st, nil)
	mcpSvr := s.MCPServer()

	sess := prepareBudgetExhaustionElicitation(t, mcpSvr, st, map[string]interface{}{
		"experimental": map[string]interface{}{
			"elicitation": true,
		},
	})

	result := callToolOnSession(t, mcpSvr, sess, "loom_elicitation_response", map[string]interface{}{"action": "pause_epic"})
	require.False(t, result.IsError)

	var got mcp.ElicitationResponseResult
	require.NoError(t, json.Unmarshal([]byte(toolText(t, result)), &got))
	assert.Equal(t, "pause_epic", got.Action)
	assert.Equal(t, "PAUSED", got.NewState)

	cp, err := st.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "PAUSED", cp.State)
}

func TestLoomElicitationResponse_InvalidAction_ReturnsClearErrorAndKeepsActive(t *testing.T) {
	cfg := fsm.DefaultConfig()
	cfg.MaxRetriesAwaitingCI = 0
	machine := fsm.NewMachine(cfg)
	st := newMemStore()
	s := mcp.NewServer(machine, st, nil)
	mcpSvr := s.MCPServer()

	sess := prepareBudgetExhaustionElicitation(t, mcpSvr, st, map[string]interface{}{
		"experimental": map[string]interface{}{
			"elicitation": true,
		},
	})

	invalid := callToolOnSession(t, mcpSvr, sess, "loom_elicitation_response", map[string]interface{}{"action": "invalid_action"})
	require.True(t, invalid.IsError)
	errText := toolText(t, invalid)
	assert.Contains(t, errText, "valid choices are skip, reassign, pause_epic")

	next := callToolOnSession(t, mcpSvr, sess, "loom_elicitation_response", map[string]interface{}{"action": "pause_epic"})
	require.False(t, next.IsError, "active elicitation should remain available after invalid action")
}

func TestLoomCheckpoint_BudgetExhaustion_WithoutElicitationCapability_FallsBackToPaused(t *testing.T) {
	cfg := fsm.DefaultConfig()
	cfg.MaxRetriesAwaitingCI = 0
	machine := fsm.NewMachine(cfg)
	st := newMemStore()
	s := mcp.NewServer(machine, st, nil)
	mcpSvr := s.MCPServer()

	for _, action := range []string{"start", "phase_identified", "copilot_assigned", "pr_opened", "pr_ready"} {
		result := callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": action})
		require.False(t, result.IsError, "failed to advance with action %q: %s", action, toolText(t, result))
	}

	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{
		State:    string(fsm.StateAwaitingCI),
		PRNumber: 42,
	}))

	sess := newTestSession(nextSessionID())
	require.NoError(t, mcpSvr.RegisterSession(context.Background(), sess))
	initializeSessionWithCapabilities(t, mcpSvr, sess, map[string]interface{}{})

	result := callToolOnSession(t, mcpSvr, sess, "loom_checkpoint", map[string]interface{}{"action": "timeout"})
	require.False(t, result.IsError)

	var got mcp.CheckpointResult
	require.NoError(t, json.Unmarshal([]byte(toolText(t, result)), &got))
	assert.Equal(t, "PAUSED", got.NewState)
	assert.Empty(t, drainNotifications(sess), "fallback path must not emit elicitation notifications")
}

func TestLoomElicitationResponse_ReassignWithoutClosingClient_ReturnsClearError(t *testing.T) {
	cfg := fsm.DefaultConfig()
	cfg.MaxRetriesAwaitingCI = 0
	machine := fsm.NewMachine(cfg)
	st := newMemStore()
	s := mcp.NewServer(machine, st, nil)
	mcpSvr := s.MCPServer()

	sess := prepareBudgetExhaustionElicitation(t, mcpSvr, st, map[string]interface{}{
		"experimental": map[string]interface{}{
			"elicitation": true,
		},
	})

	result := callToolOnSession(t, mcpSvr, sess, "loom_elicitation_response", map[string]interface{}{"action": "reassign"})
	require.True(t, result.IsError)
	assert.Contains(t, toolText(t, result), "does not support PR closing")
}
