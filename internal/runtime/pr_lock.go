package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/guillaume7/loom/internal/store"
)

const (
	PRLockScope = "pr"
)

// PRLockKey returns the lease key used to claim a mutation lock on a specific PR.
func PRLockKey(prNumber int) string {
	return fmt.Sprintf("pr:%d", prNumber)
}

// PRLockResult describes the outcome of a PR lock acquisition attempt.
type PRLockResult struct {
	Acquired     bool
	LeaseKey     string
	HolderID     string
	ContentionBy string // HolderID of the current holder if not acquired
	ExpiresAt    time.Time
}

// AcquirePRLock attempts to claim a mutation lock for the given PR number.
// If an active non-expired lease is held by a different holder, acquisition
// fails and PRLockResult.Acquired is false with ContentionBy set.
// Read-only callers should NOT call this function.
func (c *Controller) AcquirePRLock(ctx context.Context, prNumber int) (PRLockResult, error) {
	now := c.cfg.Now().UTC()
	key := PRLockKey(prNumber)

	existing, err := c.store.ReadRuntimeLease(ctx, key)
	if err != nil && err != store.ErrRuntimeLeaseNotFound {
		return PRLockResult{}, err
	}

	// If an active lease exists and belongs to a different holder, report contention.
	if err == nil && existing.ExpiresAt.After(now) && existing.HolderID != c.cfg.HolderID {
		return PRLockResult{
			Acquired:     false,
			LeaseKey:     key,
			HolderID:     existing.HolderID,
			ContentionBy: existing.HolderID,
			ExpiresAt:    existing.ExpiresAt,
		}, nil
	}

	// Claim or renew the lock.
	lease := store.RuntimeLease{
		LeaseKey:  key,
		HolderID:  c.cfg.HolderID,
		Scope:     PRLockScope,
		ExpiresAt: now.Add(c.cfg.LeaseTTL),
		CreatedAt: existing.CreatedAt,
		RenewedAt: now,
	}
	if lease.CreatedAt.IsZero() {
		lease.CreatedAt = now
	}

	if err := c.store.UpsertRuntimeLease(ctx, lease); err != nil {
		return PRLockResult{}, err
	}

	return PRLockResult{
		Acquired:  true,
		LeaseKey:  key,
		HolderID:  c.cfg.HolderID,
		ExpiresAt: lease.ExpiresAt,
	}, nil
}

// ReleasePRLock expires the mutation lock for the given PR number if held by
// this controller. It is a no-op if the lock is not held by this controller.
func (c *Controller) ReleasePRLock(ctx context.Context, prNumber int) error {
	now := c.cfg.Now().UTC()
	key := PRLockKey(prNumber)

	existing, err := c.store.ReadRuntimeLease(ctx, key)
	if err != nil {
		if err == store.ErrRuntimeLeaseNotFound {
			return nil
		}
		return err
	}

	if existing.HolderID != c.cfg.HolderID {
		return nil // not ours to release
	}

	existing.ExpiresAt = now
	existing.RenewedAt = now
	return c.store.UpsertRuntimeLease(ctx, existing)
}
