package mcp_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/guillaume7/loom/internal/fsm"
	"github.com/guillaume7/loom/internal/mcp"
	"github.com/guillaume7/loom/internal/store"
	mcplib "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --------------------------------------------------------------------------
// In-memory store (test double)
// --------------------------------------------------------------------------

type memStore struct {
	mu    sync.Mutex
	cp    store.Checkpoint
	empty bool
}

func newMemStore() *memStore { return &memStore{empty: true} }

func (s *memStore) ReadCheckpoint(_ context.Context) (store.Checkpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.empty {
		return store.Checkpoint{}, nil
	}
	return s.cp, nil
}

func (s *memStore) WriteCheckpoint(_ context.Context, cp store.Checkpoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cp = cp
	s.empty = false
	return nil
}

// failingStore is a Store that always returns an error on WriteCheckpoint.
type failingStore struct {
	memStore
}

func newFailingStore() *failingStore { return &failingStore{memStore: memStore{empty: true}} }

func (s *failingStore) WriteCheckpoint(_ context.Context, _ store.Checkpoint) error {
	return errors.New("simulated store write failure")
}

// --------------------------------------------------------------------------
// Minimal ClientSession for test context wiring
// --------------------------------------------------------------------------

type testSession struct {
	id            string
	notifications chan mcplib.JSONRPCNotification
}

func newTestSession(id string) *testSession {
	return &testSession{
		id:            id,
		notifications: make(chan mcplib.JSONRPCNotification, 16),
	}
}

func (s *testSession) Initialize()                                            {}
func (s *testSession) Initialized() bool                                      { return true }
func (s *testSession) NotificationChannel() chan<- mcplib.JSONRPCNotification { return s.notifications }
func (s *testSession) SessionID() string                                      { return s.id }

var _ mcpserver.ClientSession = (*testSession)(nil)

// --------------------------------------------------------------------------
// Test helpers
// --------------------------------------------------------------------------

// sessionIDCounter generates unique session IDs so that concurrent
// RegisterSession calls within one test process never collide.
var sessionIDCounter atomic.Int64

func nextSessionID() string {
	return fmt.Sprintf("test-session-%d", sessionIDCounter.Add(1))
}

// newTestServer creates a fresh Server backed by a real FSM machine and an
// in-memory store. It returns both the Server and its MCPServer.
func newTestServer(t *testing.T) (*mcp.Server, *mcpserver.MCPServer) {
	t.Helper()
	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newMemStore()
	s := mcp.NewServer(machine, st, nil)
	return s, s.MCPServer()
}

// callTool sends a tools/call JSON-RPC message to mcpSvr and returns the
// resulting *mcplib.CallToolResult. Tool-level errors (IsError == true) are
// returned for the caller to assert on; protocol-level errors fail the test.
func callTool(t *testing.T, mcpSvr *mcpserver.MCPServer, toolName string, args map[string]interface{}) *mcplib.CallToolResult {
	t.Helper()

	if args == nil {
		args = map[string]interface{}{}
	}

	// Register a unique session and embed it in the context.
	ctx := context.Background()
	sess := newTestSession(nextSessionID())
	require.NoError(t, mcpSvr.RegisterSession(ctx, sess))
	ctx = mcpSvr.WithContext(ctx, sess)

	msg, err := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      toolName,
			"arguments": args,
		},
	})
	require.NoError(t, err)

	raw := mcpSvr.HandleMessage(ctx, msg)
	require.NotNil(t, raw, "HandleMessage returned nil for tool %q", toolName)

	resp, ok := raw.(mcplib.JSONRPCResponse)
	require.True(t, ok, "expected JSONRPCResponse, got %T", raw)

	result, ok := resp.Result.(mcplib.CallToolResult)
	require.True(t, ok, "expected CallToolResult in response.Result, got %T", resp.Result)

	return &result
}

// callToolConcurrent is a goroutine-safe variant of callTool that returns a
// bool instead of failing the test directly. It is suitable for use inside
// goroutines where require.* (which calls runtime.Goexit) must not be called.
func callToolConcurrent(mcpSvr *mcpserver.MCPServer, toolName string, args map[string]interface{}) bool {
	if args == nil {
		args = map[string]interface{}{}
	}

	ctx := context.Background()
	sess := newTestSession(nextSessionID())
	if err := mcpSvr.RegisterSession(ctx, sess); err != nil {
		return false
	}
	ctx = mcpSvr.WithContext(ctx, sess)

	msg, err := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      toolName,
			"arguments": args,
		},
	})
	if err != nil {
		return false
	}

	raw := mcpSvr.HandleMessage(ctx, msg)
	if raw == nil {
		return false
	}
	resp, ok := raw.(mcplib.JSONRPCResponse)
	if !ok {
		return false
	}
	_, ok = resp.Result.(mcplib.CallToolResult)
	return ok
}

// toolText returns the text of the first TextContent item in result.
func toolText(t *testing.T, result *mcplib.CallToolResult) string {
	t.Helper()
	require.NotEmpty(t, result.Content, "CallToolResult has no content")
	tc, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok, "expected TextContent, got %T", result.Content[0])
	return tc.Text
}

// --------------------------------------------------------------------------
// Tests
// --------------------------------------------------------------------------

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

	require.Len(t, result.Tools, 5, "expected exactly 5 tools registered")

	var names []string
	for _, tool := range result.Tools {
		names = append(names, tool.Name)
		assert.NotEmpty(t, tool.InputSchema.Type, "tool %q has empty InputSchema.Type", tool.Name)
	}

	assert.ElementsMatch(t, []string{
		"loom_next_step",
		"loom_checkpoint",
		"loom_heartbeat",
		"loom_get_state",
		"loom_abort",
	}, names)
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

func TestLoomHeartbeat_ReturnsCurrentState(t *testing.T) {
	_, mcpSvr := newTestServer(t)

	result := callTool(t, mcpSvr, "loom_heartbeat", nil)
	assert.False(t, result.IsError)

	var got mcp.HeartbeatResult
	require.NoError(t, json.Unmarshal([]byte(toolText(t, result)), &got))

	assert.Equal(t, "IDLE", got.State)
}

func TestLoomGetState_ReturnsState(t *testing.T) {
	_, mcpSvr := newTestServer(t)

	result := callTool(t, mcpSvr, "loom_get_state", nil)
	assert.False(t, result.IsError)

	var got mcp.GetStateResult
	require.NoError(t, json.Unmarshal([]byte(toolText(t, result)), &got))

	assert.NotEmpty(t, got.State)
}

func TestLoomAbort_TransitionsToPaused(t *testing.T) {
	_, mcpSvr := newTestServer(t)

	result := callTool(t, mcpSvr, "loom_abort", nil)
	assert.False(t, result.IsError)

	var got mcp.AbortResult
	require.NoError(t, json.Unmarshal([]byte(toolText(t, result)), &got))

	assert.Equal(t, "PAUSED", got.State)
}

func TestServer_RaceCondition(t *testing.T) {
	// Verify that concurrent loom_heartbeat / loom_get_state calls do not
	// trigger the data-race detector. No state transitions occur so only
	// read paths are exercised.
	//
	// callToolConcurrent is used instead of callTool because require.*
	// calls runtime.Goexit which must only be invoked from the test goroutine.
	_, mcpSvr := newTestServer(t)

	const goroutines = 8
	const callsEach = 5

	results := make(chan bool, goroutines*callsEach)

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		g := g
		go func() {
			defer wg.Done()
			for j := 0; j < callsEach; j++ {
				var ok bool
				if (g+j)%2 == 0 {
					ok = callToolConcurrent(mcpSvr, "loom_heartbeat", nil)
				} else {
					ok = callToolConcurrent(mcpSvr, "loom_get_state", nil)
				}
				results <- ok
			}
		}()
	}

	wg.Wait()
	close(results)

	for ok := range results {
		assert.True(t, ok, "concurrent tool call returned unexpected result")
	}
}
