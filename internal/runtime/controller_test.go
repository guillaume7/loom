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

type memStore struct {
	cp            store.Checkpoint
	wakes         []store.WakeSchedule
	runtimeLeases map[string]store.RuntimeLease
	empty         bool
}

func newMemStore() *memStore {
	return &memStore{empty: true, runtimeLeases: make(map[string]store.RuntimeLease)}
}

func (s *memStore) ReadCheckpoint(_ context.Context) (store.Checkpoint, error) {
	if s.empty {
		return store.Checkpoint{}, nil
	}
	return s.cp, nil
}

func (s *memStore) WriteCheckpoint(_ context.Context, cp store.Checkpoint) error {
	s.cp = cp
	s.empty = false
	return nil
}

func (s *memStore) WriteAction(_ context.Context, _ store.Action) error { return nil }

func (s *memStore) WriteCheckpointAndAction(_ context.Context, cp store.Checkpoint, _ store.Action) error {
	s.cp = cp
	s.empty = false
	return nil
}

func (s *memStore) ReadActionByOperationKey(_ context.Context, _ string) (store.Action, error) {
	return store.Action{}, store.ErrActionNotFound
}

func (s *memStore) ReadActions(_ context.Context, _ int) ([]store.Action, error) {
	return []store.Action{}, nil
}

func (s *memStore) UpsertWakeSchedule(_ context.Context, wake store.WakeSchedule) error {
	for index, existing := range s.wakes {
		if existing.DedupeKey == wake.DedupeKey {
			s.wakes[index] = wake
			return nil
		}
	}
	wake.ID = int64(len(s.wakes) + 1)
	s.wakes = append(s.wakes, wake)
	return nil
}

func (s *memStore) ReadWakeSchedules(_ context.Context, sessionID string, _ int) ([]store.WakeSchedule, error) {
	result := make([]store.WakeSchedule, 0, len(s.wakes))
	for _, wake := range s.wakes {
		if sessionID != "" && wake.SessionID != sessionID {
			continue
		}
		result = append(result, wake)
	}
	return result, nil
}

func (s *memStore) WriteExternalEvent(_ context.Context, _ store.ExternalEvent) error { return nil }

func (s *memStore) ReadExternalEvents(_ context.Context, _ string, _ int) ([]store.ExternalEvent, error) {
	return []store.ExternalEvent{}, nil
}

func (s *memStore) UpsertRuntimeLease(_ context.Context, lease store.RuntimeLease) error {
	s.runtimeLeases[lease.LeaseKey] = lease
	return nil
}

func (s *memStore) ReadRuntimeLease(_ context.Context, leaseKey string) (store.RuntimeLease, error) {
	lease, ok := s.runtimeLeases[leaseKey]
	if !ok {
		return store.RuntimeLease{}, store.ErrRuntimeLeaseNotFound
	}
	return lease, nil
}

func (s *memStore) WritePolicyDecision(_ context.Context, _ store.PolicyDecision) error { return nil }

func (s *memStore) ReadPolicyDecisions(_ context.Context, _ string, _ int) ([]store.PolicyDecision, error) {
	return []store.PolicyDecision{}, nil
}

func (s *memStore) DeleteAll(_ context.Context) error {
	s.cp = store.Checkpoint{}
	s.wakes = nil
	s.runtimeLeases = make(map[string]store.RuntimeLease)
	s.empty = true
	return nil
}

func (s *memStore) Close() error { return nil }

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

	wakes, err := st.ReadWakeSchedules(context.Background(), "default", 10)
	require.NoError(t, err)
	require.Len(t, wakes, 1)
	assert.Equal(t, restartAt, wakes[0].ClaimedAt)
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