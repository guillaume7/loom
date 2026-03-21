package runtime

// OutcomeAction is the concrete runtime action that corresponds to a policy outcome.
type OutcomeAction string

const (
	// OutcomeActionScheduleWake schedules the next poll wake-up and keeps the
	// current FSM state. Applied for PolicyOutcomeWait and PolicyOutcomeRetry.
	OutcomeActionScheduleWake OutcomeAction = "schedule_wake"

	// OutcomeActionHaltProgress stops forward progress without scheduling a
	// wake-up. Applied for PolicyOutcomeBlock.
	OutcomeActionHaltProgress OutcomeAction = "halt_progress"

	// OutcomeActionRequestOperator requests operator attention and transitions
	// the session to DEBUGGING or PAUSED. Applied for PolicyOutcomeEscalate.
	OutcomeActionRequestOperator OutcomeAction = "request_operator"

	// OutcomeActionContinue advances to the next FSM state without scheduling
	// a wake-up. Applied for PolicyOutcomeContinue.
	OutcomeActionContinue OutcomeAction = "continue"
)

// OutcomeToAction maps a PolicyOutcome to its concrete runtime action.
//
//   - Continue → OutcomeActionContinue        (advance FSM state, no wake)
//   - Wait     → OutcomeActionScheduleWake    (stay in state, schedule next poll)
//   - Retry    → OutcomeActionScheduleWake    (stay in state, schedule next poll)
//   - Block    → OutcomeActionHaltProgress    (stay in state, no wake scheduled)
//   - Escalate → OutcomeActionRequestOperator (transition to DEBUGGING/PAUSED)
func OutcomeToAction(outcome PolicyOutcome) OutcomeAction {
	switch outcome {
	case PolicyOutcomeContinue:
		return OutcomeActionContinue
	case PolicyOutcomeWait, PolicyOutcomeRetry:
		return OutcomeActionScheduleWake
	case PolicyOutcomeBlock:
		return OutcomeActionHaltProgress
	case PolicyOutcomeEscalate:
		return OutcomeActionRequestOperator
	default:
		return OutcomeActionHaltProgress
	}
}

// EscalationCondition names a specific condition that triggers escalation or
// a blocking halt requiring operator attention. These constants match the Reason
// strings produced by the policy evaluation functions.
//
// Conditions that produce PolicyOutcomeEscalate transition the session to an
// operator-visible state (e.g. DEBUGGING). Conditions that produce
// PolicyOutcomeBlock halt forward progress without scheduling a wake-up but do
// not require immediate operator action.
type EscalationCondition string

const (
	// EscalationCIFailed is produced by EvaluateCIReadiness when one or more CI
	// checks have concluded with a failure, error, cancellation, or timeout.
	// Produces PolicyOutcomeEscalate → OutcomeActionRequestOperator.
	EscalationCIFailed EscalationCondition = "ci_failed"

	// BlockConditionMergeBlockedByCI is produced by EvaluateMergeReadiness when
	// CI has not succeeded (and is not merely pending), blocking the merge gate.
	// Produces PolicyOutcomeBlock → OutcomeActionHaltProgress.
	BlockConditionMergeBlockedByCI EscalationCondition = "merge_blocked_by_ci"

	// BlockConditionMergeObservationsStale is produced by EvaluateMergeReadiness
	// when one or both observations have exceeded the staleness threshold,
	// halting the merge gate until fresh observations are recorded.
	// Produces PolicyOutcomeBlock → OutcomeActionHaltProgress.
	BlockConditionMergeObservationsStale EscalationCondition = "merge_observations_stale"
)
