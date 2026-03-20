package runtime_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	loomgh "github.com/guillaume7/loom/internal/github"
	loomruntime "github.com/guillaume7/loom/internal/runtime"
	"github.com/guillaume7/loom/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssembleObservationModel_UsesPersistedRuntimeRecords(t *testing.T) {
	ctx := context.Background()
	st, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, st.Close())
	})

	now := time.Date(2026, 3, 20, 16, 0, 0, 0, time.UTC)
	controller := loomruntime.NewController(st, loomruntime.Config{
		HolderID:     "controller-1",
		LeaseTTL:     time.Minute,
		PollInterval: time.Minute,
		Now:          func() time.Time { return now },
	})

	require.NoError(t, st.WriteCheckpoint(ctx, store.Checkpoint{State: "AWAITING_READY", Phase: 2, PRNumber: 42}))
	_, err = controller.ObserveGitHubEvent(ctx, loomruntime.GitHubEvent{
		DeliveryID: "evt-pr",
		EventType:  "pull_request",
		Action:     "synchronize",
		Repository: "guillaume7/loom",
		PRNumber:   42,
		Payload:    json.RawMessage(`{"pull_request":{"draft":true,"head":{"ref":"feature/runtime-observation","sha":"abc123"},"base":{"ref":"main"}}}`),
	})
	require.NoError(t, err)

	require.NoError(t, st.WriteCheckpoint(ctx, store.Checkpoint{State: "AWAITING_CI", Phase: 2, PRNumber: 42}))
	_, err = controller.ObserveGitHubEvent(ctx, loomruntime.GitHubEvent{
		DeliveryID: "evt-ci",
		EventType:  "check_suite",
		Repository: "guillaume7/loom",
		PRNumber:   42,
		Payload:    json.RawMessage(`{"check_suite":{"conclusion":"success","head_branch":"feature/runtime-observation","head_sha":"abc123"}}`),
	})
	require.NoError(t, err)

	_, err = controller.ProcessDueWake(ctx, &pollingGitHubClientMock{
		pr: &loomgh.PR{Number: 42, HeadRef: "feature/runtime-observation", BaseRef: "main", HeadSHA: "abc123"},
		checkRuns: []*loomgh.CheckRun{
			{Name: "build", Status: "completed", Conclusion: "success"},
			{Name: "lint", Status: "completed", Conclusion: "success"},
		},
	})
	require.NoError(t, err)

	_, err = controller.ObserveGitHubEvent(ctx, loomruntime.GitHubEvent{
		DeliveryID: "evt-review",
		EventType:  "pull_request_review",
		Action:     "submitted",
		Repository: "guillaume7/loom",
		PRNumber:   42,
		Payload:    json.RawMessage(`{"review":{"state":"approved"},"pull_request":{"head":{"ref":"feature/runtime-observation","sha":"abc123"},"base":{"ref":"main"}}}`),
	})
	require.NoError(t, err)

	_, err = controller.ApplyManualOverride(ctx, loomruntime.ManualOverrideRequest{
		Action:      loomruntime.ManualOverridePause,
		Source:      "cli",
		RequestedBy: "octocat",
		Reason:      "hold merge",
	})
	require.NoError(t, err)

	model, err := loomruntime.AssembleObservationModel(ctx, st)
	require.NoError(t, err)

	assert.Equal(t, "default", model.SessionID)
	assert.Equal(t, loomruntime.ObservationAuthorityAuthoritative, model.Checkpoint.Authority)
	assert.Equal(t, "PAUSED", model.Checkpoint.State)

	ciPoll := findCIObservation(model.CI, "poll_ci")
	if assert.NotNil(t, ciPoll) {
		assert.Equal(t, loomruntime.ObservationAuthorityAuthoritative, ciPoll.Authority)
		assert.Equal(t, 2, ciPoll.TotalChecks)
		assert.Equal(t, 2, ciPoll.GreenChecks)
	}
	ciEvent := findCIObservation(model.CI, "check_suite")
	if assert.NotNil(t, ciEvent) {
		assert.Equal(t, "success", ciEvent.Conclusion)
	}

	review := findReviewObservation(model.Review, "pull_request_review")
	if assert.NotNil(t, review) {
		assert.Equal(t, loomruntime.ObservationAuthorityAuthoritative, review.Authority)
		assert.Equal(t, "approved", review.Status)
	}

	prObservation := findPRObservation(model.PR, "pull_request")
	if assert.NotNil(t, prObservation) {
		assert.Equal(t, 42, prObservation.PRNumber)
		if assert.NotNil(t, prObservation.Draft) {
			assert.True(t, *prObservation.Draft)
		}
	}

	branch := findBranchObservation(model.Branch, "feature/runtime-observation")
	if assert.NotNil(t, branch) {
		assert.Equal(t, loomruntime.ObservationAuthorityAuthoritative, branch.Authority)
		assert.Equal(t, "main", branch.BaseRef)
		assert.Equal(t, "abc123", branch.HeadSHA)
	}

	operator := findOperatorObservation(model.Operator, loomruntime.ManualOverridePause)
	if assert.NotNil(t, operator) {
		assert.Equal(t, loomruntime.ObservationAuthorityAuthoritative, operator.Authority)
		assert.Equal(t, "octocat", operator.RequestedBy)
		assert.Equal(t, "hold merge", operator.Reason)
	}

	if assert.NotNil(t, model.Summaries.CI) {
		assert.Equal(t, loomruntime.ObservationAuthorityDerived, model.Summaries.CI.Authority)
		assert.Equal(t, "resume", model.Summaries.CI.Verdict)
	}
	if assert.NotNil(t, model.Summaries.Operator) {
		assert.Equal(t, loomruntime.ObservationAuthorityDerived, model.Summaries.Operator.Authority)
		assert.Equal(t, loomruntime.ManualOverridePause, model.Summaries.Operator.Action)
	}
	if assert.NotNil(t, model.Summaries.Branch) {
		assert.Equal(t, loomruntime.ObservationAuthorityDerived, model.Summaries.Branch.Authority)
		assert.Equal(t, "feature/runtime-observation", model.Summaries.Branch.HeadRef)
	}
}

func TestAssembleObservationModel_KeepsDerivedSummariesSeparateFromAuthoritativeObservations(t *testing.T) {
	ctx := context.Background()
	st := newMemStore()
	now := time.Date(2026, 3, 20, 16, 5, 0, 0, time.UTC)

	require.NoError(t, st.WriteCheckpoint(ctx, store.Checkpoint{State: "AWAITING_CI", Phase: 2, PRNumber: 42}))
	require.NoError(t, st.WritePolicyDecision(ctx, store.PolicyDecision{
		SessionID:     "default",
		CorrelationID: "corr-ci",
		DecisionKind:  "runtime_poll",
		Verdict:       "resume",
		InputHash:     "hash-ci",
		Detail:        `{"session_id":"default","correlation_id":"corr-ci","wake_kind":"poll_ci","pr_number":42,"decision_verdict":"resume","ci":{"total_checks":2,"green_checks":2}}`,
		CreatedAt:     now,
	}))

	model, err := loomruntime.AssembleObservationModel(ctx, st)
	require.NoError(t, err)

	assert.Empty(t, model.CI)
	if assert.NotNil(t, model.Summaries.CI) {
		assert.Equal(t, loomruntime.ObservationAuthorityDerived, model.Summaries.CI.Authority)
		assert.Equal(t, "resume", model.Summaries.CI.Verdict)
		assert.Equal(t, "corr-ci", model.Summaries.CI.CorrelationID)
	}
}

func TestAssembleObservationModel_IgnoresUnrelatedPersistedGitHubEvents(t *testing.T) {
	ctx := context.Background()
	st, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, st.Close())
	})

	now := time.Date(2026, 3, 20, 16, 10, 0, 0, time.UTC)
	controller := loomruntime.NewController(st, loomruntime.Config{
		HolderID:     "controller-1",
		LeaseTTL:     time.Minute,
		PollInterval: time.Minute,
		Now:          func() time.Time { return now },
	})

	require.NoError(t, st.WriteCheckpoint(ctx, store.Checkpoint{State: "AWAITING_CI", Phase: 2, PRNumber: 42}))
	_, err = controller.ObserveGitHubEvent(ctx, loomruntime.GitHubEvent{
		DeliveryID: "evt-relevant",
		EventType:  "check_suite",
		Repository: "guillaume7/loom",
		PRNumber:   42,
		Payload:    json.RawMessage(`{"check_suite":{"conclusion":"success","head_branch":"feature/runtime-observation","head_sha":"abc123"}}`),
	})
	require.NoError(t, err)

	_, err = controller.ObserveGitHubEvent(ctx, loomruntime.GitHubEvent{
		DeliveryID: "evt-other-pr",
		EventType:  "check_suite",
		Repository: "guillaume7/loom",
		PRNumber:   7,
		Payload:    json.RawMessage(`{"check_suite":{"conclusion":"failure","head_branch":"feature/other-pr","head_sha":"def456"}}`),
	})
	require.NoError(t, err)

	_, err = controller.ObserveGitHubEvent(ctx, loomruntime.GitHubEvent{
		DeliveryID: "evt-unsupported",
		EventType:  "issue_comment",
		Repository: "guillaume7/loom",
		PRNumber:   42,
		Payload:    json.RawMessage(`{"pull_request":{"head":{"ref":"feature/ignored-comment","sha":"comment123"},"base":{"ref":"main"}}}`),
	})
	require.NoError(t, err)

	model, err := loomruntime.AssembleObservationModel(ctx, st)
	require.NoError(t, err)

	require.Len(t, model.CI, 1)
	assert.Equal(t, "success", model.CI[0].Conclusion)
	require.Len(t, model.Branch, 1)
	assert.NotNil(t, findBranchObservation(model.Branch, "feature/runtime-observation"))
	assert.Nil(t, findBranchObservation(model.Branch, "feature/other-pr"))
	assert.Nil(t, findBranchObservation(model.Branch, "feature/ignored-comment"))
	assert.NotNil(t, model.Summaries.CI)
	assert.Equal(t, "success", model.Summaries.CI.Conclusion)
}

func TestAssembleObservationModel_DerivesBranchFromPollingRecordsWithoutGitHubEvents(t *testing.T) {
	ctx := context.Background()
	st, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, st.Close())
	})

	now := time.Date(2026, 3, 20, 16, 15, 0, 0, time.UTC)
	controller := loomruntime.NewController(st, loomruntime.Config{
		HolderID:     "controller-1",
		LeaseTTL:     time.Minute,
		PollInterval: time.Minute,
		Now:          func() time.Time { return now },
	})

	require.NoError(t, st.WriteCheckpoint(ctx, store.Checkpoint{State: "REVIEWING", Phase: 2, PRNumber: 42}))
	require.NoError(t, st.UpsertWakeSchedule(ctx, store.WakeSchedule{
		SessionID: "default",
		WakeKind:  "poll_review",
		DueAt:     now.Add(-time.Minute),
		DedupeKey: "run:default:poll_review",
	}))

	_, err = controller.ProcessDueWake(ctx, &pollingGitHubClientMock{
		pr:           &loomgh.PR{Number: 42, HeadRef: "feature/poll-only", BaseRef: "main", HeadSHA: "abc123"},
		reviewStatus: "PENDING",
	})
	require.NoError(t, err)

	model, err := loomruntime.AssembleObservationModel(ctx, st)
	require.NoError(t, err)

	require.Len(t, model.Review, 1)
	assert.Equal(t, "poll_review", model.Review[0].EventKind)
	branch := findBranchObservation(model.Branch, "feature/poll-only")
	if assert.NotNil(t, branch) {
		assert.Equal(t, "main", branch.BaseRef)
		assert.Equal(t, "abc123", branch.HeadSHA)
	}
	if assert.NotNil(t, model.Summaries.Branch) {
		assert.Equal(t, "feature/poll-only", model.Summaries.Branch.HeadRef)
	}
}

func findCIObservation(observations []loomruntime.CIObservation, eventKind string) *loomruntime.CIObservation {
	for index := range observations {
		if observations[index].EventKind == eventKind {
			return &observations[index]
		}
	}
	return nil
}

func findReviewObservation(observations []loomruntime.ReviewObservation, eventKind string) *loomruntime.ReviewObservation {
	for index := range observations {
		if observations[index].EventKind == eventKind {
			return &observations[index]
		}
	}
	return nil
}

func findPRObservation(observations []loomruntime.PRObservation, eventKind string) *loomruntime.PRObservation {
	for index := range observations {
		if observations[index].EventKind == eventKind {
			return &observations[index]
		}
	}
	return nil
}

func findBranchObservation(observations []loomruntime.BranchObservation, headRef string) *loomruntime.BranchObservation {
	for index := range observations {
		if observations[index].HeadRef == headRef {
			return &observations[index]
		}
	}
	return nil
}

func findOperatorObservation(observations []loomruntime.OperatorObservation, action string) *loomruntime.OperatorObservation {
	for index := range observations {
		if observations[index].Action == action {
			return &observations[index]
		}
	}
	return nil
}
