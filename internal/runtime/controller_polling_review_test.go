package runtime_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	loomruntime "github.com/guillaume7/loom/internal/runtime"
	"github.com/guillaume7/loom/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestControllerProcessDueWake_ReviewApprovedTransitionsToMerging(t *testing.T) {
	now := time.Date(2026, 3, 20, 14, 11, 0, 0, time.UTC)
	ciRecordedAt := now.Add(-30 * time.Second)
	st := newMemStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "REVIEWING", Phase: 2, PRNumber: 42}))
	require.NoError(t, st.UpsertWakeSchedule(context.Background(), store.WakeSchedule{
		SessionID: "default",
		WakeKind:  "poll_review",
		DueAt:     now.Add(-time.Minute),
		DedupeKey: "run:default:poll_review",
	}))
	ciObsDetail := `{"session_id":"default","correlation_id":"poll:default:poll_ci:1","wake_kind":"poll_ci","pr_number":42,"ci":{"total_checks":2,"green_checks":2}}`
	require.NoError(t, st.WritePolicyDecision(context.Background(), store.PolicyDecision{
		SessionID:     "default",
		CorrelationID: "poll:default:poll_ci:1",
		DecisionKind:  "ci_readiness",
		Verdict:       "continue",
		InputHash:     "hash1",
		Detail:        ciObsDetail,
		CreatedAt:     ciRecordedAt,
	}))

	controller := loomruntime.NewController(st, loomruntime.Config{
		HolderID:     "controller-1",
		LeaseTTL:     time.Minute,
		PollInterval: time.Minute,
		Now:          func() time.Time { return now },
	})

	lifecycle, err := controller.ProcessDueWake(context.Background(), &pollingGitHubClientMock{reviewStatus: "APPROVED"})
	require.NoError(t, err)
	assert.Equal(t, "MERGING", lifecycle.WorkflowState)
	assert.Empty(t, lifecycle.NextWakeKind)

	cp, readErr := st.ReadCheckpoint(context.Background())
	require.NoError(t, readErr)
	assert.Equal(t, "MERGING", cp.State)
	require.Len(t, st.decisions, 2)
	assert.Equal(t, "continue", st.decisions[1].Verdict)
}

func TestControllerProcessDueWake_ReviewApprovedBlocksWhenCIEvidenceMissing(t *testing.T) {
	now := time.Date(2026, 3, 20, 14, 11, 30, 0, time.UTC)
	st := newMemStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "REVIEWING", Phase: 2, PRNumber: 42}))
	require.NoError(t, st.UpsertWakeSchedule(context.Background(), store.WakeSchedule{
		SessionID: "default",
		WakeKind:  "poll_review",
		DueAt:     now.Add(-time.Minute),
		DedupeKey: "run:default:poll_review",
	}))

	controller := loomruntime.NewController(st, loomruntime.Config{
		HolderID:     "controller-1",
		LeaseTTL:     time.Minute,
		PollInterval: time.Minute,
		Now:          func() time.Time { return now },
	})

	lifecycle, err := controller.ProcessDueWake(context.Background(), &pollingGitHubClientMock{reviewStatus: "APPROVED"})
	require.NoError(t, err)
	assert.Equal(t, "REVIEWING", lifecycle.WorkflowState)
	assert.Equal(t, "poll_review", lifecycle.NextWakeKind)

	cp, readErr := st.ReadCheckpoint(context.Background())
	require.NoError(t, readErr)
	assert.Equal(t, "REVIEWING", cp.State)
	require.Len(t, st.decisions, 1)
	assert.Equal(t, "block", st.decisions[0].Verdict)

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(st.decisions[0].Detail), &payload))
	assert.Equal(t, "merge_observations_incomplete", payload["merge_policy_reason"])
}

func TestControllerProcessDueWake_ReviewChangesRequestedTransitionsToAddressingFeedback(t *testing.T) {
	now := time.Date(2026, 3, 20, 14, 12, 0, 0, time.UTC)
	st := newMemStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "REVIEWING", Phase: 2, PRNumber: 42}))
	require.NoError(t, st.UpsertWakeSchedule(context.Background(), store.WakeSchedule{
		SessionID: "default",
		WakeKind:  "poll_review",
		DueAt:     now.Add(-time.Minute),
		DedupeKey: "run:default:poll_review",
	}))

	controller := loomruntime.NewController(st, loomruntime.Config{
		HolderID:     "controller-1",
		LeaseTTL:     time.Minute,
		PollInterval: time.Minute,
		Now:          func() time.Time { return now },
	})

	lifecycle, err := controller.ProcessDueWake(context.Background(), &pollingGitHubClientMock{reviewStatus: "CHANGES_REQUESTED"})
	require.NoError(t, err)
	assert.Equal(t, "ADDRESSING_FEEDBACK", lifecycle.WorkflowState)
	assert.Empty(t, lifecycle.NextWakeKind)

	cp, readErr := st.ReadCheckpoint(context.Background())
	require.NoError(t, readErr)
	assert.Equal(t, "ADDRESSING_FEEDBACK", cp.State)
	require.Len(t, st.decisions, 1)
	assert.Equal(t, "block", st.decisions[0].Verdict)
}

func TestControllerProcessDueWake_ReviewPendingRecordsObservationWithoutPromptReplay(t *testing.T) {
	now := time.Date(2026, 3, 20, 14, 10, 0, 0, time.UTC)
	st := newMemStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "REVIEWING", Phase: 2, PRNumber: 42}))
	require.NoError(t, st.UpsertWakeSchedule(context.Background(), store.WakeSchedule{
		SessionID: "default",
		WakeKind:  "poll_review",
		DueAt:     now.Add(-time.Minute),
		DedupeKey: "run:default:poll_review",
	}))

	controller := loomruntime.NewController(st, loomruntime.Config{
		HolderID:     "controller-1",
		LeaseTTL:     time.Minute,
		PollInterval: time.Minute,
		Now:          func() time.Time { return now },
	})

	gh := &pollingGitHubClientMock{reviewStatus: "PENDING"}
	lifecycle, err := controller.ProcessDueWake(context.Background(), gh)
	require.NoError(t, err)

	assert.Equal(t, "REVIEWING", lifecycle.WorkflowState)
	assert.Equal(t, loomruntime.ControllerStateClaimed, lifecycle.Controller)
	assert.Equal(t, "poll_review", lifecycle.NextWakeKind)
	assert.Equal(t, now.Add(time.Minute), lifecycle.NextWakeAt)
	assert.Equal(t, "poll_observation", lifecycle.DrivenBy)

	cp, err := st.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "REVIEWING", cp.State)

	require.Len(t, st.actions, 1)
	assert.Equal(t, "poll_waiting", st.actions[0].Event)
	require.Len(t, st.events, 1)
	assert.Equal(t, "poll_review", st.events[0].EventKind)
	require.Len(t, st.decisions, 1)
	assert.Equal(t, "wait", st.decisions[0].Verdict)
	assert.Equal(t, st.events[0].CorrelationID, st.decisions[0].CorrelationID)
}
