package mcp_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/guillaume7/loom/internal/fsm"
	"github.com/guillaume7/loom/internal/mcp"
	"github.com/guillaume7/loom/internal/store"
	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type reviewRequestGitHubClientMock struct {
	requestedPRs []int
	err          error
}

func (m *reviewRequestGitHubClientMock) Ping(context.Context) error { return nil }

func (m *reviewRequestGitHubClientMock) RequestReview(_ context.Context, prNumber int, reviewer string) error {
	if m.err != nil {
		return m.err
	}
	if reviewer != "copilot" {
		return errors.New("unexpected reviewer")
	}
	m.requestedPRs = append(m.requestedPRs, prNumber)
	return nil
}

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

	require.Len(t, result.Tools, 8, "expected exactly 8 tools registered")

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
		"loom_schedule_epic",
		"loom_get_state",
		"loom_abort",
		"loom_spawn_agent",
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
	assertReadOnlyHint("loom_schedule_epic", false)
	assertReadOnlyHint("loom_abort", false)
	assertReadOnlyHint("loom_spawn_agent", false)
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

func TestNewServer_HydratesMachineFromCheckpoint(t *testing.T) {
	st := newMemStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{
		State: "AWAITING_READY",
		Phase: 3,
	}))

	s := mcp.NewServer(fsm.NewMachine(fsm.DefaultConfig()), st, nil)
	mcpSvr := s.MCPServer()

	result := callTool(t, mcpSvr, "loom_next_step", nil)
	assert.False(t, result.IsError)

	var got mcp.NextStepResult
	require.NoError(t, json.Unmarshal([]byte(toolText(t, result)), &got))
	assert.Equal(t, "AWAITING_READY", got.State)
}

func TestLoomNextStep_RehydratesFromCheckpointAfterOutOfBandChange(t *testing.T) {
	st := newMemStore()
	s := mcp.NewServer(fsm.NewMachine(fsm.DefaultConfig()), st, nil)
	mcpSvr := s.MCPServer()

	callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": "start"})
	callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": "phase_identified"})
	callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": "copilot_assigned"})
	callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": "pr_opened"})

	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "AWAITING_CI", Phase: 3}))

	result := callTool(t, mcpSvr, "loom_next_step", nil)
	assert.False(t, result.IsError)

	var got mcp.NextStepResult
	require.NoError(t, json.Unmarshal([]byte(toolText(t, result)), &got))
	assert.Equal(t, "AWAITING_CI", got.State)
	assert.Contains(t, got.Instruction, "ci_green")
}

func TestLoomGetState_RehydratesFromCheckpointAfterOutOfBandChange(t *testing.T) {
	st := newMemStore()
	s := mcp.NewServer(fsm.NewMachine(fsm.DefaultConfig()), st, nil)
	mcpSvr := s.MCPServer()

	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "AWAITING_READY", Phase: 4}))

	result := callTool(t, mcpSvr, "loom_get_state", nil)
	assert.False(t, result.IsError)

	var got mcp.GetStateResult
	require.NoError(t, json.Unmarshal([]byte(toolText(t, result)), &got))
	assert.Equal(t, "AWAITING_READY", got.State)
	assert.Equal(t, 4, got.Phase)
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

func TestLoomCheckpoint_CIGreen_RequestsCopilotReview(t *testing.T) {
	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newMemStore()
	reviewer := &reviewRequestGitHubClientMock{}
	s := mcp.NewServer(machine, st, reviewer)
	mcpSvr := s.MCPServer()

	for _, action := range []string{"start", "phase_identified", "copilot_assigned", "pr_opened", "pr_ready"} {
		result := callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": action})
		require.False(t, result.IsError, "failed to advance with action %q: %s", action, toolText(t, result))
	}
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "AWAITING_CI", PRNumber: 42, Phase: 3}))

	result := callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": "ci_green"})
	require.False(t, result.IsError)

	var got mcp.CheckpointResult
	require.NoError(t, json.Unmarshal([]byte(toolText(t, result)), &got))
	assert.Equal(t, "AWAITING_CI", got.PreviousState)
	assert.Equal(t, "REVIEWING", got.NewState)
	require.Len(t, reviewer.requestedPRs, 1)
	assert.Equal(t, 42, reviewer.requestedPRs[0])

	cp, err := st.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "REVIEWING", cp.State)
	assert.Equal(t, 42, cp.PRNumber)
}

func TestLoomCheckpoint_CIGreen_RequestReviewFailure_RollsBackState(t *testing.T) {
	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newMemStore()
	reviewer := &reviewRequestGitHubClientMock{err: errors.New("review API down")}
	s := mcp.NewServer(machine, st, reviewer)
	mcpSvr := s.MCPServer()

	for _, action := range []string{"start", "phase_identified", "copilot_assigned", "pr_opened", "pr_ready"} {
		result := callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": action})
		require.False(t, result.IsError, "failed to advance with action %q: %s", action, toolText(t, result))
	}
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "AWAITING_CI", PRNumber: 42, Phase: 3}))

	result := callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": "ci_green"})
	require.True(t, result.IsError)
	assert.Contains(t, toolText(t, result), "failed to request Copilot review")

	cp, err := st.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "AWAITING_CI", cp.State)
	assert.Equal(t, 42, cp.PRNumber)
	assert.Equal(t, fsm.StateAwaitingCI, machine.State())
	assert.Empty(t, reviewer.requestedPRs)
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
