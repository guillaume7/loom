package runtime_test

import (
	"context"
	"errors"
	"testing"
	"time"

	loomruntime "github.com/guillaume7/loom/internal/runtime"
	"github.com/guillaume7/loom/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestControllerStartClaimsDuePersistedWork(t *testing.T) {
	now := time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)
	st := newMemStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "AWAITING_CI", Phase: 2}))
	require.NoError(t, st.UpsertWakeSchedule(context.Background(), store.WakeSchedule{
		SessionID: "default",
		WakeKind:  "poll_ci",
		DueAt:     now.Add(-time.Minute),
		DedupeKey: "run:default:poll_ci",
	}))

	controller := loomruntime.NewController(st, loomruntime.Config{
		HolderID:     "controller-1",
		LeaseTTL:     time.Minute,
		PollInterval: time.Minute,
		Now:          func() time.Time { return now },
	})

	lifecycle, err := controller.Start(context.Background())
	require.NoError(t, err)

	assert.Equal(t, loomruntime.ControllerStateWakeDue, lifecycle.Controller)
	assert.Equal(t, "persisted_runtime_state", lifecycle.DrivenBy)
	assert.Equal(t, "controller-1", lifecycle.HolderID)
	assert.Equal(t, "run:default", lifecycle.LeaseKey)
	assert.Equal(t, "poll_ci", lifecycle.NextWakeKind)

	lease, err := st.ReadRuntimeLease(context.Background(), "run:default")
	require.NoError(t, err)
	assert.Equal(t, "controller-1", lease.HolderID)

	wakes, err := st.ReadWakeSchedules(context.Background(), "default", 10)
	require.NoError(t, err)
	require.Len(t, wakes, 1)
	assert.Equal(t, now, wakes[0].ClaimedAt)
}

func TestControllerStartReclaimsDueWakeAfterExpiredLease(t *testing.T) {
	initialClaimAt := time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)
	restartAt := initialClaimAt.Add(2 * time.Minute)
	st := newMemStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "AWAITING_CI", Phase: 2}))
	require.NoError(t, st.UpsertWakeSchedule(context.Background(), store.WakeSchedule{
		SessionID: "default",
		WakeKind:  "poll_ci",
		DueAt:     initialClaimAt.Add(-time.Minute),
		DedupeKey: "run:default:poll_ci",
		ClaimedAt: initialClaimAt,
	}))
	require.NoError(t, st.UpsertRuntimeLease(context.Background(), store.RuntimeLease{
		LeaseKey:  "run:default",
		HolderID:  "controller-1",
		Scope:     "run",
		ExpiresAt: initialClaimAt.Add(time.Minute),
		CreatedAt: initialClaimAt,
		RenewedAt: initialClaimAt,
	}))

	controller := loomruntime.NewController(st, loomruntime.Config{
		HolderID:     "controller-2",
		LeaseTTL:     time.Minute,
		PollInterval: time.Minute,
		Now:          func() time.Time { return restartAt },
	})

	lifecycle, err := controller.Start(context.Background())
	require.NoError(t, err)

	assert.Equal(t, loomruntime.ControllerStateWakeDue, lifecycle.Controller)
	assert.Equal(t, "persisted_wake_due", lifecycle.Reason)
	assert.Equal(t, "controller-2", lifecycle.HolderID)

	lease, err := st.ReadRuntimeLease(context.Background(), "run:default")
	require.NoError(t, err)
	assert.Equal(t, "controller-2", lease.HolderID)
	assert.Equal(t, restartAt.Add(time.Minute), lease.ExpiresAt)

	decisions, err := st.ReadPolicyDecisions(context.Background(), "default", 10)
	require.NoError(t, err)
	require.Len(t, decisions, 1)
	assert.Equal(t, "lease_recovery", decisions[0].DecisionKind)
	assert.Equal(t, "recovered", decisions[0].Verdict)

	wakes, err := st.ReadWakeSchedules(context.Background(), "default", 10)
	require.NoError(t, err)
	require.Len(t, wakes, 1)
	assert.Equal(t, restartAt, wakes[0].ClaimedAt)
}

func TestControllerStartSleepsWhenActiveLeaseHeldByAnotherController(t *testing.T) {
	now := time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)
	st := newMemStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "AWAITING_CI", Phase: 2}))
	require.NoError(t, st.UpsertRuntimeLease(context.Background(), store.RuntimeLease{
		LeaseKey:  "run:default",
		HolderID:  "controller-1",
		Scope:     "run",
		ExpiresAt: now.Add(time.Minute),
		CreatedAt: now,
		RenewedAt: now,
	}))

	controller := loomruntime.NewController(st, loomruntime.Config{
		HolderID:     "controller-2",
		LeaseTTL:     time.Minute,
		PollInterval: time.Minute,
		Now:          func() time.Time { return now },
	})

	lifecycle, err := controller.Start(context.Background())
	require.NoError(t, err)

	assert.Equal(t, loomruntime.ControllerStateSleeping, lifecycle.Controller)
	assert.Equal(t, "lease_held_by_other_controller", lifecycle.Reason)
	assert.Equal(t, "controller-1", lifecycle.HolderID)
	assert.Equal(t, "run:default", lifecycle.LeaseKey)

	lease, err := st.ReadRuntimeLease(context.Background(), "run:default")
	require.NoError(t, err)
	assert.Equal(t, "controller-1", lease.HolderID)
}

func TestControllerStartSleepsUntilNextPersistedWake(t *testing.T) {
	now := time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)
	st := newMemStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "AWAITING_CI", Phase: 2}))

	controller := loomruntime.NewController(st, loomruntime.Config{
		HolderID:     "controller-1",
		LeaseTTL:     time.Minute,
		PollInterval: 45 * time.Second,
		Now:          func() time.Time { return now },
	})

	lifecycle, err := controller.Start(context.Background())
	require.NoError(t, err)

	assert.Equal(t, loomruntime.ControllerStateSleeping, lifecycle.Controller)
	assert.Equal(t, "poll_ci", lifecycle.NextWakeKind)
	assert.Equal(t, now.Add(45*time.Second), lifecycle.NextWakeAt)
	assert.Equal(t, "persisted_runtime_state", lifecycle.DrivenBy)

	wakes, err := st.ReadWakeSchedules(context.Background(), "default", 10)
	require.NoError(t, err)
	require.Len(t, wakes, 1)
	assert.Equal(t, "poll_ci", wakes[0].WakeKind)
	assert.Equal(t, now.Add(45*time.Second), wakes[0].DueAt)
}

func TestControllerPendingWakesReturnsPersistedQueueForCurrentRun(t *testing.T) {
	now := time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)
	st := newMemStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "AWAITING_CI", Phase: 2, StoryID: "TH3.E2.US1"}))
	require.NoError(t, st.UpsertWakeSchedule(context.Background(), store.WakeSchedule{
		SessionID: "TH3.E2.US1",
		WakeKind:  "poll_ci",
		DueAt:     now.Add(time.Minute),
		DedupeKey: "run:TH3.E2.US1:poll_ci",
	}))
	require.NoError(t, st.UpsertWakeSchedule(context.Background(), store.WakeSchedule{
		SessionID: "TH3.E2.US1",
		WakeKind:  "poll_review",
		DueAt:     now.Add(2 * time.Minute),
		DedupeKey: "run:TH3.E2.US1:poll_review",
	}))
	require.NoError(t, st.UpsertWakeSchedule(context.Background(), store.WakeSchedule{
		SessionID: "other-story",
		WakeKind:  "poll_ci",
		DueAt:     now.Add(3 * time.Minute),
		DedupeKey: "run:other-story:poll_ci",
	}))

	controller := loomruntime.NewController(st, loomruntime.Config{Now: func() time.Time { return now }})
	wakes, err := controller.PendingWakes(context.Background())
	require.NoError(t, err)
	require.Len(t, wakes, 2)
	assert.Equal(t, "poll_ci", wakes[0].WakeKind)
	assert.Equal(t, "run:TH3.E2.US1:poll_ci", wakes[0].DedupeKey)
	assert.Equal(t, "poll_review", wakes[1].WakeKind)
	assert.Equal(t, "run:TH3.E2.US1:poll_review", wakes[1].DedupeKey)
}

func TestControllerSnapshotExposesActiveControllerProgress(t *testing.T) {
	now := time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)
	st := newMemStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "SCANNING", Phase: 1}))
	require.NoError(t, st.UpsertRuntimeLease(context.Background(), store.RuntimeLease{
		LeaseKey:  "run:default",
		HolderID:  "controller-1",
		Scope:     "run",
		ExpiresAt: now.Add(time.Minute),
		CreatedAt: now.Add(-time.Minute),
		RenewedAt: now,
	}))

	controller := loomruntime.NewController(st, loomruntime.Config{
		HolderID: "controller-1",
		Now:      func() time.Time { return now },
	})

	lifecycle, err := controller.Snapshot(context.Background())
	require.NoError(t, err)

	assert.Equal(t, loomruntime.ControllerStateClaimed, lifecycle.Controller)
	assert.Equal(t, "active_lease_present", lifecycle.Reason)
	assert.Equal(t, "controller-1", lifecycle.HolderID)
	assert.Equal(t, "run:default", lifecycle.LeaseKey)
	assert.Equal(t, now.Add(time.Minute), lifecycle.LeaseExpires)
	assert.Equal(t, "SCANNING", lifecycle.WorkflowState)
}

func TestControllerPauseOverridePreservesRecoverableStateAndAuditsIntent(t *testing.T) {
	now := time.Date(2026, 3, 20, 13, 0, 0, 0, time.UTC)
	st := newMemStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "AWAITING_CI", Phase: 2, PRNumber: 42}))
	require.NoError(t, st.UpsertWakeSchedule(context.Background(), store.WakeSchedule{
		SessionID: "default",
		WakeKind:  "poll_ci",
		DueAt:     now.Add(time.Minute),
		DedupeKey: "run:default:poll_ci",
	}))

	controller := loomruntime.NewController(st, loomruntime.Config{Now: func() time.Time { return now }})
	lifecycle, err := controller.ApplyManualOverride(context.Background(), loomruntime.ManualOverrideRequest{
		Action:      loomruntime.ManualOverridePause,
		Source:      "cli",
		RequestedBy: "test",
		Reason:      "operator requested pause",
	})
	require.NoError(t, err)

	assert.Equal(t, loomruntime.ControllerStatePaused, lifecycle.Controller)
	assert.Equal(t, "AWAITING_CI", lifecycle.ResumeState)
	assert.Len(t, st.wakes, 1)
	require.Len(t, st.actions, 1)
	assert.Equal(t, "manual_override_pause", st.actions[0].Event)
	require.Len(t, st.events, 1)
	assert.Equal(t, "operator", st.events[0].EventSource)
	assert.Equal(t, "manual_override.pause", st.events[0].EventKind)
	require.Len(t, st.decisions, 1)
	assert.Equal(t, "operator_override", st.decisions[0].DecisionKind)
	assert.Equal(t, "pause", st.decisions[0].Verdict)
	assert.Equal(t, st.events[0].CorrelationID, st.decisions[0].CorrelationID)

	cp, err := st.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "PAUSED", cp.State)
	assert.Equal(t, "AWAITING_CI", cp.ResumeState)
}

func TestControllerPauseOverrideRejectsMissingRecoverableState(t *testing.T) {
	now := time.Date(2026, 3, 20, 13, 5, 0, 0, time.UTC)
	st := newMemStore()
	controller := loomruntime.NewController(st, loomruntime.Config{Now: func() time.Time { return now }})

	_, err := controller.ApplyManualOverride(context.Background(), loomruntime.ManualOverrideRequest{
		Action:      loomruntime.ManualOverridePause,
		Source:      "cli",
		RequestedBy: "test",
		Reason:      "operator requested pause",
	})
	require.ErrorIs(t, err, loomruntime.ErrNothingToPause)
	assert.Empty(t, st.actions)
	assert.Empty(t, st.events)
	assert.Empty(t, st.decisions)
	assert.Empty(t, st.wakes)

	cp, readErr := st.ReadCheckpoint(context.Background())
	require.NoError(t, readErr)
	assert.Equal(t, store.Checkpoint{}, cp)
}

func TestControllerResumeOverrideReusesPauseCorrelation(t *testing.T) {
	now := time.Date(2026, 3, 20, 13, 10, 0, 0, time.UTC)
	st := newMemStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "PAUSED", ResumeState: "AWAITING_CI", Phase: 2, PRNumber: 42}))
	require.NoError(t, st.WritePolicyDecision(context.Background(), store.PolicyDecision{
		SessionID:     "default",
		CorrelationID: "corr-1",
		DecisionKind:  "operator_override",
		Verdict:       "pause",
		InputHash:     "pause-corr-1",
		CreatedAt:     now.Add(-time.Minute),
	}))

	controller := loomruntime.NewController(st, loomruntime.Config{Now: func() time.Time { return now }})
	lifecycle, err := controller.ApplyManualOverride(context.Background(), loomruntime.ManualOverrideRequest{
		Action:      loomruntime.ManualOverrideResume,
		Source:      "cli",
		RequestedBy: "test",
		Reason:      "operator requested resume",
	})
	require.NoError(t, err)

	assert.Equal(t, "AWAITING_CI", lifecycle.WorkflowState)
	require.Len(t, st.events, 1)
	assert.Equal(t, "corr-1", st.events[0].CorrelationID)
	require.Len(t, st.decisions, 2)
	assert.Equal(t, "resume", st.decisions[1].Verdict)
	assert.Equal(t, "corr-1", st.decisions[1].CorrelationID)

	cp, err := st.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "AWAITING_CI", cp.State)
	assert.Equal(t, "", cp.ResumeState)
}

func TestControllerResumeOverrideLeavesPausedCheckpointRecoverableWhenControllerStartFails(t *testing.T) {
	now := time.Date(2026, 3, 20, 13, 20, 0, 0, time.UTC)
	st := newMemStore()
	st.wakeErr = errors.New("wake store unavailable")
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "PAUSED", ResumeState: "AWAITING_CI", Phase: 2, PRNumber: 42}))

	controller := loomruntime.NewController(st, loomruntime.Config{
		HolderID:     "controller-1",
		PollInterval: time.Minute,
		Now:          func() time.Time { return now },
	})

	_, err := controller.ApplyManualOverride(context.Background(), loomruntime.ManualOverrideRequest{
		Action:        loomruntime.ManualOverrideResume,
		Source:        "cli",
		RequestedBy:   "test",
		Reason:        "operator requested resume",
		CorrelationID: "corr-1",
	})
	require.ErrorContains(t, err, "wake store unavailable")
	assert.Empty(t, st.actions)
	assert.Empty(t, st.events)
	assert.Empty(t, st.decisions)

	cp, readErr := st.ReadCheckpoint(context.Background())
	require.NoError(t, readErr)
	assert.Equal(t, "PAUSED", cp.State)
	assert.Equal(t, "AWAITING_CI", cp.ResumeState)

	lease, leaseErr := st.ReadRuntimeLease(context.Background(), "run:default")
	require.NoError(t, leaseErr)
	assert.Equal(t, "controller-1", lease.HolderID)
	assert.Equal(t, now, lease.ExpiresAt)
	assert.Equal(t, now, lease.RenewedAt)
}

func TestControllerStartDoesNotScheduleUnsupportedPollWakeStates(t *testing.T) {
	now := time.Date(2026, 3, 20, 14, 13, 0, 0, time.UTC)
	st := newMemStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "DEBUGGING", Phase: 2, PRNumber: 42}))

	controller := loomruntime.NewController(st, loomruntime.Config{
		HolderID:     "controller-1",
		LeaseTTL:     time.Minute,
		PollInterval: 30 * time.Second,
		Now:          func() time.Time { return now },
	})

	lifecycle, err := controller.Start(context.Background())
	require.NoError(t, err)
	assert.Equal(t, loomruntime.ControllerStateResuming, lifecycle.Controller)
	assert.Empty(t, lifecycle.NextWakeKind)

	wakes, readErr := st.ReadWakeSchedules(context.Background(), "default", 10)
	require.NoError(t, readErr)
	assert.Empty(t, wakes)
}
