package store_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/guillaume7/loom/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// memStore is an in-memory Store used only in tests.
// It satisfies the store.Store interface without any real I/O.
type memStore struct {
	cp    store.Checkpoint
	empty bool
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

func (s *memStore) DeleteAll(_ context.Context) error {
	s.cp = store.Checkpoint{}
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
// SQLite integration tests
// --------------------------------------------------------------------------

func TestSQLiteStore_ReadCheckpoint_ReturnsZeroValue_WhenEmpty(t *testing.T) {
	st, err := store.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)

	cp, err := st.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, store.Checkpoint{}, cp)
}

func TestSQLiteStore_WriteAndReadCheckpoint_RoundTrip(t *testing.T) {
	st, err := store.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)

	want := store.Checkpoint{State: "AWAITING_CI", Phase: 3}
	require.NoError(t, st.WriteCheckpoint(context.Background(), want))

	got, err := st.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestSQLiteStore_WriteCheckpoint_Idempotent(t *testing.T) {
	st, err := store.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)

	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "SCANNING", Phase: 1}))
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "REVIEWING", Phase: 2}))

	got, err := st.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, store.Checkpoint{State: "REVIEWING", Phase: 2}, got)
}

func TestSQLiteStore_DeleteAll_ClearsCheckpoint(t *testing.T) {
	st, err := store.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)

	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "MERGING", Phase: 4}))
	require.NoError(t, st.DeleteAll(context.Background()))

	cp, err := st.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, store.Checkpoint{}, cp)
}

func TestNew_CreatesParentDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	_, err := store.New(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	_, err = os.Stat(dir)
	assert.NoError(t, err, "parent directory should have been created")
}

