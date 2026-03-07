package store_test

import (
	"context"
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
