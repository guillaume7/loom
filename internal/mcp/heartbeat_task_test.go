package mcp_test

import (
	"context"
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

	result, session := callToolWithSession(t, mcpSvr, "loom_heartbeat", map[string]interface{}{
		"poll":                  true,
		"pr_number":             42,
		"poll_interval_seconds": 0,
		"max_polls":             3,
	})
	require.False(t, result.IsError)

	notes := drainNotifications(session)
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

	result, session := callToolWithSession(t, mcpSvr, "loom_heartbeat", map[string]interface{}{
		"poll":                  true,
		"pr_number":             42,
		"poll_interval_seconds": 0,
		"max_polls":             1,
	})
	require.False(t, result.IsError)

	notes := drainNotifications(session)
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
