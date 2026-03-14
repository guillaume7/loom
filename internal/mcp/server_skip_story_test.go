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

func TestLoomCheckpoint_SkipStory_FromAwaitingCI_AdvancesPhaseResetsRetryAndPersistsDetail(t *testing.T) {
	s, mcpSvr := newTestServer(t)
	st := s.Store().(*memStore)

	for _, action := range []string{"start", "phase_identified", "copilot_assigned", "pr_opened", "pr_ready"} {
		result := callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": action})
		require.False(t, result.IsError, "failed to advance with action %q: %s", action, toolText(t, result))
	}

	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{
		State:      string(fsm.StateAwaitingCI),
		Phase:      7,
		RetryCount: 5,
	}))

	result := callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{
		"action":        "skip_story",
		"operation_key": "checkpoint:AWAITING_CI->SCANNING:skip_story",
	})
	require.False(t, result.IsError)

	var got mcp.CheckpointResult
	require.NoError(t, json.Unmarshal([]byte(toolText(t, result)), &got))
	assert.Equal(t, "AWAITING_CI", got.PreviousState)
	assert.Equal(t, "SCANNING", got.NewState)
	assert.Equal(t, 8, got.Phase)
	assert.Equal(t, "skipped story at phase 7", got.Detail)

	cp, err := st.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "SCANNING", cp.State)
	assert.Equal(t, 8, cp.Phase)
	assert.Equal(t, 0, cp.RetryCount)

	action, err := st.ReadActionByOperationKey(context.Background(), "checkpoint:AWAITING_CI->SCANNING:skip_story")
	require.NoError(t, err)
	assert.Equal(t, "skip_story", action.Event)
	assert.Contains(t, action.Detail, "\"detail\":\"skipped story at phase 7\"")
}

func TestLoomCheckpoint_SkipStory_FromDebugging_AdvancesPhaseAndResetsRetry(t *testing.T) {
	s, mcpSvr := newTestServer(t)
	st := s.Store().(*memStore)

	for _, action := range []string{"start", "phase_identified", "copilot_assigned", "pr_opened", "pr_ready", "ci_red"} {
		result := callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": action})
		require.False(t, result.IsError, "failed to advance with action %q: %s", action, toolText(t, result))
	}

	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{
		State:      string(fsm.StateDebugging),
		Phase:      2,
		RetryCount: 9,
	}))

	result := callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": "skip_story"})
	require.False(t, result.IsError)

	var got mcp.CheckpointResult
	require.NoError(t, json.Unmarshal([]byte(toolText(t, result)), &got))
	assert.Equal(t, "DEBUGGING", got.PreviousState)
	assert.Equal(t, "SCANNING", got.NewState)
	assert.Equal(t, 3, got.Phase)
	assert.Equal(t, "skipped story at phase 2", got.Detail)

	cp, err := st.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "SCANNING", cp.State)
	assert.Equal(t, 3, cp.Phase)
	assert.Equal(t, 0, cp.RetryCount)
}

func TestLoomCheckpoint_SkipStory_InvalidFromMerging(t *testing.T) {
	_, mcpSvr := newTestServer(t)

	for _, action := range []string{"start", "phase_identified", "copilot_assigned", "pr_opened", "pr_ready", "ci_green", "review_approved"} {
		result := callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": action})
		require.False(t, result.IsError, "failed to advance with action %q: %s", action, toolText(t, result))
	}

	result := callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": "skip_story"})
	assert.True(t, result.IsError)
	assert.Contains(t, toolText(t, result), "MERGING")
}
