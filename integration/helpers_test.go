// Package integration contains end-to-end integration tests for the Loom
// workflow engine, exercising the full FSM + MCP server + GitHub client +
// SQLite store stack together.
package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	loomgh "github.com/guillaume7/loom/internal/github"
	"github.com/guillaume7/loom/internal/mcp"
	"github.com/guillaume7/loom/internal/store"
	mcplib "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/require"
)

// --------------------------------------------------------------------------
// Session counter — unique session IDs prevent collisions in parallel tests
// --------------------------------------------------------------------------

var sessionIDCounter atomic.Int64

func nextSessionID() string {
	return fmt.Sprintf("integ-session-%d", sessionIDCounter.Add(1))
}

// --------------------------------------------------------------------------
// testSession implements mcpserver.ClientSession
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
// callTool — drives a single MCP tool call
// --------------------------------------------------------------------------

// callTool sends a tools/call request to mcpSvr and returns the result.
func callTool(t *testing.T, mcpSvr *mcpserver.MCPServer, toolName string, args map[string]interface{}) *mcplib.CallToolResult {
	t.Helper()
	if args == nil {
		args = map[string]interface{}{}
	}

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

	result, ok := parseCallToolResult(resp.Result)
	require.True(t, ok, "expected CallToolResult in response.Result, got %T", resp.Result)

	return result
}

// newRegisteredSession registers and returns a reusable test session.
func newRegisteredSession(t *testing.T, mcpSvr *mcpserver.MCPServer) *testSession {
	t.Helper()

	sess := newTestSession(nextSessionID())
	require.NoError(t, mcpSvr.RegisterSession(context.Background(), sess))
	return sess
}

// callToolOnSession sends a tools/call request on an already-registered session.
func callToolOnSession(t *testing.T, mcpSvr *mcpserver.MCPServer, sess *testSession, toolName string, args map[string]interface{}) *mcplib.CallToolResult {
	t.Helper()

	if args == nil {
		args = map[string]interface{}{}
	}

	ctx := mcpSvr.WithContext(context.Background(), sess)
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

	result, ok := parseCallToolResult(resp.Result)
	require.True(t, ok, "expected CallToolResult in response.Result, got %T", resp.Result)

	return result
}

func parseCallToolResult(v interface{}) (*mcplib.CallToolResult, bool) {
	switch result := v.(type) {
	case mcplib.CallToolResult:
		copied := result
		return &copied, true
	case *mcplib.CallToolResult:
		return result, true
	default:
		return nil, false
	}
}

// initializeSessionWithCapabilities sends an initialize request for an
// already-registered session so capability-gated behavior can be tested.
func initializeSessionWithCapabilities(t *testing.T, mcpSvr *mcpserver.MCPServer, sess *testSession, capabilities map[string]interface{}) {
	t.Helper()

	if capabilities == nil {
		capabilities = map[string]interface{}{}
	}

	ctx := mcpSvr.WithContext(context.Background(), sess)
	msg, err := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": mcplib.LATEST_PROTOCOL_VERSION,
			"capabilities":    capabilities,
			"clientInfo": map[string]interface{}{
				"name":    "loom-integration-test-client",
				"version": "0.0.0",
			},
		},
	})
	require.NoError(t, err)

	raw := mcpSvr.HandleMessage(ctx, msg)
	require.NotNil(t, raw)

	resp, ok := raw.(mcplib.JSONRPCResponse)
	require.True(t, ok, "expected JSONRPCResponse, got %T", raw)
	require.NotNil(t, resp.Result, "initialize returned empty result")
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

// checkpoint calls loom_checkpoint with the given action and returns the result.
func checkpoint(t *testing.T, mcpSvr *mcpserver.MCPServer, action string) *mcp.CheckpointResult {
	t.Helper()
	r := callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": action})
	if r.IsError {
		t.Fatalf("loom_checkpoint(%q) returned error: %v", action, toolText(t, r))
	}
	var result mcp.CheckpointResult
	require.NoError(t, json.Unmarshal([]byte(toolText(t, r)), &result))
	return &result
}

// nextStep calls loom_next_step and returns the result.
func nextStep(t *testing.T, mcpSvr *mcpserver.MCPServer) *mcp.NextStepResult {
	t.Helper()
	r := callTool(t, mcpSvr, "loom_next_step", nil)
	require.False(t, r.IsError)
	var result mcp.NextStepResult
	require.NoError(t, json.Unmarshal([]byte(toolText(t, r)), &result))
	return &result
}

// toolText returns the first text content item from a CallToolResult.
func toolText(t *testing.T, result *mcplib.CallToolResult) string {
	t.Helper()
	require.NotEmpty(t, result.Content, "CallToolResult has no content")
	tc, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok, "expected TextContent, got %T", result.Content[0])
	return tc.Text
}

// --------------------------------------------------------------------------
// memStore — in-memory Store implementation for tests that do not need SQLite
// --------------------------------------------------------------------------

type memStore struct {
	mu              sync.Mutex
	cp              store.Checkpoint
	actions         []store.Action
	wakes           []store.WakeSchedule
	externalEvents  []store.ExternalEvent
	runtimeLeases   map[string]store.RuntimeLease
	policyDecisions []store.PolicyDecision
	empty           bool
}

func newMemStore() *memStore {
	return &memStore{empty: true, runtimeLeases: make(map[string]store.RuntimeLease)}
}

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

func (s *memStore) UpsertWakeSchedule(_ context.Context, wake store.WakeSchedule) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for index, existing := range s.wakes {
		if existing.DedupeKey == wake.DedupeKey {
			s.wakes[index] = wake
			return nil
		}
	}
	if wake.CreatedAt.IsZero() {
		wake.CreatedAt = time.Now().UTC()
	}
	wake.ID = int64(len(s.wakes) + 1)
	s.wakes = append(s.wakes, wake)
	return nil
}

func (s *memStore) ReadWakeSchedules(_ context.Context, sessionID string, limit int) ([]store.WakeSchedule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 {
		return []store.WakeSchedule{}, nil
	}
	wakes := make([]store.WakeSchedule, 0, len(s.wakes))
	for _, wake := range s.wakes {
		if sessionID != "" && wake.SessionID != sessionID {
			continue
		}
		wakes = append(wakes, wake)
		if len(wakes) == limit {
			break
		}
	}
	return wakes, nil
}

func (s *memStore) WriteExternalEvent(_ context.Context, event store.ExternalEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if event.ObservedAt.IsZero() {
		event.ObservedAt = time.Now().UTC()
	}
	event.ID = int64(len(s.externalEvents) + 1)
	s.externalEvents = append(s.externalEvents, event)
	return nil
}

func (s *memStore) ReadExternalEvents(_ context.Context, sessionID string, limit int) ([]store.ExternalEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 {
		return []store.ExternalEvent{}, nil
	}
	events := make([]store.ExternalEvent, 0, len(s.externalEvents))
	for index := len(s.externalEvents) - 1; index >= 0; index-- {
		event := s.externalEvents[index]
		if sessionID != "" && event.SessionID != sessionID {
			continue
		}
		events = append(events, event)
		if len(events) == limit {
			break
		}
	}
	return events, nil
}

func (s *memStore) UpsertRuntimeLease(_ context.Context, lease store.RuntimeLease) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.runtimeLeases == nil {
		s.runtimeLeases = make(map[string]store.RuntimeLease)
	}
	if lease.CreatedAt.IsZero() {
		lease.CreatedAt = time.Now().UTC()
	}
	if lease.RenewedAt.IsZero() {
		lease.RenewedAt = lease.CreatedAt
	}
	s.runtimeLeases[lease.LeaseKey] = lease
	return nil
}

func (s *memStore) ReadRuntimeLease(_ context.Context, leaseKey string) (store.RuntimeLease, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	lease, ok := s.runtimeLeases[leaseKey]
	if !ok {
		return store.RuntimeLease{}, store.ErrRuntimeLeaseNotFound
	}
	return lease, nil
}

func (s *memStore) WritePolicyDecision(_ context.Context, decision store.PolicyDecision) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if decision.CreatedAt.IsZero() {
		decision.CreatedAt = time.Now().UTC()
	}
	decision.ID = int64(len(s.policyDecisions) + 1)
	s.policyDecisions = append(s.policyDecisions, decision)
	return nil
}

func (s *memStore) ReadPolicyDecisions(_ context.Context, sessionID string, limit int) ([]store.PolicyDecision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 {
		return []store.PolicyDecision{}, nil
	}
	decisions := make([]store.PolicyDecision, 0, len(s.policyDecisions))
	for index := len(s.policyDecisions) - 1; index >= 0; index-- {
		decision := s.policyDecisions[index]
		if sessionID != "" && decision.SessionID != sessionID {
			continue
		}
		decisions = append(decisions, decision)
		if len(decisions) == limit {
			break
		}
	}
	return decisions, nil
}

func (s *memStore) DeleteAll(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cp = store.Checkpoint{}
	s.actions = nil
	s.wakes = nil
	s.externalEvents = nil
	s.runtimeLeases = make(map[string]store.RuntimeLease)
	s.policyDecisions = nil
	s.empty = true
	return nil
}

func (s *memStore) Close() error { return nil }

var _ store.Store = (*memStore)(nil)

// --------------------------------------------------------------------------
// newGHClient — GitHub client pointed at an httptest server
// --------------------------------------------------------------------------

func newGHClient(baseURL string) *loomgh.HTTPClient {
	return loomgh.NewHTTPClient(
		baseURL,
		"test-token",
		"owner",
		"repo",
		loomgh.WithRetryBase(1*time.Millisecond),
	)
}

// --------------------------------------------------------------------------
// fakeClock — controllable Clock for stall-detection tests
// --------------------------------------------------------------------------

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

var _ mcp.Clock = (*fakeClock)(nil)
