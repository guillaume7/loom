package mcp_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/guillaume7/loom/internal/agentspawn"
	"github.com/guillaume7/loom/internal/fsm"
	"github.com/guillaume7/loom/internal/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --------------------------------------------------------------------------
// containmentSpawner — mock spawner for failure containment tests
// --------------------------------------------------------------------------

// containmentSpawner returns a deterministic Exit for testing failure paths.
// If pastDeadline is true the contract deadline is set one second in the past
// so that ClassifyAgentJobExit sees a timeout.
type containmentSpawner struct {
	exitCode    int
	cleanupErr  error
	pastDeadline bool
}

func (s *containmentSpawner) Spawn(req agentspawn.Request) (agentspawn.SpawnHandle, error) {
	contract := req.Contract
	if s.pastDeadline {
		contract.Deadline = time.Now().Add(-1 * time.Second)
	}
	started := agentspawn.Started{
		StoryID:  req.StoryID,
		Prompt:   req.Prompt,
		Worktree: req.Worktree,
		Contract: contract,
		Path:     "/usr/bin/code",
		Args:     []string{"chat", "-m", "loom-orchestrator", "--worktree", req.Worktree, req.Prompt},
		PID:      54321,
	}
	done := make(chan agentspawn.Exit, 1)
	done <- agentspawn.Exit{
		Started:    started,
		ExitCode:   s.exitCode,
		CleanupErr: s.cleanupErr,
	}
	close(done)
	return &containmentSpawnHandle{started: started, done: done}, nil
}

type containmentSpawnHandle struct {
	started agentspawn.Started
	done    <-chan agentspawn.Exit
}

func (h *containmentSpawnHandle) Started() agentspawn.Started { return h.started }
func (h *containmentSpawnHandle) Done() <-chan agentspawn.Exit { return h.done }

// --------------------------------------------------------------------------
// failedDetailPayload — partial payload struct for test assertions
// --------------------------------------------------------------------------

type failedDetailPayload struct {
	StoryID     string `json:"story_id"`
	PID         int    `json:"pid"`
	ExitCode    int    `json:"exit_code"`
	FailureKind string `json:"failure_kind"`
	Outcome     string `json:"outcome"`
	LockState   string `json:"lock_state"`
	Error       string `json:"error,omitempty"`
	CleanupError string `json:"cleanup_error,omitempty"`
	ObservedAt  string `json:"observed_at"`
}

// --------------------------------------------------------------------------
// Tests
// --------------------------------------------------------------------------

func TestAwaitBackgroundAgentExit_CrashWritesFailedAction(t *testing.T) {
	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newMemStore()
	s := mcp.NewServer(machine, st, nil,
		mcp.WithSpawner(&containmentSpawner{exitCode: 2}),
	)
	mcpSvr := s.MCPServer()

	result := callTool(t, mcpSvr, "loom_spawn_agent", map[string]interface{}{
		"story_id": "US-5.3",
		"prompt":   "Implement US-5.3",
		"worktree": "worktree-us-5.3",
	})
	require.False(t, result.IsError, toolText(t, result))

	failedAction := waitForActionEvent(t, s.Store(), "background_agent_failed")

	assert.Equal(t, "background_agent_exited", failedAction.StateBefore)
	assert.Equal(t, "background_agent_failed", failedAction.StateAfter)

	var detail failedDetailPayload
	require.NoError(t, json.Unmarshal([]byte(failedAction.Detail), &detail))
	assert.Equal(t, "US-5.3", detail.StoryID)
	assert.Equal(t, 54321, detail.PID)
	assert.Equal(t, 2, detail.ExitCode)
	assert.Equal(t, string(mcp.AgentJobFailureKindCrash), detail.FailureKind)
	assert.Equal(t, string(mcp.AgentJobFailureOutcomeRetry), detail.Outcome)
	assert.Equal(t, "recoverable", detail.LockState)
	assert.NotEmpty(t, detail.ObservedAt)
}

func TestAwaitBackgroundAgentExit_TimeoutWritesFailedAction(t *testing.T) {
	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newMemStore()
	s := mcp.NewServer(machine, st, nil,
		mcp.WithSpawner(&containmentSpawner{exitCode: 1, pastDeadline: true}),
	)
	mcpSvr := s.MCPServer()

	result := callTool(t, mcpSvr, "loom_spawn_agent", map[string]interface{}{
		"story_id": "US-5.3",
		"prompt":   "Implement US-5.3",
		"worktree": "worktree-us-5.3",
	})
	require.False(t, result.IsError, toolText(t, result))

	failedAction := waitForActionEvent(t, s.Store(), "background_agent_failed")

	var detail failedDetailPayload
	require.NoError(t, json.Unmarshal([]byte(failedAction.Detail), &detail))
	assert.Equal(t, string(mcp.AgentJobFailureKindTimeout), detail.FailureKind)
	assert.Equal(t, string(mcp.AgentJobFailureOutcomeRetry), detail.Outcome)
	assert.Equal(t, "recoverable", detail.LockState)
}

func TestAwaitBackgroundAgentExit_CleanupFailureWritesEscalatedOutcome(t *testing.T) {
	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newMemStore()
	s := mcp.NewServer(machine, st, nil,
		mcp.WithSpawner(&containmentSpawner{
			exitCode:   0,
			cleanupErr: errors.New("worktree removal failed"),
		}),
	)
	mcpSvr := s.MCPServer()

	result := callTool(t, mcpSvr, "loom_spawn_agent", map[string]interface{}{
		"story_id": "US-5.3",
		"prompt":   "Implement US-5.3",
		"worktree": "worktree-us-5.3",
	})
	require.False(t, result.IsError, toolText(t, result))

	failedAction := waitForActionEvent(t, s.Store(), "background_agent_failed")

	var detail failedDetailPayload
	require.NoError(t, json.Unmarshal([]byte(failedAction.Detail), &detail))
	assert.Equal(t, string(mcp.AgentJobFailureKindCleanupFailure), detail.FailureKind)
	assert.Equal(t, string(mcp.AgentJobFailureOutcomeEscalate), detail.Outcome)
	assert.Equal(t, "recoverable", detail.LockState)
	assert.Equal(t, "worktree removal failed", detail.CleanupError)
	assert.Equal(t, 0, detail.ExitCode)
}

func TestAwaitBackgroundAgentExit_SuccessDoesNotWriteFailedAction(t *testing.T) {
	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newMemStore()
	s := mcp.NewServer(machine, st, nil,
		mcp.WithSpawner(&containmentSpawner{exitCode: 0}),
	)
	mcpSvr := s.MCPServer()

	result := callTool(t, mcpSvr, "loom_spawn_agent", map[string]interface{}{
		"story_id": "US-5.3",
		"prompt":   "Implement US-5.3",
		"worktree": "worktree-us-5.3",
	})
	require.False(t, result.IsError, toolText(t, result))

	// Wait for the exit action to ensure the goroutine has completed.
	waitForActionEvent(t, s.Store(), "background_agent_exited")

	// Give the goroutine a short settling window, then verify no failure action.
	time.Sleep(50 * time.Millisecond)

	actions, err := s.Store().ReadActions(context.Background(), 50)
	require.NoError(t, err)
	for _, a := range actions {
		assert.NotEqual(t, "background_agent_failed", a.Event,
			"success exit must not produce background_agent_failed action")
	}
}
