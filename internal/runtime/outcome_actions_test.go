package runtime_test

import (
	"testing"
	"time"

	loomruntime "github.com/guillaume7/loom/internal/runtime"
	"github.com/stretchr/testify/assert"
)

func TestOutcomeToAction_MapsAllOutcomesToExpectedActions(t *testing.T) {
	tests := []struct {
		outcome loomruntime.PolicyOutcome
		want    loomruntime.OutcomeAction
	}{
		{loomruntime.PolicyOutcomeContinue, loomruntime.OutcomeActionContinue},
		{loomruntime.PolicyOutcomeWait, loomruntime.OutcomeActionScheduleWake},
		{loomruntime.PolicyOutcomeRetry, loomruntime.OutcomeActionScheduleWake},
		{loomruntime.PolicyOutcomeBlock, loomruntime.OutcomeActionHaltProgress},
		{loomruntime.PolicyOutcomeEscalate, loomruntime.OutcomeActionRequestOperator},
	}

	for _, tc := range tests {
		t.Run(string(tc.outcome), func(t *testing.T) {
			assert.Equal(t, tc.want, loomruntime.OutcomeToAction(tc.outcome))
		})
	}
}

func TestEscalationCondition_ValuesMatchPolicyReasons(t *testing.T) {
	now := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	staleAfter := 10 * time.Minute

	t.Run("EscalationCIFailed matches ci_failed reason", func(t *testing.T) {
		decision := loomruntime.EvaluateCIReadiness(loomruntime.CIReadinessInput{
			Checkpoint: loomruntime.CheckpointObservation{State: "AWAITING_CI"},
			Observation: &loomruntime.CIObservation{
				ObservedAt:   now.Add(-time.Minute),
				FailedChecks: []string{"build"},
				TotalChecks:  1,
			},
			EvaluatedAt: now,
			StaleAfter:  staleAfter,
		})
		assert.Equal(t, string(loomruntime.EscalationCIFailed), decision.Reason)
		assert.Equal(t, loomruntime.PolicyOutcomeEscalate, decision.Outcome)
	})

	t.Run("BlockConditionMergeBlockedByCI matches merge_blocked_by_ci reason", func(t *testing.T) {
		observedAt := now.Add(-time.Minute)
		decision := loomruntime.EvaluateMergeReadiness(loomruntime.MergeReadinessInput{
			Model: loomruntime.ObservationModel{
				CI: []loomruntime.CIObservation{
					{ObservedAt: observedAt, Conclusion: "failure", TotalChecks: 1, GreenChecks: 0},
				},
				Review: []loomruntime.ReviewObservation{
					{ObservedAt: observedAt, Status: "APPROVED"},
				},
			},
			EvaluatedAt: now,
			StaleAfter:  staleAfter,
		})
		assert.Equal(t, string(loomruntime.BlockConditionMergeBlockedByCI), decision.Reason)
		assert.Equal(t, loomruntime.PolicyOutcomeBlock, decision.Outcome)
	})

	t.Run("BlockConditionMergeObservationsStale matches merge_observations_stale reason", func(t *testing.T) {
		staleAt := now.Add(-2 * staleAfter)
		decision := loomruntime.EvaluateMergeReadiness(loomruntime.MergeReadinessInput{
			Model: loomruntime.ObservationModel{
				CI: []loomruntime.CIObservation{
					{ObservedAt: staleAt, Conclusion: "success", TotalChecks: 1, GreenChecks: 1},
				},
				Review: []loomruntime.ReviewObservation{
					{ObservedAt: staleAt, Status: "APPROVED"},
				},
			},
			EvaluatedAt: now,
			StaleAfter:  staleAfter,
		})
		assert.Equal(t, string(loomruntime.BlockConditionMergeObservationsStale), decision.Reason)
		assert.Equal(t, loomruntime.PolicyOutcomeBlock, decision.Outcome)
	})
}
