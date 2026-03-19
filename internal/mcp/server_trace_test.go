package mcp_test

// --------------------------------------------------------------------------
// TH2.E9: Run-Loom Session Traceability
// --------------------------------------------------------------------------

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/guillaume7/loom/internal/fsm"
	"github.com/guillaume7/loom/internal/mcp"
	"github.com/guillaume7/loom/internal/store"
	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --------------------------------------------------------------------------
// US-9.1: Session trace header and append-only event model
// --------------------------------------------------------------------------

func TestSessionTrace_OpenTrace_CreatesHeader(t *testing.T) {
	st := newMemStore()
	ctx := context.Background()

	trace := store.SessionTrace{
		SessionID:  "session-abc",
		LoomVer:    "0.1.0",
		Repository: "owner/repo",
		StartedAt:  time.Now(),
		Outcome:    "in_progress",
	}
	require.NoError(t, st.OpenSessionTrace(ctx, trace))

	got, _, err := st.ReadSessionTrace(ctx, "session-abc")
	require.NoError(t, err)
	assert.Equal(t, "session-abc", got.SessionID)
	assert.Equal(t, "0.1.0", got.LoomVer)
	assert.Equal(t, "owner/repo", got.Repository)
	assert.Equal(t, "in_progress", got.Outcome)
	assert.False(t, got.StartedAt.IsZero())
}

func TestSessionTrace_OpenTrace_Idempotent(t *testing.T) {
	st := newMemStore()
	ctx := context.Background()

	trace := store.SessionTrace{SessionID: "s1", LoomVer: "1.0", Repository: "a/b", StartedAt: time.Now()}
	require.NoError(t, st.OpenSessionTrace(ctx, trace))
	require.NoError(t, st.OpenSessionTrace(ctx, trace)) // second call should be a no-op

	traces, err := st.ListSessionTraces(ctx, 10)
	require.NoError(t, err)
	assert.Len(t, traces, 1, "duplicate open should not create a second trace")
}

func TestSessionTrace_CloseTrace_SetsOutcomeAndEndedAt(t *testing.T) {
	st := newMemStore()
	ctx := context.Background()

	require.NoError(t, st.OpenSessionTrace(ctx, store.SessionTrace{
		SessionID: "s1", StartedAt: time.Now(),
	}))
	require.NoError(t, st.CloseSessionTrace(ctx, "s1", "complete"))

	got, _, err := st.ReadSessionTrace(ctx, "s1")
	require.NoError(t, err)
	assert.Equal(t, "complete", got.Outcome)
	assert.False(t, got.EndedAt.IsZero(), "EndedAt must be set after closing")
}

func TestSessionTrace_AppendEvent_IncrementsSeq(t *testing.T) {
	st := newMemStore()
	ctx := context.Background()

	require.NoError(t, st.OpenSessionTrace(ctx, store.SessionTrace{SessionID: "s1", StartedAt: time.Now()}))

	ev1 := store.TraceEvent{SessionID: "s1", Kind: "transition", FromState: "IDLE", ToState: "SCANNING", Event: "start"}
	ev2 := store.TraceEvent{SessionID: "s1", Kind: "transition", FromState: "SCANNING", ToState: "ISSUE_CREATED", Event: "phase_identified"}

	require.NoError(t, st.AppendTraceEvent(ctx, ev1))
	require.NoError(t, st.AppendTraceEvent(ctx, ev2))

	_, events, err := st.ReadSessionTrace(ctx, "s1")
	require.NoError(t, err)
	require.Len(t, events, 2)
	assert.Equal(t, 1, events[0].Seq)
	assert.Equal(t, 2, events[1].Seq)
}

func TestSessionTrace_ReadSessionTrace_UnknownSessionID_ReturnsEmpty(t *testing.T) {
	st := newMemStore()
	ctx := context.Background()

	got, events, err := st.ReadSessionTrace(ctx, "nonexistent")
	require.NoError(t, err)
	assert.Equal(t, store.SessionTrace{}, got)
	assert.Nil(t, events)
}

// --------------------------------------------------------------------------
// US-9.2: FSM transition and operator intervention ledger
// --------------------------------------------------------------------------

func TestSessionTrace_CheckpointAppendsTransitionEvent(t *testing.T) {
	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newMemStore()
	traceID := "trace-checkpoint-test"
	s := mcp.NewServer(machine, st, nil, mcp.WithTraceSessionID(traceID))

	// Open the trace manually (Serve() normally does this).
	ctx := context.Background()
	require.NoError(t, st.OpenSessionTrace(ctx, store.SessionTrace{
		SessionID: traceID, StartedAt: time.Now(),
	}))

	mcpSvr := s.MCPServer()
	result := callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": "start"})
	assert.False(t, result.IsError, "expected checkpoint to succeed")

	_, events, err := st.ReadSessionTrace(ctx, traceID)
	require.NoError(t, err)
	require.NotEmpty(t, events, "expected at least one trace event after checkpoint")

	ev := events[0]
	assert.Equal(t, "transition", ev.Kind)
	assert.Equal(t, "IDLE", ev.FromState)
	assert.Equal(t, "SCANNING", ev.ToState)
	assert.Equal(t, "start", ev.Event)
}

func TestSessionTrace_AbortAppendsInterventionEvent(t *testing.T) {
	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newMemStore()
	traceID := "trace-abort-test"
	s := mcp.NewServer(machine, st, nil, mcp.WithTraceSessionID(traceID))

	ctx := context.Background()
	require.NoError(t, st.OpenSessionTrace(ctx, store.SessionTrace{
		SessionID: traceID, StartedAt: time.Now(),
	}))

	// Advance to a non-IDLE state first.
	mcpSvr := s.MCPServer()
	callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": "start"})

	result := callTool(t, mcpSvr, "loom_abort", nil)
	assert.False(t, result.IsError, "expected abort to succeed")

	_, events, err := st.ReadSessionTrace(ctx, traceID)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(events), 2, "expected at least two trace events (start + abort)")

	// Last event should be the abort intervention.
	last := events[len(events)-1]
	assert.Equal(t, "intervention", last.Kind)
	assert.Equal(t, "abort", last.Event)
	assert.Equal(t, "PAUSED", last.ToState)
}

func TestSessionTrace_StallAppendsSystemEvent(t *testing.T) {
	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newMemStore()
	traceID := "trace-stall-test"

	clock := newFakeClock()
	stallTimeout := 5 * time.Minute
	s := mcp.NewServer(machine, st, nil,
		mcp.WithTraceSessionID(traceID),
		mcp.WithClock(clock),
		mcp.WithMonitorConfig(mcp.MonitorConfig{
			StallTimeout:      stallTimeout,
			HeartbeatInterval: time.Hour,
			TickInterval:      time.Hour,
		}),
	)

	ctx := context.Background()
	require.NoError(t, st.OpenSessionTrace(ctx, store.SessionTrace{
		SessionID: traceID, StartedAt: time.Now(),
	}))

	// Advance FSM to a gate state (AWAITING_CI).
	mcpSvr := s.MCPServer()
	callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": "start"})
	callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": "phase_identified"})
	callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": "copilot_assigned"})
	callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": "pr_opened"})
	callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": "pr_ready"})
	// Now in AWAITING_CI (gate state).

	// Advance clock past stall timeout.
	clock.Advance(stallTimeout + time.Second)
	stalled := s.RunStallCheck(ctx)
	assert.True(t, stalled, "expected stall to be detected")

	_, events, err := st.ReadSessionTrace(ctx, traceID)
	require.NoError(t, err)

	var stallEvent *store.TraceEvent
	for i := range events {
		if events[i].Event == "stall_detected" {
			stallEvent = &events[i]
			break
		}
	}
	require.NotNil(t, stallEvent, "expected a stall_detected trace event")
	assert.Equal(t, "system", stallEvent.Kind)
	assert.Equal(t, "PAUSED", stallEvent.ToState)
}

// --------------------------------------------------------------------------
// US-9.4: Human-readable session trace resource (loom://trace)
// --------------------------------------------------------------------------

func TestLoomTraceResource_ListIncludesURI(t *testing.T) {
	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newMemStore()
	s := mcp.NewServer(machine, st, nil, mcp.WithTraceSessionID("test-trace"))
	mcpSvr := s.MCPServer()

	ctx := context.Background()
	sess := newTestSession(nextSessionID())
	require.NoError(t, mcpSvr.RegisterSession(ctx, sess))
	ctx = mcpSvr.WithContext(ctx, sess)

	msg, err := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "resources/list",
		"params":  map[string]interface{}{},
	})
	require.NoError(t, err)

	raw := mcpSvr.HandleMessage(ctx, msg)
	require.NotNil(t, raw)

	resp, ok := raw.(mcplib.JSONRPCResponse)
	require.True(t, ok, "expected JSONRPCResponse, got %T", raw)

	result, ok := resp.Result.(mcplib.ListResourcesResult)
	require.True(t, ok, "expected ListResourcesResult, got %T", resp.Result)

	var uris []string
	for _, r := range result.Resources {
		uris = append(uris, r.URI)
	}
	assert.Contains(t, uris, "loom://trace")
	assert.Contains(t, uris, "loom://trace/index")
}

func TestLoomTraceResource_NoTraceID_ReturnsFallback(t *testing.T) {
	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newMemStore()
	// No WithTraceSessionID option → traceSessionID is empty.
	s := mcp.NewServer(machine, st, nil)
	mcpSvr := s.MCPServer()

	resp := callResourceRead(t, mcpSvr, "loom://trace")
	result, ok := resp.Result.(mcplib.ReadResourceResult)
	require.True(t, ok)
	require.Len(t, result.Contents, 1)

	tc, ok := result.Contents[0].(mcplib.TextResourceContents)
	require.True(t, ok)
	assert.Equal(t, "loom://trace", tc.URI)
	assert.Equal(t, "text/markdown", tc.MIMEType)
	assert.Contains(t, tc.Text, "No active session trace")
}

func TestLoomTraceResource_WithTrace_ReturnsMarkdown(t *testing.T) {
	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newMemStore()
	traceID := "trace-md-test"
	s := mcp.NewServer(machine, st, nil,
		mcp.WithTraceSessionID(traceID),
		mcp.WithLoomVersion("0.2.0"),
		mcp.WithRepository("acme/widget"),
	)

	ctx := context.Background()
	require.NoError(t, st.OpenSessionTrace(ctx, store.SessionTrace{
		SessionID:  traceID,
		LoomVer:    "0.2.0",
		Repository: "acme/widget",
		StartedAt:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Outcome:    "in_progress",
	}))
	require.NoError(t, st.AppendTraceEvent(ctx, store.TraceEvent{
		SessionID: traceID,
		Kind:      "transition",
		FromState: "IDLE",
		ToState:   "SCANNING",
		Event:     "start",
		Reason:    "",
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 1, 0, time.UTC),
	}))

	mcpSvr := s.MCPServer()
	resp := callResourceRead(t, mcpSvr, "loom://trace")
	result, ok := resp.Result.(mcplib.ReadResourceResult)
	require.True(t, ok)
	require.Len(t, result.Contents, 1)

	tc, ok := result.Contents[0].(mcplib.TextResourceContents)
	require.True(t, ok)
	assert.Equal(t, "text/markdown", tc.MIMEType)

	text := tc.Text
	assert.Contains(t, text, "# Session Trace")
	assert.Contains(t, text, traceID)
	assert.Contains(t, text, "acme/widget")
	assert.Contains(t, text, "0.2.0")
	assert.Contains(t, text, "IDLE")
	assert.Contains(t, text, "SCANNING")
	assert.Contains(t, text, "start")
	assert.Contains(t, text, "in_progress")
}

func TestLoomTraceResource_WithGitHubRefs_ShowsGitHubLedger(t *testing.T) {
	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newMemStore()
	traceID := "trace-gh-ledger"
	s := mcp.NewServer(machine, st, nil, mcp.WithTraceSessionID(traceID))

	ctx := context.Background()
	require.NoError(t, st.OpenSessionTrace(ctx, store.SessionTrace{
		SessionID: traceID, StartedAt: time.Now(),
	}))
	require.NoError(t, st.AppendTraceEvent(ctx, store.TraceEvent{
		SessionID:   traceID,
		Kind:        "transition",
		FromState:   "AWAITING_PR",
		ToState:     "AWAITING_CI",
		Event:       "pr_opened",
		PRNumber:    42,
		IssueNumber: 7,
	}))

	mcpSvr := s.MCPServer()
	resp := callResourceRead(t, mcpSvr, "loom://trace")
	result, ok := resp.Result.(mcplib.ReadResourceResult)
	require.True(t, ok)
	require.Len(t, result.Contents, 1)

	tc, ok := result.Contents[0].(mcplib.TextResourceContents)
	require.True(t, ok)

	assert.Contains(t, tc.Text, "## GitHub Ledger")
	assert.Contains(t, tc.Text, "#42")
	assert.Contains(t, tc.Text, "#7")
	assert.Contains(t, tc.Text, "PR")
	assert.Contains(t, tc.Text, "Issue")
}

// --------------------------------------------------------------------------
// US-9.5: Session index (loom://trace/index)
// --------------------------------------------------------------------------

func TestLoomTraceIndexResource_ReturnsJSONArray(t *testing.T) {
	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newMemStore()
	ctx := context.Background()

	// Populate two session traces.
	require.NoError(t, st.OpenSessionTrace(ctx, store.SessionTrace{
		SessionID: "older-session", LoomVer: "0.1.0", Repository: "a/b",
		StartedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Outcome: "complete",
	}))
	require.NoError(t, st.CloseSessionTrace(ctx, "older-session", "complete"))
	require.NoError(t, st.OpenSessionTrace(ctx, store.SessionTrace{
		SessionID: "newer-session", LoomVer: "0.2.0", Repository: "a/b",
		StartedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), Outcome: "in_progress",
	}))

	s := mcp.NewServer(machine, st, nil)
	mcpSvr := s.MCPServer()

	resp := callResourceRead(t, mcpSvr, "loom://trace/index")
	result, ok := resp.Result.(mcplib.ReadResourceResult)
	require.True(t, ok)
	require.Len(t, result.Contents, 1)

	tc, ok := result.Contents[0].(mcplib.TextResourceContents)
	require.True(t, ok)
	assert.Equal(t, "application/json", tc.MIMEType)

	var entries []map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &entries))
	require.Len(t, entries, 2)

	// Newest first.
	assert.Equal(t, "newer-session", entries[0]["session_id"])
	assert.Equal(t, "older-session", entries[1]["session_id"])

	// Older session should have an ended_at, newer should not.
	_, olderHasEndedAt := entries[1]["ended_at"]
	assert.True(t, olderHasEndedAt, "older session should have ended_at")
}

func TestLoomTraceIndexResource_EmptyStore_ReturnsEmptyArray(t *testing.T) {
	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newMemStore()
	s := mcp.NewServer(machine, st, nil)
	mcpSvr := s.MCPServer()

	resp := callResourceRead(t, mcpSvr, "loom://trace/index")
	result, ok := resp.Result.(mcplib.ReadResourceResult)
	require.True(t, ok)
	require.Len(t, result.Contents, 1)

	tc, ok := result.Contents[0].(mcplib.TextResourceContents)
	require.True(t, ok)

	var entries []map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &entries))
	assert.Empty(t, entries)
}

// --------------------------------------------------------------------------
// SQLite store: session trace round-trip tests
// --------------------------------------------------------------------------

func TestSQLiteSessionTrace_RoundTrip(t *testing.T) {
	st, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, st.Close()) })

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)

	// Open a trace.
	require.NoError(t, st.OpenSessionTrace(ctx, store.SessionTrace{
		SessionID:  "sqlite-s1",
		LoomVer:    "1.2.3",
		Repository: "myorg/myrepo",
		StartedAt:  now,
		Outcome:    "in_progress",
	}))

	// Append events.
	require.NoError(t, st.AppendTraceEvent(ctx, store.TraceEvent{
		SessionID: "sqlite-s1",
		Kind:      "transition",
		FromState: "IDLE",
		ToState:   "SCANNING",
		Event:     "start",
		Reason:    "initial start",
		CreatedAt: now.Add(time.Second),
	}))
	require.NoError(t, st.AppendTraceEvent(ctx, store.TraceEvent{
		SessionID:   "sqlite-s1",
		Kind:        "github",
		FromState:   "SCANNING",
		ToState:     "ISSUE_CREATED",
		Event:       "phase_identified",
		PRNumber:    0,
		IssueNumber: 12,
		CreatedAt:   now.Add(2 * time.Second),
	}))

	// Read back.
	trace, events, err := st.ReadSessionTrace(ctx, "sqlite-s1")
	require.NoError(t, err)
	assert.Equal(t, "sqlite-s1", trace.SessionID)
	assert.Equal(t, "1.2.3", trace.LoomVer)
	assert.Equal(t, "myorg/myrepo", trace.Repository)
	assert.Equal(t, "in_progress", trace.Outcome)
	assert.True(t, trace.EndedAt.IsZero(), "trace not yet closed")
	require.Len(t, events, 2)
	assert.Equal(t, 1, events[0].Seq)
	assert.Equal(t, "start", events[0].Event)
	assert.Equal(t, 2, events[1].Seq)
	assert.Equal(t, 12, events[1].IssueNumber)

	// Close the trace.
	require.NoError(t, st.CloseSessionTrace(ctx, "sqlite-s1", "complete"))

	trace, _, err = st.ReadSessionTrace(ctx, "sqlite-s1")
	require.NoError(t, err)
	assert.Equal(t, "complete", trace.Outcome)
	assert.False(t, trace.EndedAt.IsZero())
}

func TestSQLiteSessionTrace_ListSessionTraces_NewestFirst(t *testing.T) {
	st, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, st.Close()) })

	ctx := context.Background()
	for i, id := range []string{"alpha", "beta", "gamma"} {
		_ = i
		require.NoError(t, st.OpenSessionTrace(ctx, store.SessionTrace{
			SessionID: id, StartedAt: time.Now(),
		}))
	}

	traces, err := st.ListSessionTraces(ctx, 10)
	require.NoError(t, err)
	require.Len(t, traces, 3)
	// Newest inserted last → returned first.
	assert.Equal(t, "gamma", traces[0].SessionID)
	assert.Equal(t, "beta", traces[1].SessionID)
	assert.Equal(t, "alpha", traces[2].SessionID)
}

func TestSQLiteSessionTrace_DeleteAll_ClearsTraces(t *testing.T) {
	st, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, st.Close()) })

	ctx := context.Background()
	require.NoError(t, st.OpenSessionTrace(ctx, store.SessionTrace{
		SessionID: "s1", StartedAt: time.Now(),
	}))
	require.NoError(t, st.AppendTraceEvent(ctx, store.TraceEvent{
		SessionID: "s1", Kind: "transition", Event: "start", CreatedAt: time.Now(),
	}))

	require.NoError(t, st.DeleteAll(ctx))

	trace, events, err := st.ReadSessionTrace(ctx, "s1")
	require.NoError(t, err)
	assert.Equal(t, store.SessionTrace{}, trace)
	assert.Nil(t, events)
}

// callResourceRead is defined in server_resources_test.go already.
// This file reuses the helper directly via the shared test package.
