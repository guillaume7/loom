package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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
	mu                    sync.Mutex
	cp                    store.Checkpoint
	actions               []store.Action
	empty                 bool
	readActionLookupCalls int
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

func (s *memStore) WriteAction(_ context.Context, action store.Action) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.actions {
		if existing.OperationKey == action.OperationKey {
			return store.ErrDuplicateOperationKey
		}
	}
	if action.CreatedAt.IsZero() {
		action.CreatedAt = time.Now().UTC()
	}
	action.ID = int64(len(s.actions) + 1)
	s.actions = append(s.actions, action)
	return nil
}

func (s *memStore) WriteCheckpointAndAction(_ context.Context, cp store.Checkpoint, action store.Action) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.actions {
		if existing.OperationKey == action.OperationKey {
			return store.ErrDuplicateOperationKey
		}
	}
	s.cp = cp
	s.empty = false
	if action.CreatedAt.IsZero() {
		action.CreatedAt = time.Now().UTC()
	}
	action.ID = int64(len(s.actions) + 1)
	s.actions = append(s.actions, action)
	return nil
}

func (s *memStore) ReadActionByOperationKey(_ context.Context, operationKey string) (store.Action, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.readActionLookupCalls++
	for _, action := range s.actions {
		if action.OperationKey == operationKey {
			return action, nil
		}
	}
	return store.Action{}, store.ErrActionNotFound
}

func (s *memStore) ReadActions(_ context.Context, limit int) ([]store.Action, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 || len(s.actions) == 0 {
		return []store.Action{}, nil
	}
	if limit > len(s.actions) {
		limit = len(s.actions)
	}
	actions := make([]store.Action, 0, limit)
	for index := len(s.actions) - 1; index >= len(s.actions)-limit; index-- {
		actions = append(actions, s.actions[index])
	}
	return actions, nil
}

func (s *memStore) DeleteAll(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cp = store.Checkpoint{}
	s.actions = nil
	s.empty = true
	return nil
}

func (s *memStore) Close() error { return nil }

// failingStore is a Store that always returns an error on WriteCheckpoint.
type failingStore struct {
	memStore
}

func newFailingStore() *failingStore { return &failingStore{memStore: memStore{empty: true}} }

func (s *failingStore) WriteCheckpoint(_ context.Context, _ store.Checkpoint) error {
	return errors.New("simulated store write failure")
}

func (s *failingStore) WriteCheckpointAndAction(_ context.Context, _ store.Checkpoint, _ store.Action) error {
	return errors.New("simulated store write failure")
}

func (s *failingStore) Close() error { return nil }

// duplicateWithoutCachedResultStore reports duplicate operation keys for
// idempotent checkpoint writes but never returns a cached action.
type duplicateWithoutCachedResultStore struct {
	memStore
}

func newDuplicateWithoutCachedResultStore() *duplicateWithoutCachedResultStore {
	return &duplicateWithoutCachedResultStore{memStore: memStore{empty: true}}
}

func (s *duplicateWithoutCachedResultStore) WriteCheckpointAndAction(_ context.Context, _ store.Checkpoint, _ store.Action) error {
	return store.ErrDuplicateOperationKey
}

func (s *duplicateWithoutCachedResultStore) ReadActionByOperationKey(_ context.Context, _ string) (store.Action, error) {
	return store.Action{}, store.ErrActionNotFound
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

// --------------------------------------------------------------------------
// Atomicity failure-window tests (TH2.E2.US3 fix)
// --------------------------------------------------------------------------

// transientFailStore fails WriteCheckpointAndAction on the first call only,
// then delegates to the underlying memStore. This simulates a transient I/O
// error that leaves no partial writes in the store.
type transientFailStore struct {
	memStore
	failed bool
}

func newTransientFailStore() *transientFailStore {
	return &transientFailStore{memStore: memStore{empty: true}}
}

func (s *transientFailStore) WriteCheckpointAndAction(ctx context.Context, cp store.Checkpoint, action store.Action) error {
	s.memStore.mu.Lock()
	if !s.failed {
		s.failed = true
		s.memStore.mu.Unlock()
		return errors.New("simulated transient write failure")
	}
	s.memStore.mu.Unlock()
	return s.memStore.WriteCheckpointAndAction(ctx, cp, action)
}

// TestLoomCheckpoint_AtomicWriteFailure_LeavesStoreConsistent verifies that
// when WriteCheckpointAndAction fails, neither the checkpoint nor the action
// log entry is persisted — eliminating the partial-write window.
func TestLoomCheckpoint_AtomicWriteFailure_LeavesStoreConsistent(t *testing.T) {
	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newTransientFailStore()
	s := mcp.NewServer(machine, st, nil)
	mcpSvr := s.MCPServer()

	// First call: WriteCheckpointAndAction fails.
	result := callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{
		"action":        "start",
		"operation_key": "checkpoint:IDLE->SCANNING:attempt1",
	})
	assert.True(t, result.IsError, "expected tool error on first attempt")

	// After the failure the store must be pristine: no checkpoint, no action entry.
	cp, err := st.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Empty(t, cp.State, "checkpoint must not be persisted after failed WriteCheckpointAndAction")

	_, lookupErr := st.ReadActionByOperationKey(context.Background(), "checkpoint:IDLE->SCANNING:attempt1")
	assert.ErrorIs(t, lookupErr, store.ErrActionNotFound,
		"action log entry must not be persisted after failed WriteCheckpointAndAction")
}

// TestLoomCheckpoint_AtomicWriteFailure_StoreWriteFailure_WithOperationKey verifies
// that a store write failure on the idempotent path (with operation_key) returns an
// error, consistent with the non-idempotent path behaviour.
func TestLoomCheckpoint_AtomicWriteFailure_StoreWriteFailure_WithOperationKey(t *testing.T) {
	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newFailingStore()
	s := mcp.NewServer(machine, st, nil)
	mcpSvr := s.MCPServer()

	result := callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{
		"action":        "start",
		"operation_key": "checkpoint:IDLE->SCANNING",
	})
	assert.True(t, result.IsError, "expected tool error when store write fails on idempotent path")
}

// TestLoomCheckpoint_SameProcessRetry_AfterTransientWriteFailure verifies the
// rollback property: when WriteCheckpointAndAction fails with a non-duplicate
// error, the in-memory FSM is rolled back to its pre-transition state so that
// a same-process retry with the same operation_key can successfully fire the
// event again and persist the checkpoint.
//
// Without a rollback the FSM would be stuck at newState after the first
// failure, causing the second attempt to fail with an invalid-transition error.
func TestLoomCheckpoint_SameProcessRetry_AfterTransientWriteFailure(t *testing.T) {
	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newTransientFailStore()
	s := mcp.NewServer(machine, st, nil)
	mcpSvr := s.MCPServer()

	args := map[string]interface{}{
		"action":        "start",
		"operation_key": "checkpoint:IDLE->SCANNING",
	}

	// First call: WriteCheckpointAndAction fails → FSM must be rolled back.
	result1 := callTool(t, mcpSvr, "loom_checkpoint", args)
	assert.True(t, result1.IsError, "expected tool error on first attempt (transient failure)")

	// Verify FSM was rolled back: store still empty, no action persisted.
	cp, err := st.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Empty(t, cp.State, "store checkpoint must remain empty after rollback")

	// Second call: same operation_key, same server process, store now succeeds.
	// The FSM must be at IDLE again (rolled back) so 'start' fires correctly.
	result2 := callTool(t, mcpSvr, "loom_checkpoint", args)
	assert.False(t, result2.IsError, "expected success on retry after rollback")

	var got mcp.CheckpointResult
	require.NoError(t, json.Unmarshal([]byte(toolText(t, result2)), &got))
	assert.Equal(t, "IDLE", got.PreviousState, "retry must fire from rolled-back IDLE state")
	assert.Equal(t, "SCANNING", got.NewState)

	// Verify the action was committed on the successful retry.
	action, lookupErr := st.ReadActionByOperationKey(context.Background(), "checkpoint:IDLE->SCANNING")
	require.NoError(t, lookupErr)
	assert.Equal(t, "IDLE", action.StateBefore)
	assert.Equal(t, "SCANNING", action.StateAfter)
}

// TestLoomCheckpoint_SameProcessRetry_CountersRolledBack verifies that retry
// counters (e.g. awaitingPRRetries) are also restored on rollback, preventing
// premature budget exhaustion when a timeout self-loop write fails transiently.
func TestLoomCheckpoint_SameProcessRetry_CountersRolledBack(t *testing.T) {
	// Build a machine already at AWAITING_PR so we can test timeout self-loops.
	m := fsm.NewMachine(fsm.DefaultConfig())
	if _, err := m.Transition(fsm.EventStart); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Transition(fsm.EventPhaseIdentified); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Transition(fsm.EventCopilotAssigned); err != nil {
		t.Fatal(err)
	}

	st := newTransientFailStore()
	s := mcp.NewServer(m, st, nil)
	mcpSvr := s.MCPServer()

	timeoutArgs := map[string]interface{}{
		"action":        "timeout",
		"operation_key": "checkpoint:AWAITING_PR->AWAITING_PR:t1",
	}

	// First call: write fails → rollback → awaitingPRRetries must be 0 again.
	r1 := callTool(t, mcpSvr, "loom_checkpoint", timeoutArgs)
	assert.True(t, r1.IsError, "expected error on first attempt")

	// Second call: write succeeds this time (transient store only fails once).
	r2 := callTool(t, mcpSvr, "loom_checkpoint", timeoutArgs)
	assert.False(t, r2.IsError, "expected success on retry")

	var got mcp.CheckpointResult
	require.NoError(t, json.Unmarshal([]byte(toolText(t, r2)), &got))
	// AWAITING_PR --timeout--> AWAITING_PR (within budget)
	assert.Equal(t, "AWAITING_PR", got.PreviousState)
	assert.Equal(t, "AWAITING_PR", got.NewState)
}

// duplicateOnWriteStore allows ReadActionByOperationKey to succeed after the
// first WriteCheckpointAndAction commits, then refuses the second write to
// simulate a concurrent duplicate.
type duplicateOnWriteStore struct {
	memStore
	firstDone bool
}

func newDuplicateOnWriteStore() *duplicateOnWriteStore {
	return &duplicateOnWriteStore{memStore: memStore{empty: true}}
}

func (s *duplicateOnWriteStore) WriteCheckpointAndAction(ctx context.Context, cp store.Checkpoint, action store.Action) error {
	s.memStore.mu.Lock()
	defer s.memStore.mu.Unlock()
	if s.firstDone {
		return store.ErrDuplicateOperationKey
	}
	// Commit both writes using the internal (unlocked) logic.
	for _, existing := range s.memStore.actions {
		if existing.OperationKey == action.OperationKey {
			return store.ErrDuplicateOperationKey
		}
	}
	s.memStore.cp = cp
	s.memStore.empty = false
	if action.CreatedAt.IsZero() {
		action.CreatedAt = time.Now().UTC()
	}
	action.ID = int64(len(s.memStore.actions) + 1)
	s.memStore.actions = append(s.memStore.actions, action)
	s.firstDone = true
	return nil
}

// TestLoomCheckpoint_AtomicDuplicateOnWrite_ReturnsCachedResult verifies that
// when WriteCheckpointAndAction returns ErrDuplicateOperationKey (concurrent
// duplicate), the handler looks up the previously committed result and returns
// it successfully — safe replay without a double transition.
func TestLoomCheckpoint_AtomicDuplicateOnWrite_ReturnsCachedResult(t *testing.T) {
	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newDuplicateOnWriteStore()
	s := mcp.NewServer(machine, st, nil)
	mcpSvr := s.MCPServer()

	first := callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{
		"action":        "start",
		"operation_key": "checkpoint:IDLE->SCANNING",
	})
	require.False(t, first.IsError, "first call must succeed")

	// Now the initial ReadActionByOperationKey lookup won't find the entry
	// (simulating a race where the lookup happens before the other goroutine
	// committed), but WriteCheckpointAndAction returns ErrDuplicateOperationKey.
	// We reset firstDone so the next outer lookup succeeds but the write is
	// rejected — exercising the ErrDuplicateOperationKey branch.
	// To do this properly we need to call with a *new* server that hasn't
	// cached the lookup, but the same store that already has the action.
	machine2 := fsm.NewMachine(fsm.DefaultConfig())
	s2 := mcp.NewServer(machine2, st, nil)
	mcpSvr2 := s2.MCPServer()

	// Reset firstDone so the idempotent lookup is bypassed (cold server),
	// but the write is rejected as a duplicate.
	st.firstDone = false
	// Write the action so the second lookup finds it after ErrDuplicateOperationKey.
	st.firstDone = true // next write will return ErrDuplicateOperationKey

	second := callTool(t, mcpSvr2, "loom_checkpoint", map[string]interface{}{
		"action":        "start",
		"operation_key": "checkpoint:IDLE->SCANNING",
	})
	require.False(t, second.IsError, "second call must succeed via ErrDuplicateOperationKey replay")
	assert.Equal(t, toolText(t, first), toolText(t, second), "replayed result must match original")
}

func TestLoomCheckpoint_IdempotentDuplicateWithoutCachedResult_RollsBackFSM(t *testing.T) {
	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newDuplicateWithoutCachedResultStore()
	s := mcp.NewServer(machine, st, nil)
	mcpSvr := s.MCPServer()

	result := callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{
		"action":        "start",
		"operation_key": "checkpoint:IDLE->SCANNING",
	})
	assert.True(t, result.IsError, "expected tool error for duplicate without cached result")
	assert.Contains(t, toolText(t, result), "duplicate operation key without cached result")

	next := callTool(t, mcpSvr, "loom_next_step", nil)
	assert.False(t, next.IsError)

	var got mcp.NextStepResult
	require.NoError(t, json.Unmarshal([]byte(toolText(t, next)), &got))
	assert.Equal(t, "IDLE", got.State, "FSM must be rolled back when duplicate branch cannot return cached result")
}

func TestReadOnlyTools_SkipIdempotencyLookup(t *testing.T) {
	s, mcpSvr := newTestServer(t)
	st := s.Store().(*memStore)

	getState := callTool(t, mcpSvr, "loom_get_state", nil)
	heartbeat := callTool(t, mcpSvr, "loom_heartbeat", nil)

	assert.False(t, getState.IsError)
	assert.False(t, heartbeat.IsError)

	st.mu.Lock()
	defer st.mu.Unlock()
	assert.Equal(t, 0, st.readActionLookupCalls)
}

func TestLoomHeartbeat_ReturnsCurrentState(t *testing.T) {
	_, mcpSvr := newTestServer(t)

	result := callTool(t, mcpSvr, "loom_heartbeat", nil)
	assert.False(t, result.IsError)

	var got mcp.HeartbeatResult
	require.NoError(t, json.Unmarshal([]byte(toolText(t, result)), &got))

	assert.Equal(t, "IDLE", got.State)
	// IDLE is a non-gate state: Wait must be false, RetryInSeconds must be 0.
	assert.False(t, got.Wait)
	assert.Equal(t, 0, got.RetryInSeconds)
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

// --------------------------------------------------------------------------
// E8: Coverage — handleAbort store failure, startMonitor goroutine, Serve
// --------------------------------------------------------------------------

// failingAbortStore fails on all writes, used to test handleAbort error path.
type failingAbortStore struct {
	memStore
}

func (s *failingAbortStore) WriteCheckpoint(_ context.Context, _ store.Checkpoint) error {
	return errors.New("simulated abort store failure")
}

func (s *failingAbortStore) Close() error { return nil }

func TestLoomAbort_StoreWriteFailure_ReturnsError(t *testing.T) {
	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := &failingAbortStore{}
	s := mcp.NewServer(machine, st, nil)
	mcpSvr := s.MCPServer()

	result := callTool(t, mcpSvr, "loom_abort", nil)
	assert.True(t, result.IsError, "expected tool error when abort store write fails")
}

// TestServe_CancelledContext verifies that Serve returns without error when
// the provided context is already cancelled. This exercises the Serve method
// and the startMonitor goroutine startup.
func TestServe_CancelledContext(t *testing.T) {
	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newMemStore()
	s := mcp.NewServer(machine, st, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	// Use a pipe so Serve has valid I/O but the context is already done.
	pr, pw := io.Pipe()
	defer pr.Close()
	defer pw.Close()

	var buf bytes.Buffer
	// Serve should return quickly because ctx is already cancelled.
	err := s.Serve(ctx, pr, &buf)
	// Accept either nil or a context error.
	if err != nil {
		assert.ErrorIs(t, err, context.Canceled)
	}
}
