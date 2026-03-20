package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/guillaume7/loom/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSQLiteStore_RuntimeRecordsExtendCheckpointWithoutReplacingIt(t *testing.T) {
	st := newMemDB(t)
	ctx := context.Background()

	checkpoint := store.Checkpoint{
		State:     "AWAITING_CI",
		Phase:     3,
		PRNumber:  42,
		UpdatedAt: time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
	}
	require.NoError(t, st.WriteCheckpoint(ctx, checkpoint))

	wakeDue := time.Date(2026, 3, 20, 10, 5, 0, 0, time.UTC)
	require.NoError(t, st.UpsertWakeSchedule(ctx, store.WakeSchedule{
		SessionID: "session-1",
		WakeKind:  "poll_ci",
		DueAt:     wakeDue,
		DedupeKey: "session-1:ci:42",
		Payload:   `{"pr":42}`,
		CreatedAt: time.Date(2026, 3, 20, 10, 1, 0, 0, time.UTC),
	}))
	require.NoError(t, st.UpsertRuntimeLease(ctx, store.RuntimeLease{
		LeaseKey:  "run:session-1",
		HolderID:  "runtime-1",
		Scope:     "run",
		ExpiresAt: time.Date(2026, 3, 20, 10, 10, 0, 0, time.UTC),
		CreatedAt: time.Date(2026, 3, 20, 10, 1, 30, 0, time.UTC),
		RenewedAt: time.Date(2026, 3, 20, 10, 2, 0, 0, time.UTC),
	}))

	gotCheckpoint, err := st.ReadCheckpoint(ctx)
	require.NoError(t, err)
	assert.Equal(t, checkpoint, gotCheckpoint)

	wakes, err := st.ReadWakeSchedules(ctx, "session-1", 10)
	require.NoError(t, err)
	require.Len(t, wakes, 1)
	assert.Equal(t, "poll_ci", wakes[0].WakeKind)
	assert.Equal(t, wakeDue, wakes[0].DueAt)

	lease, err := st.ReadRuntimeLease(ctx, "run:session-1")
	require.NoError(t, err)
	assert.Equal(t, "runtime-1", lease.HolderID)
	assert.Equal(t, "run", lease.Scope)
	assert.Equal(t, time.Date(2026, 3, 20, 10, 10, 0, 0, time.UTC), lease.ExpiresAt)
}

func TestSQLiteStore_ExternalEventsAndPolicyDecisionsPersistForReplay(t *testing.T) {
	st := newMemDB(t)
	ctx := context.Background()

	require.NoError(t, st.WriteExternalEvent(ctx, store.ExternalEvent{
		SessionID:     "session-1",
		EventSource:   "github",
		EventKind:     "check_suite.completed",
		ExternalID:    "evt-123",
		CorrelationID: "trace-1",
		Payload:       `{"conclusion":"success"}`,
		ObservedAt:    time.Date(2026, 3, 20, 11, 0, 0, 0, time.UTC),
	}))
	require.NoError(t, st.WriteExternalEvent(ctx, store.ExternalEvent{
		SessionID:     "session-1",
		EventSource:   "timer",
		EventKind:     "wake_due",
		CorrelationID: "trace-2",
		Payload:       `{"wake_kind":"poll_ci"}`,
		ObservedAt:    time.Date(2026, 3, 20, 11, 1, 0, 0, time.UTC),
	}))
	require.NoError(t, st.WritePolicyDecision(ctx, store.PolicyDecision{
		SessionID:     "session-1",
		CorrelationID: "trace-2",
		DecisionKind:  "ci_gate",
		Verdict:       "continue",
		InputHash:     "abc123",
		Detail:        `{"reason":"checks green"}`,
		CreatedAt:     time.Date(2026, 3, 20, 11, 2, 0, 0, time.UTC),
	}))

	events, err := st.ReadExternalEvents(ctx, "session-1", 10)
	require.NoError(t, err)
	require.Len(t, events, 2)
	assert.Equal(t, "wake_due", events[0].EventKind)
	assert.Equal(t, "trace-2", events[0].CorrelationID)
	assert.Equal(t, "check_suite.completed", events[1].EventKind)
	assert.Equal(t, "trace-1", events[1].CorrelationID)

	decisions, err := st.ReadPolicyDecisions(ctx, "session-1", 10)
	require.NoError(t, err)
	require.Len(t, decisions, 1)
	assert.Equal(t, "ci_gate", decisions[0].DecisionKind)
	assert.Equal(t, "continue", decisions[0].Verdict)
	assert.Equal(t, "trace-2", decisions[0].CorrelationID)
}

func TestSQLiteStore_DeleteAllClearsRuntimeRecords(t *testing.T) {
	st := newMemDB(t)
	ctx := context.Background()

	require.NoError(t, st.WriteCheckpoint(ctx, store.Checkpoint{State: "AWAITING_CI", Phase: 3}))
	require.NoError(t, st.UpsertWakeSchedule(ctx, store.WakeSchedule{
		SessionID: "session-1",
		WakeKind:  "poll_ci",
		DueAt:     time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC),
		DedupeKey: "session-1:ci:42",
	}))
	require.NoError(t, st.WriteExternalEvent(ctx, store.ExternalEvent{
		SessionID:   "session-1",
		EventSource: "github",
		EventKind:   "check_suite.completed",
		Payload:     `{}`,
	}))
	require.NoError(t, st.UpsertRuntimeLease(ctx, store.RuntimeLease{
		LeaseKey:  "run:session-1",
		HolderID:  "runtime-1",
		Scope:     "run",
		ExpiresAt: time.Date(2026, 3, 20, 12, 5, 0, 0, time.UTC),
	}))
	require.NoError(t, st.WritePolicyDecision(ctx, store.PolicyDecision{
		SessionID:    "session-1",
		DecisionKind: "ci_gate",
		Verdict:      "wait",
		InputHash:    "hash-1",
	}))

	require.NoError(t, st.DeleteAll(ctx))

	checkpoint, err := st.ReadCheckpoint(ctx)
	require.NoError(t, err)
	assert.Equal(t, store.Checkpoint{}, checkpoint)

	wakes, err := st.ReadWakeSchedules(ctx, "session-1", 10)
	require.NoError(t, err)
	assert.Empty(t, wakes)

	events, err := st.ReadExternalEvents(ctx, "session-1", 10)
	require.NoError(t, err)
	assert.Empty(t, events)

	_, err = st.ReadRuntimeLease(ctx, "run:session-1")
	require.ErrorIs(t, err, store.ErrRuntimeLeaseNotFound)

	decisions, err := st.ReadPolicyDecisions(ctx, "session-1", 10)
	require.NoError(t, err)
	assert.Empty(t, decisions)
}