package mcp_test

import (
	"errors"
	"testing"
	"time"

	"github.com/guillaume7/loom/internal/agentspawn"
	"github.com/guillaume7/loom/internal/mcp"
	"github.com/stretchr/testify/assert"
)

func TestClassifyAgentJobExit_CleanSuccess(t *testing.T) {
	exit := agentspawn.Exit{ExitCode: 0}
	contract := agentspawn.JobContract{Deadline: time.Now().Add(30 * time.Minute)}

	_, failed := mcp.ClassifyAgentJobExit(exit, contract, time.Now())

	assert.False(t, failed, "clean exit should not be classified as a failure")
}

func TestClassifyAgentJobExit_Crash(t *testing.T) {
	exit := agentspawn.Exit{ExitCode: 1}
	contract := agentspawn.JobContract{Deadline: time.Now().Add(30 * time.Minute)}
	observedAt := time.Now()

	result, failed := mcp.ClassifyAgentJobExit(exit, contract, observedAt)

	assert.True(t, failed)
	assert.Equal(t, mcp.AgentJobFailureKindCrash, result.Kind)
	assert.Equal(t, mcp.AgentJobFailureOutcomeRetry, result.Outcome)
	assert.Equal(t, "recoverable", result.LockState)
}

func TestClassifyAgentJobExit_Crash_NegativeExitCode(t *testing.T) {
	exit := agentspawn.Exit{ExitCode: -1}
	contract := agentspawn.JobContract{Deadline: time.Now().Add(30 * time.Minute)}

	result, failed := mcp.ClassifyAgentJobExit(exit, contract, time.Now())

	assert.True(t, failed)
	assert.Equal(t, mcp.AgentJobFailureKindCrash, result.Kind)
	assert.Equal(t, mcp.AgentJobFailureOutcomeRetry, result.Outcome)
}

func TestClassifyAgentJobExit_Timeout(t *testing.T) {
	exit := agentspawn.Exit{ExitCode: 1}
	pastDeadline := time.Now().Add(-1 * time.Second)
	contract := agentspawn.JobContract{Deadline: pastDeadline}
	observedAt := time.Now()

	result, failed := mcp.ClassifyAgentJobExit(exit, contract, observedAt)

	assert.True(t, failed)
	assert.Equal(t, mcp.AgentJobFailureKindTimeout, result.Kind)
	assert.Equal(t, mcp.AgentJobFailureOutcomeRetry, result.Outcome)
	assert.Equal(t, "recoverable", result.LockState)
}

func TestClassifyAgentJobExit_TimeoutTakesPriorityOverCrash(t *testing.T) {
	// Even when exit code is non-zero, timeout classification wins when
	// the exit is observed after the contract deadline.
	exit := agentspawn.Exit{ExitCode: 137}
	contract := agentspawn.JobContract{Deadline: time.Now().Add(-5 * time.Minute)}

	result, failed := mcp.ClassifyAgentJobExit(exit, contract, time.Now())

	assert.True(t, failed)
	assert.Equal(t, mcp.AgentJobFailureKindTimeout, result.Kind)
}

func TestClassifyAgentJobExit_CleanupFailure(t *testing.T) {
	exit := agentspawn.Exit{ExitCode: 0, CleanupErr: errors.New("rm: cannot remove worktree")}
	contract := agentspawn.JobContract{Deadline: time.Now().Add(30 * time.Minute)}

	result, failed := mcp.ClassifyAgentJobExit(exit, contract, time.Now())

	assert.True(t, failed)
	assert.Equal(t, mcp.AgentJobFailureKindCleanupFailure, result.Kind)
	assert.Equal(t, mcp.AgentJobFailureOutcomeEscalate, result.Outcome)
	assert.Equal(t, "recoverable", result.LockState)
}

func TestClassifyMalformedOutput(t *testing.T) {
	result := mcp.ClassifyMalformedOutput()

	assert.Equal(t, mcp.AgentJobFailureKindMalformedOutput, result.Kind)
	assert.Equal(t, mcp.AgentJobFailureOutcomeBlock, result.Outcome)
	assert.Equal(t, "recoverable", result.LockState)
}
