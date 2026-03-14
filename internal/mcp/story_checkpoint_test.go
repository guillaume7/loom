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
