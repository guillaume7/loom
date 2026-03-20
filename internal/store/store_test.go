package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/guillaume7/loom/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// memStore is an in-memory Store used only in tests.
// It satisfies the store.Store interface without any real I/O.
type memStore struct {
	cp              store.Checkpoint
	actions         []store.Action
	wakes           []store.WakeSchedule
	externalEvents  []store.ExternalEvent
	runtimeLeases   map[string]store.RuntimeLease
	policyDecisions []store.PolicyDecision
	empty           bool
}

func newMemStore() *memStore {
	return &memStore{
		empty:         true,
		runtimeLeases: make(map[string]store.RuntimeLease),
	}
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

func (s *memStore) WriteAction(_ context.Context, action store.Action) error {
	for _, existing := range s.actions {
		if existing.OperationKey == action.OperationKey {
			return store.ErrDuplicateOperationKey
		}
	}
	if action.CreatedAt.IsZero() {
		action.CreatedAt = time.Now().UTC()
	}
	action.ID = int64(len(s.actions) + 1)
	s.actions = append(s.actions, action)
	return nil
}

func (s *memStore) WriteCheckpointAndAction(_ context.Context, cp store.Checkpoint, action store.Action) error {
	for _, existing := range s.actions {
		if existing.OperationKey == action.OperationKey {
			return store.ErrDuplicateOperationKey
		}
	}
	s.cp = cp
	s.empty = false
	if action.CreatedAt.IsZero() {
		action.CreatedAt = time.Now().UTC()
	}
	action.ID = int64(len(s.actions) + 1)
	s.actions = append(s.actions, action)
	return nil
}

func (s *memStore) ReadActionByOperationKey(_ context.Context, operationKey string) (store.Action, error) {
	for _, action := range s.actions {
		if action.OperationKey == operationKey {
			return action, nil
		}
	}
	return store.Action{}, store.ErrActionNotFound
}

func (s *memStore) ReadActions(_ context.Context, limit int) ([]store.Action, error) {
	if limit <= 0 {
		return []store.Action{}, nil
	}
	if len(s.actions) == 0 {
		return []store.Action{}, nil
	}
	if limit > len(s.actions) {
		limit = len(s.actions)
	}
	actions := make([]store.Action, 0, limit)
	for index := len(s.actions) - 1; index >= len(s.actions)-limit; index-- {
		actions = append(actions, s.actions[index])
	}
	return actions, nil
}

func (s *memStore) UpsertWakeSchedule(_ context.Context, wake store.WakeSchedule) error {
	for index, existing := range s.wakes {
		if existing.DedupeKey == wake.DedupeKey {
			s.wakes[index] = wake
			return nil
		}
	}
	if wake.CreatedAt.IsZero() {
		wake.CreatedAt = time.Now().UTC()
	}
	wake.ID = int64(len(s.wakes) + 1)
	s.wakes = append(s.wakes, wake)
	return nil
}

func (s *memStore) ReadWakeSchedules(_ context.Context, sessionID string, limit int) ([]store.WakeSchedule, error) {
	if limit <= 0 {
		return []store.WakeSchedule{}, nil
	}
	wakes := make([]store.WakeSchedule, 0, len(s.wakes))
	for _, wake := range s.wakes {
		if sessionID != "" && wake.SessionID != sessionID {
			continue
		}
		wakes = append(wakes, wake)
	}
	if limit > len(wakes) {
		limit = len(wakes)
	}
	return wakes[:limit], nil
}

func (s *memStore) WriteExternalEvent(_ context.Context, event store.ExternalEvent) error {
	if event.ObservedAt.IsZero() {
		event.ObservedAt = time.Now().UTC()
	}
	event.ID = int64(len(s.externalEvents) + 1)
	s.externalEvents = append(s.externalEvents, event)
	return nil
}

func (s *memStore) ReadExternalEvents(_ context.Context, sessionID string, limit int) ([]store.ExternalEvent, error) {
	if limit <= 0 {
		return []store.ExternalEvent{}, nil
	}
	events := make([]store.ExternalEvent, 0, len(s.externalEvents))
	for index := len(s.externalEvents) - 1; index >= 0; index-- {
		event := s.externalEvents[index]
		if sessionID != "" && event.SessionID != sessionID {
			continue
		}
		events = append(events, event)
		if len(events) == limit {
			break
		}
	}
	return events, nil
}

func (s *memStore) UpsertRuntimeLease(_ context.Context, lease store.RuntimeLease) error {
	if s.runtimeLeases == nil {
		s.runtimeLeases = make(map[string]store.RuntimeLease)
	}
	if lease.CreatedAt.IsZero() {
		lease.CreatedAt = time.Now().UTC()
	}
	if lease.RenewedAt.IsZero() {
		lease.RenewedAt = lease.CreatedAt
	}
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

func (s *memStore) WritePolicyDecision(_ context.Context, decision store.PolicyDecision) error {
	if decision.CreatedAt.IsZero() {
		decision.CreatedAt = time.Now().UTC()
	}
	decision.ID = int64(len(s.policyDecisions) + 1)
	s.policyDecisions = append(s.policyDecisions, decision)
	return nil
}

func (s *memStore) ReadPolicyDecisions(_ context.Context, sessionID string, limit int) ([]store.PolicyDecision, error) {
	if limit <= 0 {
		return []store.PolicyDecision{}, nil
	}
	decisions := make([]store.PolicyDecision, 0, len(s.policyDecisions))
	for index := len(s.policyDecisions) - 1; index >= 0; index-- {
		decision := s.policyDecisions[index]
		if sessionID != "" && decision.SessionID != sessionID {
			continue
		}
		decisions = append(decisions, decision)
		if len(decisions) == limit {
			break
		}
	}
	return decisions, nil
}

func (s *memStore) DeleteAll(_ context.Context) error {
	s.cp = store.Checkpoint{}
	s.actions = nil
	s.wakes = nil
	s.externalEvents = nil
	s.runtimeLeases = make(map[string]store.RuntimeLease)
	s.policyDecisions = nil
	s.empty = true
	return nil
}

func (s *memStore) Close() error { return nil }

func TestMemStore_ReadCheckpoint_ReturnsZeroValue_WhenEmpty(t *testing.T) {
	s := newMemStore()
	cp, err := s.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, store.Checkpoint{}, cp)
}

func TestMemStore_WriteAndReadCheckpoint_RoundTrip(t *testing.T) {
	s := newMemStore()
	want := store.Checkpoint{State: "SCANNING", ResumeState: "", Phase: 2}

	err := s.WriteCheckpoint(context.Background(), want)
	require.NoError(t, err)

	got, err := s.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestMemStore_DeleteAll_ClearsCheckpoint(t *testing.T) {
	s := newMemStore()
	require.NoError(t, s.WriteCheckpoint(context.Background(), store.Checkpoint{State: "SCANNING", Phase: 1}))
	require.NoError(t, s.DeleteAll(context.Background()))

	cp, err := s.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, store.Checkpoint{}, cp)
}

// --------------------------------------------------------------------------
// SQLite checkpoint read/write/delete tests using ":memory:" (no filesystem access)
// --------------------------------------------------------------------------

func newMemDB(t *testing.T) store.Store {
	t.Helper()
	st, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, st.Close())
	})
	return st
}

func TestSQLiteStore_ReadCheckpoint_ReturnsZeroValue_WhenEmpty(t *testing.T) {
	cp, err := newMemDB(t).ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, store.Checkpoint{}, cp)
}

func TestSQLiteStore_WriteAndReadCheckpoint_RoundTrip(t *testing.T) {
	st := newMemDB(t)

	ts := time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC)
	want := store.Checkpoint{
		State:       "AWAITING_CI",
		ResumeState: "",
		Phase:       3,
		PRNumber:    42,
		IssueNumber: 7,
		RetryCount:  2,
		UpdatedAt:   ts,
	}
	require.NoError(t, st.WriteCheckpoint(context.Background(), want))

	got, err := st.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestSQLiteStore_WriteCheckpoint_Idempotent(t *testing.T) {
	st := newMemDB(t)

	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "SCANNING", Phase: 1}))
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "REVIEWING", Phase: 2}))

	got, err := st.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "REVIEWING", got.State)
	assert.Equal(t, 2, got.Phase)
}

func TestSQLiteStore_WriteCheckpoint_AutoSetsUpdatedAt(t *testing.T) {
	st := newMemDB(t)

	before := time.Now().UTC().Truncate(time.Second)
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "SCANNING", Phase: 1}))

	got, err := st.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.False(t, got.UpdatedAt.IsZero(), "UpdatedAt should be set automatically")
	assert.True(t, !got.UpdatedAt.Before(before), "UpdatedAt should be >= write time")
}

func TestSQLiteStore_DeleteAll_ClearsCheckpoint(t *testing.T) {
	st := newMemDB(t)

	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "MERGING", Phase: 4}))
	require.NoError(t, st.DeleteAll(context.Background()))

	cp, err := st.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, store.Checkpoint{}, cp)
}

func TestSQLiteStore_DeleteAll_ClearsActionLog(t *testing.T) {
	st := newMemDB(t)
	ctx := context.Background()

	require.NoError(t, st.WriteAction(ctx, store.Action{
		SessionID:    "s1",
		OperationKey: "create_issue:E2-US1",
		StateBefore:  "SCANNING",
		StateAfter:   "ISSUE_CREATED",
		Event:        "issue_created",
		Detail:       `{"issue":42}`,
		CreatedAt:    time.Date(2026, 3, 13, 10, 0, 0, 0, time.UTC),
	}))

	require.NoError(t, st.DeleteAll(ctx))

	actions, err := st.ReadActions(ctx, 10)
	require.NoError(t, err)
	assert.NotNil(t, actions)
	assert.Empty(t, actions)
}

func TestSQLiteStore_DeleteAll_AllowsReusingOperationKey(t *testing.T) {
	st := newMemDB(t)
	ctx := context.Background()

	first := store.Action{
		SessionID:    "s1",
		OperationKey: "create_issue:E2-US1",
		StateBefore:  "SCANNING",
		StateAfter:   "ISSUE_CREATED",
		Event:        "issue_created",
		Detail:       `{"issue":42}`,
		CreatedAt:    time.Date(2026, 3, 13, 10, 0, 0, 0, time.UTC),
	}
	reused := store.Action{
		SessionID:    "s2",
		OperationKey: first.OperationKey,
		StateBefore:  "SCANNING",
		StateAfter:   "ISSUE_CREATED",
		Event:        "issue_created",
		Detail:       `{"issue":99}`,
		CreatedAt:    time.Date(2026, 3, 13, 11, 0, 0, 0, time.UTC),
	}

	require.NoError(t, st.WriteAction(ctx, first))
	require.NoError(t, st.DeleteAll(ctx))
	require.NoError(t, st.WriteAction(ctx, reused))

	actions, err := st.ReadActions(ctx, 10)
	require.NoError(t, err)
	require.Len(t, actions, 1)
	assert.Equal(t, reused.OperationKey, actions[0].OperationKey)
	assert.Equal(t, reused.SessionID, actions[0].SessionID)
	assert.Equal(t, reused.Detail, actions[0].Detail)
	assert.Equal(t, reused.CreatedAt, actions[0].CreatedAt)
}
