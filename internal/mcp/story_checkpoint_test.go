package mcp_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/guillaume7/loom/internal/fsm"
	"github.com/guillaume7/loom/internal/mcp"
	"github.com/guillaume7/loom/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoomCheckpoint_WithStoryID_WritesStoryScopedCheckpoint(t *testing.T) {
	st, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, st.Close()) })

	machine := fsm.NewMachine(fsm.DefaultConfig())
	s := mcp.NewServer(machine, st, nil, mcp.WithStoryID("TH2.E8.US2"))
	mcpSvr := s.MCPServer()

	result := callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{
		"action": "start",
	})
	require.False(t, result.IsError)

	sqliteStore, ok := st.(interface {
		ReadCheckpointByStoryID(context.Context, string) (store.Checkpoint, error)
	})
	require.True(t, ok)

	storyCP, err := sqliteStore.ReadCheckpointByStoryID(context.Background(), "TH2.E8.US2")
	require.NoError(t, err)
	assert.Equal(t, "TH2.E8.US2", storyCP.StoryID)
	assert.Equal(t, "SCANNING", storyCP.State)

	legacyCP, err := st.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, store.Checkpoint{}, legacyCP)
}

func TestLoomCheckpoint_WithStoryID_ScopesOperationKeysPerStory(t *testing.T) {
	st, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, st.Close()) })

	serverOne := mcp.NewServer(fsm.NewMachine(fsm.DefaultConfig()), st, nil, mcp.WithStoryID("TH2.E8.US2"))
	serverTwo := mcp.NewServer(fsm.NewMachine(fsm.DefaultConfig()), st, nil, mcp.WithStoryID("TH2.E8.US3"))

	first := callTool(t, serverOne.MCPServer(), "loom_checkpoint", map[string]interface{}{
		"action":        "start",
		"operation_key": "checkpoint:IDLE->SCANNING",
	})
	second := callTool(t, serverTwo.MCPServer(), "loom_checkpoint", map[string]interface{}{
		"action":        "start",
		"operation_key": "checkpoint:IDLE->SCANNING",
	})

	require.False(t, first.IsError)
	require.False(t, second.IsError)

	var gotFirst mcp.CheckpointResult
	var gotSecond mcp.CheckpointResult
	require.NoError(t, json.Unmarshal([]byte(toolText(t, first)), &gotFirst))
	require.NoError(t, json.Unmarshal([]byte(toolText(t, second)), &gotSecond))
	assert.Equal(t, "SCANNING", gotFirst.NewState)
	assert.Equal(t, "SCANNING", gotSecond.NewState)

	actions, err := st.ReadActions(context.Background(), 10)
	require.NoError(t, err)
	require.Len(t, actions, 2)
	assert.NotEqual(t, actions[0].OperationKey, actions[1].OperationKey)
}

func TestLoomAbort_WithStoryID_PausesStoryScopedCheckpoint(t *testing.T) {
	st, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, st.Close()) })

	scopedStore, ok := st.(interface {
		ReadCheckpointByStoryID(context.Context, string) (store.Checkpoint, error)
		WriteCheckpointByStoryID(context.Context, string, store.Checkpoint) error
	})
	require.True(t, ok)

	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{
		State:    "AWAITING_CI",
		Phase:    1,
		PRNumber: 7,
	}))
	require.NoError(t, scopedStore.WriteCheckpointByStoryID(context.Background(), "TH3.E1.US4", store.Checkpoint{
		State:    "AWAITING_PR",
		Phase:    2,
		PRNumber: 42,
	}))

	server := mcp.NewServer(fsm.NewMachine(fsm.DefaultConfig()), st, nil, mcp.WithStoryID("TH3.E1.US4"))
	result := callTool(t, server.MCPServer(), "loom_abort", nil)
	require.False(t, result.IsError)

	var got mcp.AbortResult
	require.NoError(t, json.Unmarshal([]byte(toolText(t, result)), &got))
	assert.Equal(t, "PAUSED", got.State)

	storyCP, err := scopedStore.ReadCheckpointByStoryID(context.Background(), "TH3.E1.US4")
	require.NoError(t, err)
	assert.Equal(t, "TH3.E1.US4", storyCP.StoryID)
	assert.Equal(t, "PAUSED", storyCP.State)
	assert.Equal(t, "AWAITING_PR", storyCP.ResumeState)
	assert.Equal(t, 42, storyCP.PRNumber)

	legacyCP, err := st.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "AWAITING_CI", legacyCP.State)
	assert.Empty(t, legacyCP.ResumeState)
	assert.Equal(t, 7, legacyCP.PRNumber)

	events, err := st.ReadExternalEvents(context.Background(), "TH3.E1.US4", 10)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "manual_override.pause", events[0].EventKind)

	decisions, err := st.ReadPolicyDecisions(context.Background(), "TH3.E1.US4", 10)
	require.NoError(t, err)
	require.Len(t, decisions, 1)
	assert.Equal(t, "pause", decisions[0].Verdict)
}

func TestLoomElicitationResponse_PauseEpic_WithStoryID_PausesStoryScopedCheckpoint(t *testing.T) {
	st, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, st.Close()) })

	scopedStore, ok := st.(interface {
		ReadCheckpointByStoryID(context.Context, string) (store.Checkpoint, error)
		WriteCheckpointByStoryID(context.Context, string, store.Checkpoint) error
	})
	require.True(t, ok)

	cfg := fsm.DefaultConfig()
	cfg.MaxRetriesAwaitingCI = 0
	server := mcp.NewServer(fsm.NewMachine(cfg), st, nil, mcp.WithStoryID("TH3.E1.US4"))
	mcpSvr := server.MCPServer()

	for _, action := range []string{"start", "phase_identified", "copilot_assigned", "pr_opened", "pr_ready"} {
		result := callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": action})
		require.False(t, result.IsError, "failed to advance with action %q: %s", action, toolText(t, result))
	}

	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{
		State:    "AWAITING_PR",
		Phase:    1,
		PRNumber: 7,
	}))
	require.NoError(t, scopedStore.WriteCheckpointByStoryID(context.Background(), "TH3.E1.US4", store.Checkpoint{
		State:    "AWAITING_CI",
		Phase:    3,
		PRNumber: 42,
	}))

	sess := newTestSession(nextSessionID())
	require.NoError(t, mcpSvr.RegisterSession(context.Background(), sess))
	initializeSessionWithCapabilities(t, mcpSvr, sess, map[string]interface{}{
		"experimental": map[string]interface{}{
			"elicitation": true,
		},
	})

	checkpointResult := callToolOnSession(t, mcpSvr, sess, "loom_checkpoint", map[string]interface{}{"action": "timeout"})
	require.False(t, checkpointResult.IsError)

	result := callToolOnSession(t, mcpSvr, sess, "loom_elicitation_response", map[string]interface{}{"action": "pause_epic"})
	require.False(t, result.IsError)

	var got mcp.ElicitationResponseResult
	require.NoError(t, json.Unmarshal([]byte(toolText(t, result)), &got))
	assert.Equal(t, "pause_epic", got.Action)
	assert.Equal(t, "PAUSED", got.NewState)

	storyCP, err := scopedStore.ReadCheckpointByStoryID(context.Background(), "TH3.E1.US4")
	require.NoError(t, err)
	assert.Equal(t, "TH3.E1.US4", storyCP.StoryID)
	assert.Equal(t, "PAUSED", storyCP.State)
	assert.Equal(t, "AWAITING_CI", storyCP.ResumeState)
	assert.Equal(t, 42, storyCP.PRNumber)

	legacyCP, err := st.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "AWAITING_PR", legacyCP.State)
	assert.Empty(t, legacyCP.ResumeState)
	assert.Equal(t, 7, legacyCP.PRNumber)

	events, err := st.ReadExternalEvents(context.Background(), "TH3.E1.US4", 10)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "operator", events[0].EventSource)
	assert.Equal(t, "manual_override.pause", events[0].EventKind)

	decisions, err := st.ReadPolicyDecisions(context.Background(), "TH3.E1.US4", 10)
	require.NoError(t, err)
	require.Len(t, decisions, 1)
	assert.Equal(t, "operator_override", decisions[0].DecisionKind)
	assert.Equal(t, "pause", decisions[0].Verdict)
	assert.Equal(t, events[0].CorrelationID, decisions[0].CorrelationID)

	defaultEvents, err := st.ReadExternalEvents(context.Background(), "default", 10)
	require.NoError(t, err)
	assert.Empty(t, defaultEvents)

	defaultDecisions, err := st.ReadPolicyDecisions(context.Background(), "default", 10)
	require.NoError(t, err)
	assert.Empty(t, defaultDecisions)
}
