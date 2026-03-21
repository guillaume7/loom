package runtime_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	loomgh "github.com/guillaume7/loom/internal/github"
	loomruntime "github.com/guillaume7/loom/internal/runtime"
	"github.com/guillaume7/loom/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestControllerProcessDueWake_PRReadyUsesPersistedCheckpointState(t *testing.T) {
	now := time.Date(2026, 3, 20, 14, 0, 0, 0, time.UTC)
	st := newMemStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "AWAITING_READY", Phase: 2, PRNumber: 42}))
	require.NoError(t, st.UpsertWakeSchedule(context.Background(), store.WakeSchedule{
		SessionID: "default",
		WakeKind:  "poll_pr_ready",
		DueAt:     now.Add(-time.Minute),
		DedupeKey: "run:default:poll_pr_ready",
	}))

	controller := loomruntime.NewController(st, loomruntime.Config{
		HolderID:     "controller-1",
		LeaseTTL:     time.Minute,
		PollInterval: 30 * time.Second,
		Now:          func() time.Time { return now },
	})

	gh := &pollingGitHubClientMock{pr: &loomgh.PR{Number: 42, Draft: false}}
	lifecycle, err := controller.ProcessDueWake(context.Background(), gh)
	require.NoError(t, err)

	assert.Equal(t, "AWAITING_CI", lifecycle.WorkflowState)
	assert.Equal(t, loomruntime.ControllerStateClaimed, lifecycle.Controller)
	assert.Equal(t, "poll_ci", lifecycle.NextWakeKind)
	assert.Equal(t, now.Add(30*time.Second), lifecycle.NextWakeAt)
	assert.Equal(t, "poll_observation", lifecycle.DrivenBy)
	assert.Equal(t, 0, gh.markReadyCalls)

	cp, err := st.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "AWAITING_CI", cp.State)
	assert.Zero(t, cp.RetryCount)

	require.Len(t, st.actions, 1)
	assert.Equal(t, "pr_ready", st.actions[0].Event)
	require.Len(t, st.events, 1)
	assert.Equal(t, "poll_pr_ready", st.events[0].EventKind)
	require.Len(t, st.decisions, 1)
	assert.Equal(t, "resume", st.decisions[0].Verdict)
}

func TestControllerProcessDueWake_CIPollWritesObservationAndSchedulesReviewPolling(t *testing.T) {
	now := time.Date(2026, 3, 20, 14, 5, 0, 0, time.UTC)
	st := newMemStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "AWAITING_CI", Phase: 2, PRNumber: 42}))
	require.NoError(t, st.UpsertWakeSchedule(context.Background(), store.WakeSchedule{
		SessionID: "default",
		WakeKind:  "poll_ci",
		DueAt:     now.Add(-time.Minute),
		DedupeKey: "run:default:poll_ci",
	}))

	controller := loomruntime.NewController(st, loomruntime.Config{
		HolderID:     "controller-1",
		LeaseTTL:     time.Minute,
		PollInterval: 45 * time.Second,
		Now:          func() time.Time { return now },
	})

	gh := &pollingGitHubClientMock{
		pr: &loomgh.PR{Number: 42, HeadSHA: "abc123"},
		checkRuns: []*loomgh.CheckRun{
			{Name: "build", Status: "completed", Conclusion: "success"},
			{Name: "lint", Status: "completed", Conclusion: "success"},
		},
	}
	lifecycle, err := controller.ProcessDueWake(context.Background(), gh)
	require.NoError(t, err)

	assert.Equal(t, "REVIEWING", lifecycle.WorkflowState)
	assert.Equal(t, loomruntime.ControllerStateClaimed, lifecycle.Controller)
	assert.Equal(t, "poll_review", lifecycle.NextWakeKind)
	assert.Equal(t, now.Add(45*time.Second), lifecycle.NextWakeAt)
	assert.Equal(t, "poll_observation", lifecycle.DrivenBy)
	assert.Zero(t, gh.requestReviewCalls)

	cp, err := st.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "REVIEWING", cp.State)

	require.Len(t, st.events, 1)
	assert.Equal(t, "poll_ci", st.events[0].EventKind)

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(st.events[0].Payload), &payload))
	assert.Equal(t, "ci_green", payload["outcome"])
	assert.Equal(t, "resume", payload["decision_verdict"])
	assert.Equal(t, "ci_readiness", payload["policy_decision"])
	assert.Equal(t, "continue", payload["policy_outcome"])

	require.Len(t, st.decisions, 1)
	assert.Equal(t, "ci_readiness", st.decisions[0].DecisionKind)
	assert.Equal(t, "continue", st.decisions[0].Verdict)
}

func TestControllerProcessDueWake_CIGreenDoesNotMutateGitHubBeforePersistence(t *testing.T) {
	now := time.Date(2026, 3, 20, 14, 7, 0, 0, time.UTC)
	st := newMemStore()
	st.checkpointActionErr = errors.New("checkpoint write failed")
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "AWAITING_CI", Phase: 2, PRNumber: 42}))
	require.NoError(t, st.UpsertWakeSchedule(context.Background(), store.WakeSchedule{
		SessionID: "default",
		WakeKind:  "poll_ci",
		DueAt:     now.Add(-time.Minute),
		DedupeKey: "run:default:poll_ci",
	}))

	controller := loomruntime.NewController(st, loomruntime.Config{
		HolderID:     "controller-1",
		LeaseTTL:     time.Minute,
		PollInterval: 45 * time.Second,
		Now:          func() time.Time { return now },
	})

	gh := &pollingGitHubClientMock{
		pr:        &loomgh.PR{Number: 42, HeadSHA: "abc123"},
		checkRuns: []*loomgh.CheckRun{{Name: "build", Status: "completed", Conclusion: "success"}},
	}

	_, err := controller.ProcessDueWake(context.Background(), gh)
	require.ErrorContains(t, err, "checkpoint write failed")
	assert.Zero(t, gh.requestReviewCalls)

	cp, readErr := st.ReadCheckpoint(context.Background())
	require.NoError(t, readErr)
	assert.Equal(t, "AWAITING_CI", cp.State)
	assert.Empty(t, st.actions)
	assert.Empty(t, st.events)
	assert.Empty(t, st.decisions)
}

func TestControllerProcessDueWake_CIFailedTransitionsToDebugging(t *testing.T) {
	now := time.Date(2026, 3, 20, 14, 8, 0, 0, time.UTC)
	st := newMemStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "AWAITING_CI", Phase: 2, PRNumber: 42}))
	require.NoError(t, st.UpsertWakeSchedule(context.Background(), store.WakeSchedule{
		SessionID: "default",
		WakeKind:  "poll_ci",
		DueAt:     now.Add(-time.Minute),
		DedupeKey: "run:default:poll_ci",
	}))

	controller := loomruntime.NewController(st, loomruntime.Config{
		HolderID:     "controller-1",
		LeaseTTL:     time.Minute,
		PollInterval: 45 * time.Second,
		Now:          func() time.Time { return now },
	})

	gh := &pollingGitHubClientMock{
		pr:        &loomgh.PR{Number: 42, HeadSHA: "abc123"},
		checkRuns: []*loomgh.CheckRun{{Name: "build", Status: "completed", Conclusion: "failure"}},
	}

	lifecycle, err := controller.ProcessDueWake(context.Background(), gh)
	require.NoError(t, err)
	assert.Equal(t, "DEBUGGING", lifecycle.WorkflowState)
	assert.Empty(t, lifecycle.NextWakeKind)
	assert.Zero(t, gh.requestReviewCalls)

	cp, readErr := st.ReadCheckpoint(context.Background())
	require.NoError(t, readErr)
	assert.Equal(t, "DEBUGGING", cp.State)
	require.Len(t, st.decisions, 1)
	assert.Equal(t, "escalate", st.decisions[0].Verdict)
}

func TestControllerProcessDueWake_PRReadyRetryBudgetExhaustionPauses(t *testing.T) {
	now := time.Date(2026, 3, 20, 14, 9, 0, 0, time.UTC)
	st := newMemStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{
		State:      "AWAITING_READY",
		Phase:      2,
		PRNumber:   42,
		RetryCount: 60,
	}))
	require.NoError(t, st.UpsertWakeSchedule(context.Background(), store.WakeSchedule{
		SessionID: "default",
		WakeKind:  "poll_pr_ready",
		DueAt:     now.Add(-time.Minute),
		DedupeKey: "run:default:poll_pr_ready",
	}))

	controller := loomruntime.NewController(st, loomruntime.Config{
		HolderID:     "controller-1",
		LeaseTTL:     time.Minute,
		PollInterval: 30 * time.Second,
		Now:          func() time.Time { return now },
	})

	gh := &pollingGitHubClientMock{pr: &loomgh.PR{Number: 42, Draft: true}}
	lifecycle, err := controller.ProcessDueWake(context.Background(), gh)
	require.NoError(t, err)
	assert.Equal(t, "PAUSED", lifecycle.WorkflowState)
	assert.Equal(t, loomruntime.ControllerStatePaused, lifecycle.Controller)
	assert.Equal(t, "AWAITING_READY", lifecycle.ResumeState)
	assert.Zero(t, gh.markReadyCalls)

	cp, readErr := st.ReadCheckpoint(context.Background())
	require.NoError(t, readErr)
	assert.Equal(t, "PAUSED", cp.State)
	assert.Equal(t, "AWAITING_READY", cp.ResumeState)
	require.Len(t, st.decisions, 1)
	assert.Equal(t, "pause", st.decisions[0].Verdict)
	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(st.decisions[0].Detail), &payload))
	assert.Equal(t, "draft_retry_exhausted", payload["outcome"])
}

func TestControllerProcessDueWake_CIRetryBudgetExhaustionPauses(t *testing.T) {
	now := time.Date(2026, 3, 20, 14, 9, 30, 0, time.UTC)
	st := newMemStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{
		State:      "AWAITING_CI",
		Phase:      2,
		PRNumber:   42,
		RetryCount: 20,
	}))
	require.NoError(t, st.UpsertWakeSchedule(context.Background(), store.WakeSchedule{
		SessionID: "default",
		WakeKind:  "poll_ci",
		DueAt:     now.Add(-time.Minute),
		DedupeKey: "run:default:poll_ci",
	}))

	controller := loomruntime.NewController(st, loomruntime.Config{
		HolderID:     "controller-1",
		LeaseTTL:     time.Minute,
		PollInterval: 30 * time.Second,
		Now:          func() time.Time { return now },
	})

	gh := &pollingGitHubClientMock{
		pr:        &loomgh.PR{Number: 42, HeadSHA: "abc123"},
		checkRuns: []*loomgh.CheckRun{{Name: "build", Status: "in_progress"}},
	}
	lifecycle, err := controller.ProcessDueWake(context.Background(), gh)
	require.NoError(t, err)
	assert.Equal(t, "PAUSED", lifecycle.WorkflowState)
	assert.Equal(t, loomruntime.ControllerStatePaused, lifecycle.Controller)
	assert.Equal(t, "AWAITING_CI", lifecycle.ResumeState)

	cp, readErr := st.ReadCheckpoint(context.Background())
	require.NoError(t, readErr)
	assert.Equal(t, "PAUSED", cp.State)
	assert.Equal(t, "AWAITING_CI", cp.ResumeState)
	require.Len(t, st.decisions, 1)
	assert.Equal(t, "pause", st.decisions[0].Verdict)
}
