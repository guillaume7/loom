package runtime_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	loomgh "github.com/guillaume7/loom/internal/github"
	loomruntime "github.com/guillaume7/loom/internal/runtime"
	"github.com/guillaume7/loom/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestControllerProcessDueWake_SkipsAlreadyProcessedWakeWithoutRePolling(t *testing.T) {
	now := time.Date(2026, 3, 20, 14, 20, 0, 0, time.UTC)
	createdAt := now.Add(-2 * time.Minute)
	st := newMemStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "AWAITING_CI", Phase: 2, PRNumber: 42}))
	wake := store.WakeSchedule{
		SessionID: "default",
		WakeKind:  "poll_ci",
		DueAt:     now.Add(-time.Minute),
		DedupeKey: "run:default:poll_ci",
		ClaimedAt: now.Add(-30 * time.Second),
		CreatedAt: createdAt,
	}
	require.NoError(t, st.UpsertWakeSchedule(context.Background(), wake))
	operationKey := fmt.Sprintf("poll_resume:default:%s:created:%d", wake.DedupeKey, createdAt.UnixNano())
	require.NoError(t, st.WriteAction(context.Background(), store.Action{
		SessionID:    "default",
		OperationKey: operationKey,
		StateBefore:  "AWAITING_CI",
		StateAfter:   "REVIEWING",
		Event:        "ci_green",
		CreatedAt:    now.Add(-time.Second),
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
	lifecycle, err := controller.ProcessDueWake(context.Background(), gh)
	require.NoError(t, err)

	assert.Equal(t, loomruntime.ControllerStateClaimed, lifecycle.Controller)
	assert.Equal(t, "wake_already_processed", lifecycle.Reason)
	assert.Equal(t, "resume_deduplication", lifecycle.DrivenBy)
	assert.Zero(t, gh.getPRCalls)
	assert.Zero(t, gh.getCheckRunsCalls)
	require.Len(t, st.actions, 1)
	require.Len(t, st.events, 1)
	assert.Equal(t, "wake_duplicate_skipped", st.events[0].EventKind)
	require.Len(t, st.decisions, 1)
	assert.Equal(t, "runtime_resume_dedupe", st.decisions[0].DecisionKind)
	assert.Equal(t, "skip_duplicate", st.decisions[0].Verdict)

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(st.events[0].Payload), &payload))
	assert.Equal(t, "wake_already_processed", payload["reason"])
	assert.Equal(t, operationKey, payload["operation_key"])

	cp, err := st.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "AWAITING_CI", cp.State)
}

func TestControllerProcessDueWake_SkipsConcurrentDuplicateOperationKey(t *testing.T) {
	now := time.Date(2026, 3, 20, 14, 22, 0, 0, time.UTC)
	st := newDuplicatePollWriteStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "AWAITING_CI", Phase: 2, PRNumber: 42}))
	require.NoError(t, st.UpsertWakeSchedule(context.Background(), store.WakeSchedule{
		SessionID: "default",
		WakeKind:  "poll_ci",
		DueAt:     now.Add(-time.Minute),
		DedupeKey: "run:default:poll_ci",
		CreatedAt: now.Add(-2 * time.Minute),
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
		},
	}
	lifecycle, err := controller.ProcessDueWake(context.Background(), gh)
	require.NoError(t, err)

	assert.Equal(t, loomruntime.ControllerStateClaimed, lifecycle.Controller)
	assert.Equal(t, "duplicate_operation_key", lifecycle.Reason)
	assert.Equal(t, "resume_deduplication", lifecycle.DrivenBy)
	assert.Equal(t, 1, gh.getPRCalls)
	assert.Equal(t, 1, gh.getCheckRunsCalls)
	require.Len(t, st.actions, 1)
	require.Len(t, st.events, 1)
	assert.Equal(t, "wake_duplicate_skipped", st.events[0].EventKind)
	require.Len(t, st.decisions, 1)
	assert.Equal(t, "runtime_resume_dedupe", st.decisions[0].DecisionKind)
	assert.Equal(t, "skip_duplicate", st.decisions[0].Verdict)

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(st.decisions[0].Detail), &payload))
	assert.Equal(t, "duplicate_operation_key", payload["reason"])

	cp, err := st.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "REVIEWING", cp.State)
}

func TestControllerProcessDueWake_SkipsCrossPathDuplicateAfterGitHubWakeUpdate(t *testing.T) {
	now := time.Date(2026, 3, 20, 14, 24, 0, 0, time.UTC)
	claimedAt := now.Add(-30 * time.Second)
	createdAt := now.Add(-2 * time.Minute)
	st := newMemStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "AWAITING_CI", Phase: 2, PRNumber: 42}))
	require.NoError(t, st.UpsertWakeSchedule(context.Background(), store.WakeSchedule{
		SessionID: "default",
		WakeKind:  "poll_ci",
		DueAt:     now.Add(-time.Minute),
		DedupeKey: "run:default:poll_ci",
		Payload:   `{"attempt":1}`,
		ClaimedAt: claimedAt,
		CreatedAt: createdAt,
	}))

	initialWake := st.wakes[0]
	operationKey := fmt.Sprintf("poll_resume:default:%s:created:%d", initialWake.DedupeKey, createdAt.UnixNano())
	require.NoError(t, st.WriteAction(context.Background(), store.Action{
		SessionID:    "default",
		OperationKey: operationKey,
		StateBefore:  "AWAITING_CI",
		StateAfter:   "REVIEWING",
		Event:        "ci_green",
		CreatedAt:    now.Add(-time.Second),
	}))

	controller := loomruntime.NewController(st, loomruntime.Config{
		HolderID:     "controller-1",
		LeaseTTL:     time.Minute,
		PollInterval: 45 * time.Second,
		Now:          func() time.Time { return now },
	})

	receipt, err := controller.ObserveGitHubEvent(context.Background(), loomruntime.GitHubEvent{
		DeliveryID: "evt-cross-path",
		EventType:  "check_suite",
		PRNumber:   42,
		Payload:    json.RawMessage(`{"check_suite":{"conclusion":"success"}}`),
	})
	require.NoError(t, err)
	assert.True(t, receipt.WakeScheduled)
	assert.Equal(t, "poll_ci", receipt.WakeKind)

	wakes, err := st.ReadWakeSchedules(context.Background(), "default", 10)
	require.NoError(t, err)
	require.Len(t, wakes, 1)
	assert.Equal(t, initialWake.ID, wakes[0].ID)
	assert.Equal(t, now, wakes[0].DueAt)
	assert.Equal(t, claimedAt, wakes[0].ClaimedAt)
	assert.Equal(t, createdAt, wakes[0].CreatedAt)
	assert.Contains(t, wakes[0].Payload, "success")

	gh := &pollingGitHubClientMock{
		pr:        &loomgh.PR{Number: 42, HeadSHA: "abc123"},
		checkRuns: []*loomgh.CheckRun{{Name: "build", Status: "completed", Conclusion: "success"}},
	}
	lifecycle, err := controller.ProcessDueWake(context.Background(), gh)
	require.NoError(t, err)

	assert.Equal(t, loomruntime.ControllerStateClaimed, lifecycle.Controller)
	assert.Equal(t, "wake_already_processed", lifecycle.Reason)
	assert.Equal(t, "resume_deduplication", lifecycle.DrivenBy)
	assert.Zero(t, gh.getPRCalls)
	assert.Zero(t, gh.getCheckRunsCalls)
	require.Len(t, st.actions, 1)
	require.Len(t, st.events, 2)
	assert.Equal(t, "check_suite", st.events[0].EventKind)
	assert.Equal(t, "wake_duplicate_skipped", st.events[1].EventKind)
	require.Len(t, st.decisions, 1)
	assert.Equal(t, "runtime_resume_dedupe", st.decisions[0].DecisionKind)
	assert.Equal(t, "skip_duplicate", st.decisions[0].Verdict)

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(st.events[1].Payload), &payload))
	assert.Equal(t, "wake_already_processed", payload["reason"])
	assert.Equal(t, operationKey, payload["operation_key"])

	cp, err := st.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "AWAITING_CI", cp.State)
}

func TestControllerProcessDueWake_StaleObservationConflictBlocksEvaluation(t *testing.T) {
	now := time.Date(2026, 3, 20, 14, 30, 0, 0, time.UTC)
	staleAt := now.Add(-3 * loomruntime.DefaultObservationStaleAfter)
	st := newMemStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "AWAITING_CI", Phase: 2, PRNumber: 42}))
	require.NoError(t, st.UpsertWakeSchedule(context.Background(), store.WakeSchedule{
		SessionID: "default",
		WakeKind:  "poll_ci",
		DueAt:     now.Add(-time.Minute),
		DedupeKey: "run:default:poll_ci",
	}))
	require.NoError(t, st.WriteExternalEvent(context.Background(), store.ExternalEvent{
		SessionID:     "default",
		EventSource:   "poll",
		EventKind:     "poll_ci",
		CorrelationID: "poll:default:stale",
		Payload:       `{"session_id":"default","correlation_id":"poll:default:stale","wake_kind":"poll_ci","pr_number":42,"decision_verdict":"wait","ci":{"total_checks":1,"green_checks":0,"pending_checks":["build"]}}`,
		ObservedAt:    staleAt,
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
	lifecycle, err := controller.ProcessDueWake(context.Background(), gh)
	require.NoError(t, err)

	assert.Equal(t, loomruntime.ControllerStateClaimed, lifecycle.Controller)
	assert.Equal(t, "resume_conflict", lifecycle.DrivenBy)
	assert.Equal(t, "observation_model_stale", lifecycle.Reason)
	assert.Equal(t, "AWAITING_CI", lifecycle.WorkflowState)
	assert.Zero(t, gh.getPRCalls)
	assert.Zero(t, gh.getCheckRunsCalls)
	require.Empty(t, st.actions)
	require.Len(t, st.decisions, 1)
	assert.Equal(t, "resume_conflict", st.decisions[0].DecisionKind)
	assert.Equal(t, "block", st.decisions[0].Verdict)
}

func TestControllerProcessDueWake_PRLockContentionBlocksEvaluation(t *testing.T) {
	now := time.Date(2026, 3, 20, 14, 31, 0, 0, time.UTC)
	st := newMemStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "AWAITING_CI", Phase: 2, PRNumber: 42}))
	require.NoError(t, st.UpsertWakeSchedule(context.Background(), store.WakeSchedule{
		SessionID: "default",
		WakeKind:  "poll_ci",
		DueAt:     now.Add(-time.Minute),
		DedupeKey: "run:default:poll_ci",
	}))
	require.NoError(t, st.UpsertRuntimeLease(context.Background(), store.RuntimeLease{
		LeaseKey:  "pr:42",
		HolderID:  "controller-2",
		Scope:     "pr",
		ExpiresAt: now.Add(time.Minute),
		CreatedAt: now.Add(-time.Minute),
		RenewedAt: now,
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
	lifecycle, err := controller.ProcessDueWake(context.Background(), gh)
	require.NoError(t, err)

	assert.Equal(t, loomruntime.ControllerStateClaimed, lifecycle.Controller)
	assert.Equal(t, "resume_conflict", lifecycle.DrivenBy)
	assert.Equal(t, "pr_mutation_lock_held_by_other", lifecycle.Reason)
	assert.Equal(t, "AWAITING_CI", lifecycle.WorkflowState)
	assert.Zero(t, gh.getPRCalls)
	assert.Zero(t, gh.getCheckRunsCalls)
	require.Empty(t, st.actions)
	require.Len(t, st.decisions, 1)
	assert.Equal(t, "resume_conflict", st.decisions[0].DecisionKind)
	assert.Equal(t, "wait", st.decisions[0].Verdict)
}
