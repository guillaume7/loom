package mcp

import (
	"time"

	"github.com/guillaume7/loom/internal/agentspawn"
)

// AgentJobFailureKind identifies the root cause of a failed agent job exit.
type AgentJobFailureKind string

const (
	// AgentJobFailureKindTimeout indicates the agent process was observed to
	// exit after its contract deadline.
	AgentJobFailureKindTimeout AgentJobFailureKind = "timeout"

	// AgentJobFailureKindCrash indicates the agent process exited with a
	// non-zero exit code before its contract deadline.
	AgentJobFailureKindCrash AgentJobFailureKind = "crash"

	// AgentJobFailureKindMalformedOutput indicates the agent produced output
	// that does not satisfy the contract's ExpectedOutput specification.
	// This kind is representable before a real output parser is wired.
	AgentJobFailureKindMalformedOutput AgentJobFailureKind = "malformed_output"

	// AgentJobFailureKindCleanupFailure indicates the worktree cleanup failed
	// after an otherwise zero-exit agent run.
	AgentJobFailureKindCleanupFailure AgentJobFailureKind = "cleanup_failure"
)

// AgentJobFailureOutcome is the explicit containment action the runtime should
// take in response to a classified agent job failure.
type AgentJobFailureOutcome string

const (
	// AgentJobFailureOutcomeRetry indicates the job may be retried
	// automatically by the scheduler on the next evaluation pass.
	AgentJobFailureOutcomeRetry AgentJobFailureOutcome = "retry"

	// AgentJobFailureOutcomeEscalate indicates the runtime cannot recover
	// automatically and operator intervention is required.
	AgentJobFailureOutcomeEscalate AgentJobFailureOutcome = "escalate"

	// AgentJobFailureOutcomeBlock indicates the story is blocked and cannot
	// proceed without explicit external action.
	AgentJobFailureOutcomeBlock AgentJobFailureOutcome = "block"
)

// AgentJobFailureResult captures the explicit containment outcome for a
// failed agent job. It is embedded in the background_agent_failed action
// payload to support operator review and eventual replay.
type AgentJobFailureResult struct {
	// Kind is the root cause classification of the failure.
	Kind AgentJobFailureKind `json:"failure_kind"`
	// Outcome is the containment action the runtime should take.
	Outcome AgentJobFailureOutcome `json:"outcome"`
	// LockState describes the state of run leases and PR locks after failure.
	// "recoverable" confirms the runtime is not silently holding ownership,
	// satisfying the AC2 requirement for explicit lease recovery state.
	LockState string `json:"lock_state"`
}

// ClassifyAgentJobExit maps an observed Exit to an explicit AgentJobFailureResult.
// It returns (result, true) when the job has failed, and (zero, false) for a
// clean success.
//
// Classification rules applied in priority order:
//  1. exitCode == 0 and cleanupErr == nil  → clean success, no failure.
//  2. exitCode == 0 and cleanupErr != nil  → cleanup_failure + escalate.
//  3. observedAt after contract.Deadline   → timeout + retry.
//  4. non-zero exitCode                    → crash + retry.
//
// All failure kinds carry LockState "recoverable" to make explicit that a
// failed job does not silently hold run leases or PR locks.
func ClassifyAgentJobExit(exit agentspawn.Exit, contract agentspawn.JobContract, observedAt time.Time) (AgentJobFailureResult, bool) {
	if exit.ExitCode == 0 && exit.CleanupErr == nil {
		return AgentJobFailureResult{}, false
	}

	if exit.ExitCode == 0 && exit.CleanupErr != nil {
		return AgentJobFailureResult{
			Kind:      AgentJobFailureKindCleanupFailure,
			Outcome:   AgentJobFailureOutcomeEscalate,
			LockState: "recoverable",
		}, true
	}

	if observedAt.After(contract.Deadline) {
		return AgentJobFailureResult{
			Kind:      AgentJobFailureKindTimeout,
			Outcome:   AgentJobFailureOutcomeRetry,
			LockState: "recoverable",
		}, true
	}

	return AgentJobFailureResult{
		Kind:      AgentJobFailureKindCrash,
		Outcome:   AgentJobFailureOutcomeRetry,
		LockState: "recoverable",
	}, true
}

// ClassifyMalformedOutput returns the AgentJobFailureResult for a malformed
// output case. This helper is available before a real output parser is wired,
// so the contract can represent and record malformed-output failures for
// operator review and eventual replay.
func ClassifyMalformedOutput() AgentJobFailureResult {
	return AgentJobFailureResult{
		Kind:      AgentJobFailureKindMalformedOutput,
		Outcome:   AgentJobFailureOutcomeBlock,
		LockState: "recoverable",
	}
}
