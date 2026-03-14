package store_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/guillaume7/loom/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// memStore is an in-memory Store used only in tests.
// It satisfies the store.Store interface without any real I/O.
type memStore struct {
	cp      store.Checkpoint
	actions []store.Action
	empty   bool
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

func TestSQLiteStore_WriteAndReadActions_RoundTrip(t *testing.T) {
	st := newMemDB(t)

	want := store.Action{
		SessionID:    "s1",
		OperationKey: "create_issue:E2-US1",
		StateBefore:  "SCANNING",
		StateAfter:   "ISSUE_CREATED",
		Event:        "issue_created",
		Detail:       `{"issue":42}`,
		CreatedAt:    time.Date(2026, 3, 13, 10, 0, 0, 0, time.UTC),
	}

	require.NoError(t, st.WriteAction(context.Background(), want))

	actions, err := st.ReadActions(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, actions, 1)
	assert.Positive(t, actions[0].ID)
	assert.Equal(t, want.SessionID, actions[0].SessionID)
	assert.Equal(t, want.OperationKey, actions[0].OperationKey)
	assert.Equal(t, want.StateBefore, actions[0].StateBefore)
	assert.Equal(t, want.StateAfter, actions[0].StateAfter)
	assert.Equal(t, want.Event, actions[0].Event)
	assert.Equal(t, want.Detail, actions[0].Detail)
	assert.Equal(t, want.CreatedAt, actions[0].CreatedAt)
}

func TestSQLiteStore_WriteAction_ReturnsDuplicateOperationKeyError(t *testing.T) {
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
	duplicate := store.Action{
		SessionID:    "s2",
		OperationKey: first.OperationKey,
		StateBefore:  "SCANNING",
		StateAfter:   "ISSUE_CREATED",
		Event:        "issue_created",
		Detail:       `{"issue":99}`,
		CreatedAt:    time.Date(2026, 3, 13, 11, 0, 0, 0, time.UTC),
	}

	require.NoError(t, st.WriteAction(ctx, first))
	err := st.WriteAction(ctx, duplicate)
	require.ErrorIs(t, err, store.ErrDuplicateOperationKey)

	actions, readErr := st.ReadActions(ctx, 10)
	require.NoError(t, readErr)
	require.Len(t, actions, 1)
	assert.Equal(t, first.SessionID, actions[0].SessionID)
	assert.Equal(t, first.Detail, actions[0].Detail)
	assert.Equal(t, first.CreatedAt, actions[0].CreatedAt)
}

// --------------------------------------------------------------------------
// WriteCheckpointAndAction atomicity tests (TH2.E2.US3 fix)
// --------------------------------------------------------------------------

func TestSQLiteStore_WriteCheckpointAndAction_AtomicSuccess(t *testing.T) {
	st := newMemDB(t)
	ctx := context.Background()

	cp := store.Checkpoint{State: "SCANNING", Phase: 1}
	action := store.Action{
		SessionID:    "s1",
		OperationKey: "checkpoint:IDLE->SCANNING",
		StateBefore:  "IDLE",
		StateAfter:   "SCANNING",
		Event:        "start",
		Detail:       `{"previous_state":"IDLE","new_state":"SCANNING","phase":1}`,
		CreatedAt:    time.Date(2026, 3, 13, 10, 0, 0, 0, time.UTC),
	}
	require.NoError(t, st.WriteCheckpointAndAction(ctx, cp, action))

	gotCp, err := st.ReadCheckpoint(ctx)
	require.NoError(t, err)
	assert.Equal(t, "SCANNING", gotCp.State)
	assert.Equal(t, 1, gotCp.Phase)

	gotAction, err := st.ReadActionByOperationKey(ctx, action.OperationKey)
	require.NoError(t, err)
	assert.Equal(t, action.StateBefore, gotAction.StateBefore)
	assert.Equal(t, action.StateAfter, gotAction.StateAfter)
	assert.Equal(t, action.Event, gotAction.Event)
	assert.Equal(t, action.Detail, gotAction.Detail)
}

// TestSQLiteStore_WriteCheckpointAndAction_DuplicateKey_RollsBackCheckpoint is
// the core atomicity regression test: when the action write fails with a
// duplicate key, the checkpoint update in the same transaction is rolled back.
// Previously WriteCheckpoint + WriteAction were called sequentially, creating
// a window where the checkpoint was persisted but the action was not.
func TestSQLiteStore_WriteCheckpointAndAction_DuplicateKey_RollsBackCheckpoint(t *testing.T) {
	st := newMemDB(t)
	ctx := context.Background()

	// Persist a baseline checkpoint and an action with a known key.
	require.NoError(t, st.WriteCheckpoint(ctx, store.Checkpoint{State: "SCANNING", Phase: 1}))
	require.NoError(t, st.WriteAction(ctx, store.Action{
		SessionID:    "s1",
		OperationKey: "checkpoint:IDLE->SCANNING",
		StateBefore:  "IDLE",
		StateAfter:   "SCANNING",
		Event:        "start",
		Detail:       `{"previous_state":"IDLE","new_state":"SCANNING","phase":1}`,
		CreatedAt:    time.Date(2026, 3, 13, 10, 0, 0, 0, time.UTC),
	}))

	// Now attempt to atomically advance the checkpoint to ISSUE_CREATED while
	// re-using the existing operation key — the action write should fail with
	// ErrDuplicateOperationKey and the checkpoint must be rolled back.
	err := st.WriteCheckpointAndAction(ctx,
		store.Checkpoint{State: "ISSUE_CREATED", Phase: 1},
		store.Action{
			SessionID:    "s2",
			OperationKey: "checkpoint:IDLE->SCANNING", // duplicate
			StateBefore:  "SCANNING",
			StateAfter:   "ISSUE_CREATED",
			Event:        "phase_identified",
			Detail:       `{"previous_state":"SCANNING","new_state":"ISSUE_CREATED","phase":1}`,
			CreatedAt:    time.Date(2026, 3, 13, 11, 0, 0, 0, time.UTC),
		},
	)
	require.ErrorIs(t, err, store.ErrDuplicateOperationKey)

	// Checkpoint must still be SCANNING — the update was rolled back atomically.
	gotCp, err := st.ReadCheckpoint(ctx)
	require.NoError(t, err)
	assert.Equal(t, "SCANNING", gotCp.State,
		"checkpoint update must be rolled back when action write fails with duplicate key")

	// The action log must still have exactly the original entry.
	actions, err := st.ReadActions(ctx, 10)
	require.NoError(t, err)
	require.Len(t, actions, 1)
	assert.Equal(t, "s1", actions[0].SessionID)
}

func TestSQLiteStore_WriteCheckpointAndAction_AutoSetsTimestamps(t *testing.T) {
	st := newMemDB(t)
	ctx := context.Background()

	before := time.Now().UTC().Truncate(time.Second)
	require.NoError(t, st.WriteCheckpointAndAction(ctx,
		store.Checkpoint{State: "SCANNING", Phase: 1},
		store.Action{
			SessionID:    "s1",
			OperationKey: "checkpoint:IDLE->SCANNING:ts",
			StateBefore:  "IDLE",
			StateAfter:   "SCANNING",
			Event:        "start",
			Detail:       "{}",
		},
	))

	gotCp, err := st.ReadCheckpoint(ctx)
	require.NoError(t, err)
	assert.False(t, gotCp.UpdatedAt.IsZero(), "checkpoint UpdatedAt must be auto-set")
	assert.True(t, !gotCp.UpdatedAt.Before(before), "checkpoint UpdatedAt must be >= write time")

	gotAction, err := st.ReadActionByOperationKey(ctx, "checkpoint:IDLE->SCANNING:ts")
	require.NoError(t, err)
	assert.False(t, gotAction.CreatedAt.IsZero(), "action CreatedAt must be auto-set")
	assert.True(t, !gotAction.CreatedAt.Before(before), "action CreatedAt must be >= write time")
}

func TestSQLiteStore_ReadActionByOperationKey_ReturnsMatch(t *testing.T) {
	st := newMemDB(t)
	ctx := context.Background()
	want := store.Action{
		SessionID:    "s1",
		OperationKey: "checkpoint:IDLE->SCANNING",
		StateBefore:  "IDLE",
		StateAfter:   "SCANNING",
		Event:        "start",
		Detail:       `{"state":"SCANNING"}`,
		CreatedAt:    time.Date(2026, 3, 13, 10, 0, 0, 0, time.UTC),
	}

	require.NoError(t, st.WriteAction(ctx, want))

	got, err := st.ReadActionByOperationKey(ctx, want.OperationKey)
	require.NoError(t, err)
	assert.Positive(t, got.ID)
	assert.Equal(t, want.SessionID, got.SessionID)
	assert.Equal(t, want.OperationKey, got.OperationKey)
	assert.Equal(t, want.StateBefore, got.StateBefore)
	assert.Equal(t, want.StateAfter, got.StateAfter)
	assert.Equal(t, want.Event, got.Event)
	assert.Equal(t, want.Detail, got.Detail)
	assert.Equal(t, want.CreatedAt, got.CreatedAt)
}

func TestSQLiteStore_ReadActionByOperationKey_ReturnsNotFound(t *testing.T) {
	st := newMemDB(t)

	_, err := st.ReadActionByOperationKey(context.Background(), "missing-operation")
	require.ErrorIs(t, err, store.ErrActionNotFound)
}

func TestSQLiteStore_ReadActions_AppliesLimitAndNewestFirst(t *testing.T) {
	st := newMemDB(t)
	ctx := context.Background()
	base := time.Date(2026, 3, 13, 10, 0, 0, 0, time.UTC)

	for index := 0; index < 10; index++ {
		action := store.Action{
			SessionID:    "s1",
			OperationKey: "operation-" + string(rune('a'+index)),
			StateBefore:  "SCANNING",
			StateAfter:   "ISSUE_CREATED",
			Event:        "issue_created",
			Detail:       "{}",
			CreatedAt:    base.Add(time.Duration(index) * time.Minute),
		}
		require.NoError(t, st.WriteAction(ctx, action))
	}

	actions, err := st.ReadActions(ctx, 3)
	require.NoError(t, err)
	require.Len(t, actions, 3)
	assert.Equal(t, "operation-j", actions[0].OperationKey)
	assert.Equal(t, "operation-i", actions[1].OperationKey)
	assert.Equal(t, "operation-h", actions[2].OperationKey)
	assert.True(t, actions[0].CreatedAt.After(actions[1].CreatedAt))
	assert.True(t, actions[1].CreatedAt.After(actions[2].CreatedAt))
}

func TestSQLiteStore_ReadActions_ReturnsEmptySlice_WhenTableEmpty(t *testing.T) {
	st := newMemDB(t)

	actions, err := st.ReadActions(context.Background(), 10)
	require.NoError(t, err)
	assert.NotNil(t, actions)
	assert.Empty(t, actions)
}

func TestSQLiteStore_ReadActions_ReturnsEmptySlice_WhenLimitIsZero(t *testing.T) {
	st := newMemDB(t)

	require.NoError(t, st.WriteAction(context.Background(), store.Action{
		SessionID:    "s1",
		OperationKey: "create_issue:E2-US1",
		StateBefore:  "SCANNING",
		StateAfter:   "ISSUE_CREATED",
		Event:        "issue_created",
		Detail:       `{"issue":42}`,
		CreatedAt:    time.Date(2026, 3, 13, 10, 0, 0, 0, time.UTC),
	}))

	actions, err := st.ReadActions(context.Background(), 0)
	require.NoError(t, err)
	assert.NotNil(t, actions)
	assert.Empty(t, actions)
}

// --------------------------------------------------------------------------
// SQLite filesystem tests
// --------------------------------------------------------------------------

func TestNew_CreatesParentDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	st, err := store.New(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	require.NoError(t, st.Close())
	_, err = os.Stat(dir)
	assert.NoError(t, err, "parent directory should have been created")
}
