package runtime_test

import (
	"context"
	"testing"
	"time"

	loomruntime "github.com/guillaume7/loom/internal/runtime"
	"github.com/guillaume7/loom/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvaluateCIReadiness_Outcomes(t *testing.T) {
	evaluatedAt := time.Date(2026, 3, 20, 18, 0, 0, 0, time.UTC)
	checkpoint := loomruntime.CheckpointObservation{State: "AWAITING_CI", PRNumber: 42}

	t.Run("green continues", func(t *testing.T) {
		decision := loomruntime.EvaluateCIReadiness(loomruntime.CIReadinessInput{
			Checkpoint: checkpoint,
			Observation: &loomruntime.CIObservation{
				ObservedAt:  evaluatedAt.Add(-time.Minute),
				EventKind:   "poll_ci",
				TotalChecks: 2,
				GreenChecks: 2,
			},
			EvaluatedAt: evaluatedAt,
		})
		assert.Equal(t, loomruntime.PolicyDecisionCIReadiness, decision.Name)
		assert.Equal(t, loomruntime.PolicyOutcomeContinue, decision.Outcome)
		assert.Equal(t, "ci_green", decision.Reason)
	})

	t.Run("pending waits", func(t *testing.T) {
		decision := loomruntime.EvaluateCIReadiness(loomruntime.CIReadinessInput{
			Checkpoint: checkpoint,
			Observation: &loomruntime.CIObservation{
				ObservedAt:    evaluatedAt.Add(-time.Minute),
				EventKind:     "poll_ci",
				TotalChecks:   2,
				GreenChecks:   1,
				PendingChecks: []string{"lint"},
			},
			EvaluatedAt: evaluatedAt,
		})
		assert.Equal(t, loomruntime.PolicyOutcomeWait, decision.Outcome)
		assert.Equal(t, "ci_pending", decision.Reason)
	})

	t.Run("failed escalates", func(t *testing.T) {
		decision := loomruntime.EvaluateCIReadiness(loomruntime.CIReadinessInput{
			Checkpoint: checkpoint,
			Observation: &loomruntime.CIObservation{
				ObservedAt:   evaluatedAt.Add(-time.Minute),
				EventKind:    "poll_ci",
				FailedChecks: []string{"build"},
			},
			EvaluatedAt: evaluatedAt,
		})
		assert.Equal(t, loomruntime.PolicyOutcomeEscalate, decision.Outcome)
		assert.Equal(t, "ci_failed", decision.Reason)
	})

	t.Run("missing observation retries", func(t *testing.T) {
		decision := loomruntime.EvaluateCIReadiness(loomruntime.CIReadinessInput{
			Checkpoint:  checkpoint,
			EvaluatedAt: evaluatedAt,
		})
		assert.Equal(t, loomruntime.PolicyOutcomeRetry, decision.Outcome)
		assert.Equal(t, "ci_observation_missing", decision.Reason)
	})
}

func TestEvaluateReviewReadiness_Outcomes(t *testing.T) {
	evaluatedAt := time.Date(2026, 3, 20, 18, 5, 0, 0, time.UTC)
	checkpoint := loomruntime.CheckpointObservation{State: "REVIEWING", PRNumber: 42}

	t.Run("approved continues", func(t *testing.T) {
		decision := loomruntime.EvaluateReviewReadiness(loomruntime.ReviewReadinessInput{
			Checkpoint: checkpoint,
			Observation: &loomruntime.ReviewObservation{
				ObservedAt: evaluatedAt.Add(-time.Minute),
				Status:     "APPROVED",
			},
			EvaluatedAt: evaluatedAt,
		})
		assert.Equal(t, loomruntime.PolicyOutcomeContinue, decision.Outcome)
		assert.Equal(t, "review_approved", decision.Reason)
	})

	t.Run("changes requested blocks", func(t *testing.T) {
		decision := loomruntime.EvaluateReviewReadiness(loomruntime.ReviewReadinessInput{
			Checkpoint: checkpoint,
			Observation: &loomruntime.ReviewObservation{
				ObservedAt: evaluatedAt.Add(-time.Minute),
				Status:     "CHANGES_REQUESTED",
			},
			EvaluatedAt: evaluatedAt,
		})
		assert.Equal(t, loomruntime.PolicyOutcomeBlock, decision.Outcome)
		assert.Equal(t, "changes_requested", decision.Reason)
	})

	t.Run("pending waits", func(t *testing.T) {
		decision := loomruntime.EvaluateReviewReadiness(loomruntime.ReviewReadinessInput{
			Checkpoint: checkpoint,
			Observation: &loomruntime.ReviewObservation{
				ObservedAt: evaluatedAt.Add(-time.Minute),
				Status:     "PENDING",
			},
			EvaluatedAt: evaluatedAt,
		})
		assert.Equal(t, loomruntime.PolicyOutcomeWait, decision.Outcome)
		assert.Equal(t, "review_pending", decision.Reason)
	})

	t.Run("stale observation retries", func(t *testing.T) {
		decision := loomruntime.EvaluateReviewReadiness(loomruntime.ReviewReadinessInput{
			Checkpoint: checkpoint,
			Observation: &loomruntime.ReviewObservation{
				ObservedAt: evaluatedAt.Add(-3 * loomruntime.DefaultObservationStaleAfter),
				Status:     "PENDING",
			},
			EvaluatedAt: evaluatedAt,
		})
		assert.Equal(t, loomruntime.PolicyOutcomeRetry, decision.Outcome)
		assert.Equal(t, "review_observation_stale", decision.Reason)
	})
}

func TestEvaluateMergeReadiness_UsesPersistedObservationsConservatively(t *testing.T) {
	ctx := context.Background()
	st := newMemStore()
	evaluatedAt := time.Date(2026, 3, 20, 18, 10, 0, 0, time.UTC)

	require.NoError(t, st.WriteCheckpoint(ctx, store.Checkpoint{State: "MERGING", Phase: 2, PRNumber: 42}))
	require.NoError(t, st.WriteExternalEvent(ctx, store.ExternalEvent{
		SessionID:     "default",
		EventSource:   "poll",
		EventKind:     "poll_ci",
		CorrelationID: "corr-ci",
		ObservedAt:    evaluatedAt.Add(-time.Minute),
		Payload:       `{"session_id":"default","correlation_id":"corr-ci","wake_kind":"poll_ci","pr_number":42,"decision_verdict":"resume","ci":{"total_checks":2,"green_checks":2}}`,
	}))
	require.NoError(t, st.WriteExternalEvent(ctx, store.ExternalEvent{
		SessionID:     "default",
		EventSource:   "poll",
		EventKind:     "poll_review",
		CorrelationID: "corr-review",
		ObservedAt:    evaluatedAt.Add(-time.Minute),
		Payload:       `{"session_id":"default","correlation_id":"corr-review","wake_kind":"poll_review","pr_number":42,"decision_verdict":"resume","review_status":"APPROVED"}`,
	}))

	model, err := loomruntime.AssembleObservationModel(ctx, st)
	require.NoError(t, err)

	decision := loomruntime.EvaluateMergeReadiness(loomruntime.MergeReadinessInput{
		Model:       model,
		EvaluatedAt: evaluatedAt,
	})
	assert.Equal(t, loomruntime.PolicyDecisionMergeReadiness, decision.Name)
	assert.Equal(t, loomruntime.PolicyOutcomeContinue, decision.Outcome)
	assert.Equal(t, "merge_ready", decision.Reason)

	decision = loomruntime.EvaluateMergeReadiness(loomruntime.MergeReadinessInput{
		Model:       model,
		EvaluatedAt: evaluatedAt.Add(3 * loomruntime.DefaultObservationStaleAfter),
	})
	assert.Equal(t, loomruntime.PolicyOutcomeBlock, decision.Outcome)
	assert.Equal(t, "merge_observations_stale", decision.Reason)

	emptyDecision := loomruntime.EvaluateMergeReadiness(loomruntime.MergeReadinessInput{
		Model:       loomruntime.ObservationModel{Checkpoint: loomruntime.CheckpointObservation{State: "MERGING", PRNumber: 42}},
		EvaluatedAt: evaluatedAt,
	})
	assert.Equal(t, loomruntime.PolicyOutcomeBlock, emptyDecision.Outcome)
	assert.Equal(t, "merge_observations_incomplete", emptyDecision.Reason)
}
