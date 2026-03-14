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

func TestLoomCheckpoint_BudgetExhaustion_EmitsElicitationAndDoesNotPause(t *testing.T) {
	cfg := fsm.DefaultConfig()
	cfg.MaxRetriesAwaitingCI = 0
	machine := fsm.NewMachine(cfg)
	st := newMemStore()
	s := mcp.NewServer(machine, st, nil)
	mcpSvr := s.MCPServer()

	callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": "start"})
	callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": "phase_identified"})
	callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": "copilot_assigned"})
	callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": "pr_opened"})
	callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": "pr_ready"})

	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{
		State:    string(fsm.StateAwaitingCI),
		PRNumber: 42,
	}))

	sess := newTestSession(nextSessionID())
	require.NoError(t, mcpSvr.RegisterSession(context.Background(), sess))
	initializeSessionWithCapabilities(t, mcpSvr, sess, map[string]interface{}{
		"experimental": map[string]interface{}{
			"elicitation": true,
		},
	})
	checkpointRes := callToolOnSession(t, mcpSvr, sess, "loom_checkpoint", map[string]interface{}{
		"action": "timeout",
	})
	require.False(t, checkpointRes.IsError)

	var got mcp.CheckpointResult
	require.NoError(t, json.Unmarshal([]byte(toolText(t, checkpointRes)), &got))
	assert.Equal(t, "AWAITING_CI", got.PreviousState)
	assert.Equal(t, "AWAITING_CI", got.NewState)

	notes := drainNotifications(sess)
	require.Len(t, notes, 1)
	note := notes[0]
	assert.Equal(t, "loom/elicitation", note.Method)
	assert.Equal(t, "elicitation", note.Params.AdditionalFields["type"])
	assert.Equal(t, "PR #42 — CI budget exhausted", note.Params.AdditionalFields["title"])
	_, hasDescription := note.Params.AdditionalFields["description"]
	assert.True(t, hasDescription)

	schemaAny, ok := note.Params.AdditionalFields["schema"]
	require.True(t, ok)
	schema, ok := schemaAny.(map[string]any)
	require.True(t, ok)

	actionAny, ok := schema["action"]
	require.True(t, ok)
	actionSchema, ok := actionAny.(map[string]any)
	require.True(t, ok)

	assert.Equal(t, "string", actionSchema["type"])

	enumValues, ok := actionSchema["enum"].([]string)
	require.True(t, ok)
	assert.Equal(t, []string{"skip", "reassign", "pause_epic"}, enumValues)

	enumDescriptions, ok := actionSchema["enumDescriptions"].([]string)
	require.True(t, ok)
	assert.Equal(t, []string{
		"Skip this user story and advance to the next",
		"Re-assign the PR to a fresh @copilot session",
		"Pause the epic and require human intervention",
	}, enumDescriptions)

	nextStepRes := callTool(t, mcpSvr, "loom_next_step", nil)
	require.False(t, nextStepRes.IsError)

	var next mcp.NextStepResult
	require.NoError(t, json.Unmarshal([]byte(toolText(t, nextStepRes)), &next))
	assert.Equal(t, "AWAITING_CI", next.State)

	cp, err := st.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "AWAITING_CI", cp.State)
}
