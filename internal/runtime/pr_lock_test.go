package runtime_test

import (
	"context"
	"testing"
	"time"

	loomruntime "github.com/guillaume7/loom/internal/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAcquirePRLock_SucceedsWhenNoExistingLock(t *testing.T) {
	st := newMemStore()
	cfg := loomruntime.Config{
		HolderID: "controller-1",
		LeaseTTL: 2 * time.Minute,
		Now:      func() time.Time { return time.Now() },
	}
	c := loomruntime.NewController(st, cfg)

	result, err := c.AcquirePRLock(context.Background(), 42)
	require.NoError(t, err)
	assert.True(t, result.Acquired)
	assert.Equal(t, "controller-1", result.HolderID)
	assert.Equal(t, loomruntime.PRLockKey(42), result.LeaseKey)
}

func TestAcquirePRLock_FailsWhenActiveLockHeldByOtherController(t *testing.T) {
	st := newMemStore()
	now := time.Now().UTC()

	// controller-1 acquires the lock.
	cfg1 := loomruntime.Config{
		HolderID: "controller-1",
		LeaseTTL: 2 * time.Minute,
		Now:      func() time.Time { return now },
	}
	c1 := loomruntime.NewController(st, cfg1)
	_, err := c1.AcquirePRLock(context.Background(), 42)
	require.NoError(t, err)

	// controller-2 attempts to acquire the same lock while it is active.
	cfg2 := loomruntime.Config{
		HolderID: "controller-2",
		LeaseTTL: 2 * time.Minute,
		Now:      func() time.Time { return now },
	}
	c2 := loomruntime.NewController(st, cfg2)
	result, err := c2.AcquirePRLock(context.Background(), 42)
	require.NoError(t, err)
	assert.False(t, result.Acquired)
	assert.Equal(t, "controller-1", result.ContentionBy)
}

func TestReleasePRLock_ExpiresTheLeaseForOwningController(t *testing.T) {
	st := newMemStore()
	now := time.Now().UTC()
	cfg := loomruntime.Config{
		HolderID: "controller-1",
		LeaseTTL: 2 * time.Minute,
		Now:      func() time.Time { return now },
	}
	c := loomruntime.NewController(st, cfg)

	_, err := c.AcquirePRLock(context.Background(), 42)
	require.NoError(t, err)

	err = c.ReleasePRLock(context.Background(), 42)
	require.NoError(t, err)

	lease, err := st.ReadRuntimeLease(context.Background(), loomruntime.PRLockKey(42))
	require.NoError(t, err)
	assert.False(t, lease.ExpiresAt.After(now), "expected ExpiresAt <= now after release")
}

func TestAcquirePRLock_SucceedsAfterOtherControllerLockExpires(t *testing.T) {
	st := newMemStore()
	past := time.Now().UTC().Add(-5 * time.Minute)
	present := time.Now().UTC()

	// controller-1 acquires a lock in the past with a 1-second TTL (already expired).
	cfg1 := loomruntime.Config{
		HolderID: "controller-1",
		LeaseTTL: 1 * time.Second,
		Now:      func() time.Time { return past },
	}
	c1 := loomruntime.NewController(st, cfg1)
	_, err := c1.AcquirePRLock(context.Background(), 42)
	require.NoError(t, err)

	// controller-2 acquires after expiry.
	cfg2 := loomruntime.Config{
		HolderID: "controller-2",
		LeaseTTL: 2 * time.Minute,
		Now:      func() time.Time { return present },
	}
	c2 := loomruntime.NewController(st, cfg2)
	result, err := c2.AcquirePRLock(context.Background(), 42)
	require.NoError(t, err)
	assert.True(t, result.Acquired)
	assert.Equal(t, "controller-2", result.HolderID)

	// Store must reflect controller-2 as holder.
	lease, err := st.ReadRuntimeLease(context.Background(), loomruntime.PRLockKey(42))
	require.NoError(t, err)
	assert.Equal(t, "controller-2", lease.HolderID)
}
