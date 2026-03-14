package mcp_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/guillaume7/loom/internal/fsm"
	"github.com/guillaume7/loom/internal/mcp"
	"github.com/guillaume7/loom/internal/store"
	mcplib "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
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
	result, _ := callToolWithSession(t, mcpSvr, toolName, args)
	return result
}

// callToolWithSession is like callTool but returns the registered test session
// so callers can inspect any server notifications sent during the call.
func callToolWithSession(t *testing.T, mcpSvr *mcpserver.MCPServer, toolName string, args map[string]interface{}) (*mcplib.CallToolResult, *testSession) {
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

	return &result, sess
}

func drainNotifications(session *testSession) []mcplib.JSONRPCNotification {
	notifications := make([]mcplib.JSONRPCNotification, 0)
	for {
		select {
		case note := <-session.notifications:
			notifications = append(notifications, note)
		default:
			return notifications
		}
	}
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
