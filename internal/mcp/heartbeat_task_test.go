package mcp_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/guillaume7/loom/internal/fsm"
	loomgh "github.com/guillaume7/loom/internal/github"
	"github.com/guillaume7/loom/internal/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type heartbeatPollingClientMock struct {
	pr              *loomgh.PR
	checkRunsByPoll [][]*loomgh.CheckRun
	pollCalls       int
}

func (m *heartbeatPollingClientMock) Ping(context.Context) error { return nil }

func (m *heartbeatPollingClientMock) GetPR(_ context.Context, prNumber int) (*loomgh.PR, error) {
	if m.pr == nil {
		m.pr = &loomgh.PR{Number: prNumber}
	}
	return m.pr, nil
}

func (m *heartbeatPollingClientMock) GetCheckRuns(_ context.Context, _ string) ([]*loomgh.CheckRun, error) {
	if len(m.checkRunsByPoll) == 0 {
		return []*loomgh.CheckRun{}, nil
	}
	index := m.pollCalls
	if index >= len(m.checkRunsByPoll) {
		index = len(m.checkRunsByPoll) - 1
	}
	m.pollCalls++
	return m.checkRunsByPoll[index], nil
}

func TestLoomHeartbeat_PollingMode_EmitsTaskLifecycle_Success(t *testing.T) {
	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newMemStore()
	gh := &heartbeatPollingClientMock{
		pr: &loomgh.PR{Number: 42, HeadSHA: "abc123"},
		checkRunsByPoll: [][]*loomgh.CheckRun{
			{
				{Name: "build", Status: "in_progress", Conclusion: ""},
				{Name: "lint", Status: "completed", Conclusion: "success"},
			},
			{
				{Name: "build", Status: "completed", Conclusion: "success"},
				{Name: "lint", Status: "completed", Conclusion: "success"},
			},
		},
	}
	s := mcp.NewServer(machine, st, gh)
	mcpSvr := s.MCPServer()
	sess := newTestSession(nextSessionID())
	require.NoError(t, mcpSvr.RegisterSession(context.Background(), sess))
	initializeSessionWithCapabilities(t, mcpSvr, sess, map[string]interface{}{
		"experimental": map[string]interface{}{
			"tasks": true,
		},
	})

	result := callToolOnSession(t, mcpSvr, sess, "loom_heartbeat", map[string]interface{}{
		"poll":                  true,
		"pr_number":             42,
		"poll_interval_seconds": 0,
		"max_polls":             3,
	})
	require.False(t, result.IsError)

	notes := drainNotifications(sess)
	require.Len(t, notes, 4)

	assert.Equal(t, "loom/task/start", notes[0].Method)
	assert.Equal(t, "loom-ci-poll-pr-42", notes[0].Params.AdditionalFields["id"])
	assert.Equal(t, "Watching CI for PR #42", notes[0].Params.AdditionalFields["title"])
	assert.Equal(t, true, notes[0].Params.AdditionalFields["cancellable"])

	assert.Equal(t, "loom/task/progress", notes[1].Method)
	assert.Equal(t, "loom-ci-poll-pr-42", notes[1].Params.AdditionalFields["id"])
	progress1, ok := notes[1].Params.AdditionalFields["text"].(string)
	require.True(t, ok)
	assert.Contains(t, progress1, "1/2 checks green")
	assert.Contains(t, progress1, "waiting on 'build'")

	assert.Equal(t, "loom/task/progress", notes[2].Method)
	assert.Equal(t, "loom-ci-poll-pr-42", notes[2].Params.AdditionalFields["id"])
	progress2, ok := notes[2].Params.AdditionalFields["text"].(string)
	require.True(t, ok)
	assert.Contains(t, progress2, "2/2 checks green")

	assert.Equal(t, "loom/task/done", notes[3].Method)
	assert.Equal(t, "loom-ci-poll-pr-42", notes[3].Params.AdditionalFields["id"])
	doneResult, ok := notes[3].Params.AdditionalFields["result"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, true, doneResult["all_green"])
}

func TestLoomHeartbeat_PollingMode_EmitsDoneWithFailedChecks(t *testing.T) {
	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newMemStore()
	gh := &heartbeatPollingClientMock{
		pr: &loomgh.PR{Number: 42, HeadSHA: "abc123"},
		checkRunsByPoll: [][]*loomgh.CheckRun{
			{
				{Name: "build", Status: "completed", Conclusion: "failure"},
				{Name: "lint", Status: "completed", Conclusion: "success"},
			},
		},
	}
	s := mcp.NewServer(machine, st, gh)
	mcpSvr := s.MCPServer()
	sess := newTestSession(nextSessionID())
	require.NoError(t, mcpSvr.RegisterSession(context.Background(), sess))
	initializeSessionWithCapabilities(t, mcpSvr, sess, map[string]interface{}{
		"experimental": map[string]interface{}{
			"tasks": true,
		},
	})

	result := callToolOnSession(t, mcpSvr, sess, "loom_heartbeat", map[string]interface{}{
		"poll":                  true,
		"pr_number":             42,
		"poll_interval_seconds": 0,
		"max_polls":             1,
	})
	require.False(t, result.IsError)

	notes := drainNotifications(sess)
	require.Len(t, notes, 3)

	assert.Equal(t, "loom/task/start", notes[0].Method)
	assert.Equal(t, "loom-ci-poll-pr-42", notes[0].Params.AdditionalFields["id"])
	assert.Equal(t, "loom/task/progress", notes[1].Method)

	progress, ok := notes[1].Params.AdditionalFields["text"].(string)
	require.True(t, ok)
	assert.Contains(t, progress, "failures: build")

	assert.Equal(t, "loom/task/done", notes[2].Method)
	doneResult, ok := notes[2].Params.AdditionalFields["result"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, false, doneResult["all_green"])
	failedAny, ok := doneResult["failed_checks"]
	require.True(t, ok)
	switch failed := failedAny.(type) {
	case []string:
		require.Equal(t, []string{"build"}, failed)
	case []any:
		require.Len(t, failed, 1)
		assert.Equal(t, "build", failed[0])
	default:
		require.Failf(t, "failed_checks type", "unexpected failed_checks type %T", failedAny)
	}
}

func TestLoomHeartbeat_PollingMode_WithoutTaskCapability_BlocksWithoutTaskEvents(t *testing.T) {
	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newMemStore()
	gh := &heartbeatPollingClientMock{
		pr: &loomgh.PR{Number: 42, HeadSHA: "abc123"},
		checkRunsByPoll: [][]*loomgh.CheckRun{
			{
				{Name: "build", Status: "in_progress", Conclusion: ""},
			},
			{
				{Name: "build", Status: "completed", Conclusion: "success"},
			},
		},
	}
	s := mcp.NewServer(machine, st, gh)
	mcpSvr := s.MCPServer()
	sess := newTestSession(nextSessionID())
	require.NoError(t, mcpSvr.RegisterSession(context.Background(), sess))
	initializeSessionWithCapabilities(t, mcpSvr, sess, map[string]interface{}{})

	result := callToolOnSession(t, mcpSvr, sess, "loom_heartbeat", map[string]interface{}{
		"poll":                  true,
		"pr_number":             42,
		"poll_interval_seconds": 0,
		"max_polls":             3,
	})
	require.False(t, result.IsError)

	var payload mcp.HeartbeatResult
	require.NoError(t, json.Unmarshal([]byte(toolText(t, result)), &payload))
	assert.Equal(t, "IDLE", payload.State)
	assert.False(t, payload.Wait)
	assert.Equal(t, 0, payload.RetryInSeconds)
	assert.Equal(t, 2, gh.pollCalls, "polling should block and perform CI polling in v1 fallback mode")
	assert.Empty(t, drainNotifications(sess), "fallback mode must not emit task lifecycle notifications")
}

func TestLoomHeartbeat_PollingMode_TaskCapabilityNegotiatedOnceAndCached(t *testing.T) {
	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newMemStore()
	gh := &heartbeatPollingClientMock{
		pr: &loomgh.PR{Number: 42, HeadSHA: "abc123"},
		checkRunsByPoll: [][]*loomgh.CheckRun{
			{
				{Name: "build", Status: "completed", Conclusion: "success"},
			},
		},
	}
	s := mcp.NewServer(machine, st, gh)
	mcpSvr := s.MCPServer()
	sess := newTestSession(nextSessionID())
	require.NoError(t, mcpSvr.RegisterSession(context.Background(), sess))
	initializeSessionWithCapabilities(t, mcpSvr, sess, map[string]interface{}{
		"experimental": map[string]interface{}{
			"tasks": true,
		},
	})

	first := callToolOnSession(t, mcpSvr, sess, "loom_heartbeat", map[string]interface{}{
		"poll":                  true,
		"pr_number":             42,
		"poll_interval_seconds": 0,
		"max_polls":             1,
	})
	require.False(t, first.IsError)
	firstNotes := drainNotifications(sess)
	require.Len(t, firstNotes, 3)
	require.Equal(t, "loom/task/start", firstNotes[0].Method)

	second := callToolOnSession(t, mcpSvr, sess, "loom_heartbeat", map[string]interface{}{
		"poll":                  true,
		"pr_number":             42,
		"poll_interval_seconds": 0,
		"max_polls":             1,
	})
	require.False(t, second.IsError)
	secondNotes := drainNotifications(sess)
	require.Len(t, secondNotes, 3)
	require.Equal(t, "loom/task/start", secondNotes[0].Method)

	// Each call should only emit one start/progress/done sequence. If capability
	// negotiation happened per call, behavior could drift; this ensures cached
	// session capability is reused across multiple tool calls.
	assert.Equal(t, []string{"loom/task/start", "loom/task/progress", "loom/task/done"}, []string{firstNotes[0].Method, firstNotes[1].Method, firstNotes[2].Method})
	assert.Equal(t, []string{"loom/task/start", "loom/task/progress", "loom/task/done"}, []string{secondNotes[0].Method, secondNotes[1].Method, secondNotes[2].Method})

	// Sanity check that polling ran twice (one poll per call with max_polls=1).
	assert.Equal(t, 2, gh.pollCalls)
}
