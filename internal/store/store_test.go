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
	cp          store.Checkpoint
	actions     []store.Action
	traces      []store.SessionTrace
	traceEvents []store.TraceEvent
	empty       bool
}

func newMemStore() *memStore { return &memStore{empty: true} }

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

func (s *memStore) DeleteAll(_ context.Context) error {
	s.cp = store.Checkpoint{}
	s.actions = nil
	s.traces = nil
	s.traceEvents = nil
	s.empty = true
	return nil
}

func (s *memStore) Close() error { return nil }

func (s *memStore) OpenSessionTrace(_ context.Context, trace store.SessionTrace) error {
	for _, t := range s.traces {
		if t.SessionID == trace.SessionID {
			return nil
		}
	}
	if trace.Outcome == "" {
		trace.Outcome = "in_progress"
	}
	s.traces = append(s.traces, trace)
	return nil
}

func (s *memStore) AppendTraceEvent(_ context.Context, ev store.TraceEvent) error {
	seq := 1
	for _, e := range s.traceEvents {
		if e.SessionID == ev.SessionID && e.Seq >= seq {
			seq = e.Seq + 1
		}
	}
	ev.Seq = seq
	ev.ID = int64(len(s.traceEvents) + 1)
	s.traceEvents = append(s.traceEvents, ev)
	return nil
}

func (s *memStore) CloseSessionTrace(_ context.Context, sessionID, outcome string) error {
	if outcome == "" {
		outcome = "complete"
	}
	for i, t := range s.traces {
		if t.SessionID == sessionID {
			s.traces[i].EndedAt = time.Now()
			s.traces[i].Outcome = outcome
			return nil
		}
	}
	return nil
}

func (s *memStore) ReadSessionTrace(_ context.Context, sessionID string) (store.SessionTrace, []store.TraceEvent, error) {
	var found *store.SessionTrace
	for i := range s.traces {
		if s.traces[i].SessionID == sessionID {
			found = &s.traces[i]
			break
		}
	}
	if found == nil {
		return store.SessionTrace{}, nil, nil
	}
	var events []store.TraceEvent
	for _, e := range s.traceEvents {
		if e.SessionID == sessionID {
			events = append(events, e)
		}
	}
	return *found, events, nil
}

func (s *memStore) ListSessionTraces(_ context.Context, limit int) ([]store.SessionTrace, error) {
	if limit <= 0 {
		return []store.SessionTrace{}, nil
	}
	result := make([]store.SessionTrace, 0, limit)
	for i := len(s.traces) - 1; i >= 0 && len(result) < limit; i-- {
		result = append(result, s.traces[i])
	}
	return result, nil
}

func TestMemStore_ReadCheckpoint_ReturnsZeroValue_WhenEmpty(t *testing.T) {
	s := newMemStore()
	cp, err := s.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, store.Checkpoint{}, cp)
}

func TestMemStore_WriteAndReadCheckpoint_RoundTrip(t *testing.T) {
	s := newMemStore()
	want := store.Checkpoint{State: "SCANNING", Phase: 2}

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
