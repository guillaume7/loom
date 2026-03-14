package mcp_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/guillaume7/loom/internal/fsm"
	"github.com/guillaume7/loom/internal/mcp"
	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewServer_ReturnsNonNil(t *testing.T) {
	s := mcp.NewServer(fsm.NewMachine(fsm.DefaultConfig()), newMemStore(), nil)
	assert.NotNil(t, s)
}

func TestToolsList_RegistersAllToolsWithSchemas(t *testing.T) {
	_, mcpSvr := newTestServer(t)

	ctx := context.Background()
	sess := newTestSession(nextSessionID())
	require.NoError(t, mcpSvr.RegisterSession(ctx, sess))
	ctx = mcpSvr.WithContext(ctx, sess)

	msg, err := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
		"params":  map[string]interface{}{},
	})
	require.NoError(t, err)

	raw := mcpSvr.HandleMessage(ctx, msg)
	require.NotNil(t, raw)

	resp, ok := raw.(mcplib.JSONRPCResponse)
	require.True(t, ok, "expected JSONRPCResponse, got %T", raw)

	result, ok := resp.Result.(mcplib.ListToolsResult)
	require.True(t, ok, "expected ListToolsResult, got %T", resp.Result)

	require.Len(t, result.Tools, 6, "expected exactly 6 tools registered")

	var names []string
	toolsByName := make(map[string]mcplib.Tool, len(result.Tools))
	for _, tool := range result.Tools {
		names = append(names, tool.Name)
		toolsByName[tool.Name] = tool
		assert.NotEmpty(t, tool.InputSchema.Type, "tool %q has empty InputSchema.Type", tool.Name)
	}

	assert.ElementsMatch(t, []string{
		"loom_next_step",
		"loom_checkpoint",
		"loom_heartbeat",
		"loom_elicitation_response",
		"loom_get_state",
		"loom_abort",
	}, names)

	assertReadOnlyHint := func(toolName string, expected bool) {
		tool, ok := toolsByName[toolName]
		require.True(t, ok, "expected tool %q in tools/list", toolName)
		require.NotNil(t, tool.Annotations.ReadOnlyHint, "tool %q missing annotations.readOnlyHint", toolName)
		assert.Equal(t, expected, *tool.Annotations.ReadOnlyHint, "unexpected annotations.readOnlyHint for %q", toolName)
	}

	assertReadOnlyHint("loom_get_state", true)
	assertReadOnlyHint("loom_heartbeat", true)
	assertReadOnlyHint("loom_next_step", false)
	assertReadOnlyHint("loom_checkpoint", false)
	assertReadOnlyHint("loom_abort", false)
}

func TestLoomNextStep_ReturnsStateAndInstruction(t *testing.T) {
	_, mcpSvr := newTestServer(t)

	result := callTool(t, mcpSvr, "loom_next_step", nil)
	assert.False(t, result.IsError)

	var got mcp.NextStepResult
	require.NoError(t, json.Unmarshal([]byte(toolText(t, result)), &got))

	assert.Equal(t, "IDLE", got.State)
	assert.NotEmpty(t, got.Instruction)
}

func TestLoomCheckpoint_ValidAction_AdvancesState(t *testing.T) {
	_, mcpSvr := newTestServer(t)

	result := callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{
		"action": "start",
	})
	assert.False(t, result.IsError)

	var got mcp.CheckpointResult
	require.NoError(t, json.Unmarshal([]byte(toolText(t, result)), &got))

	assert.Equal(t, "IDLE", got.PreviousState)
	assert.Equal(t, "SCANNING", got.NewState)
}

func TestLoomCheckpoint_BackwardCompatEvent_AdvancesState(t *testing.T) {
	_, mcpSvr := newTestServer(t)

	// "event" field accepted for backward compatibility when "action" is absent.
	result := callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{
		"event": "start",
	})
	assert.False(t, result.IsError)

	var got mcp.CheckpointResult
	require.NoError(t, json.Unmarshal([]byte(toolText(t, result)), &got))

	assert.Equal(t, "IDLE", got.PreviousState)
	assert.Equal(t, "SCANNING", got.NewState)
}

func TestLoomCheckpoint_InvalidAction_ReturnsError(t *testing.T) {
	_, mcpSvr := newTestServer(t)

	result := callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{
		"action": "not_a_real_event",
	})
	assert.True(t, result.IsError)
}

func TestLoomCheckpoint_MissingAction_ReturnsError(t *testing.T) {
	_, mcpSvr := newTestServer(t)

	// Empty arguments map — no "action" or "event" key.
	result := callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{})
	assert.True(t, result.IsError)
}

func TestLoomCheckpoint_StoreWriteFailure_ReturnsError(t *testing.T) {
	// Use a server backed by a store that always fails on writes.
	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newFailingStore()
	s := mcp.NewServer(machine, st, nil)
	mcpSvr := s.MCPServer()

	result := callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{
		"action": "start",
	})
	assert.True(t, result.IsError, "expected tool error when store write fails")
}

func TestLoomCheckpoint_NonIdempotentStoreWriteFailure_RollsBackFSM(t *testing.T) {
	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newFailingStore()
	s := mcp.NewServer(machine, st, nil)
	mcpSvr := s.MCPServer()

	result := callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{
		"action": "start",
	})
	assert.True(t, result.IsError, "expected tool error when store write fails")

	next := callTool(t, mcpSvr, "loom_next_step", nil)
	assert.False(t, next.IsError)

	var got mcp.NextStepResult
	require.NoError(t, json.Unmarshal([]byte(toolText(t, next)), &got))
	assert.Equal(t, "IDLE", got.State, "FSM must remain at pre-transition state after non-idempotent write failure")
}

func TestLoomNextStep_Idempotency_RetryReturnsCachedResult(t *testing.T) {
	s, mcpSvr := newTestServer(t)
	st := s.Store().(*memStore)

	first := callTool(t, mcpSvr, "loom_next_step", map[string]interface{}{
		"operation_key": "next_step:IDLE",
	})
	assert.False(t, first.IsError)

	second := callTool(t, mcpSvr, "loom_next_step", map[string]interface{}{
		"operation_key": "next_step:IDLE",
	})
	assert.False(t, second.IsError)
	assert.Equal(t, toolText(t, first), toolText(t, second))

	st.mu.Lock()
	defer st.mu.Unlock()
	require.Len(t, st.actions, 1)
	assert.Equal(t, "next_step:IDLE", st.actions[0].OperationKey)
	assert.Equal(t, "next_step", st.actions[0].Event)
}

func TestLoomCheckpoint_Idempotency_FirstExecutionLogsAction(t *testing.T) {
	s, mcpSvr := newTestServer(t)
	st := s.Store().(*memStore)

	result := callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{
		"action":        "start",
		"operation_key": "checkpoint:IDLE->SCANNING",
	})
	assert.False(t, result.IsError)

	var got mcp.CheckpointResult
	require.NoError(t, json.Unmarshal([]byte(toolText(t, result)), &got))
	assert.Equal(t, "IDLE", got.PreviousState)
	assert.Equal(t, "SCANNING", got.NewState)

	action, err := st.ReadActionByOperationKey(context.Background(), "checkpoint:IDLE->SCANNING")
	require.NoError(t, err)
	assert.Equal(t, "IDLE", action.StateBefore)
	assert.Equal(t, "SCANNING", action.StateAfter)
	assert.Equal(t, "start", action.Event)
	assert.Equal(t, toolText(t, result), action.Detail)
}

func TestLoomCheckpoint_Idempotency_RetryReturnsCachedResult(t *testing.T) {
	_, mcpSvr := newTestServer(t)
	args := map[string]interface{}{
		"action":        "start",
		"operation_key": "checkpoint:IDLE->SCANNING",
	}

	first := callTool(t, mcpSvr, "loom_checkpoint", args)
	assert.False(t, first.IsError)

	second := callTool(t, mcpSvr, "loom_checkpoint", args)
	assert.False(t, second.IsError)
	assert.Equal(t, toolText(t, first), toolText(t, second))
}

func TestLoomCheckpoint_Idempotency_DifferentOperationKeyExecutes(t *testing.T) {
	s, mcpSvr := newTestServer(t)
	st := s.Store().(*memStore)

	first := callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{
		"action":        "start",
		"operation_key": "checkpoint:IDLE->SCANNING",
	})
	assert.False(t, first.IsError)

	second := callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{
		"action":        "phase_identified",
		"operation_key": "checkpoint:SCANNING->ISSUE_CREATED",
	})
	assert.False(t, second.IsError)

	var got mcp.CheckpointResult
	require.NoError(t, json.Unmarshal([]byte(toolText(t, second)), &got))
	assert.Equal(t, "SCANNING", got.PreviousState)
	assert.Equal(t, "ISSUE_CREATED", got.NewState)

	st.mu.Lock()
	defer st.mu.Unlock()
	require.Len(t, st.actions, 2)
	assert.Equal(t, "checkpoint:IDLE->SCANNING", st.actions[0].OperationKey)
	assert.Equal(t, "checkpoint:SCANNING->ISSUE_CREATED", st.actions[1].OperationKey)
}
