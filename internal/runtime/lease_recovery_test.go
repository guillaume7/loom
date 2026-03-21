package runtime_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	loomruntime "github.com/guillaume7/loom/internal/runtime"
	"github.com/guillaume7/loom/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func expiredLease(key, holderID string, expiredAt time.Time) store.RuntimeLease {
	return store.RuntimeLease{
		LeaseKey:  key,
		HolderID:  holderID,
		Scope:     "run",
		ExpiresAt: expiredAt,
		CreatedAt: expiredAt.Add(-2 * time.Minute),
		RenewedAt: expiredAt.Add(-time.Minute),
	}
}

func TestRecoverExpiredLease_SucceedsWhenLeaseExpired(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	expiredAt := now.Add(-5 * time.Minute)

	st := newMemStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{
		StoryID: "story-1",
		State:   "open",
	}))

	leaseKey := "run:story-1"
	require.NoError(t, st.UpsertRuntimeLease(context.Background(), expiredLease(leaseKey, "controller-1", expiredAt)))

	cfg := loomruntime.Config{
		HolderID:     "controller-2",
		LeaseTTL:     2 * time.Minute,
		PollInterval: 30 * time.Second,
		Now:          func() time.Time { return now },
	}
	ctrl := loomruntime.NewController(st, cfg)

	result, err := ctrl.RecoverExpiredLease(context.Background())
	require.NoError(t, err)

	assert.Equal(t, leaseKey, result.LeaseKey)
	assert.Equal(t, "controller-1", result.PreviousHolderID)
	assert.Equal(t, expiredAt.UTC(), result.LeaseExpiredAt.UTC())
	assert.NotEmpty(t, result.AuditID)

	// Store should now show controller-2 as holder.
	lease, err := st.ReadRuntimeLease(context.Background(), leaseKey)
	require.NoError(t, err)
	assert.Equal(t, "controller-2", lease.HolderID)

	// A PolicyDecision with kind "lease_recovery" must have been written.
	decisions, err := st.ReadPolicyDecisions(context.Background(), "story-1", 10)
	require.NoError(t, err)
	require.Len(t, decisions, 1)
	assert.Equal(t, "lease_recovery", decisions[0].DecisionKind)
	assert.Equal(t, "recovered", decisions[0].Verdict)

	var detail map[string]any
	require.NoError(t, json.Unmarshal([]byte(decisions[0].Detail), &detail))
	assert.Equal(t, "controller-1", detail["previous_holder"])
	assert.Equal(t, "controller-2", detail["recovery_holder"])
}

func TestRecoverExpiredLease_FailsWhenLeaseStillActive(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	st := newMemStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{
		StoryID: "story-2",
		State:   "open",
	}))

	leaseKey := "run:story-2"
	activeLease := store.RuntimeLease{
		LeaseKey:  leaseKey,
		HolderID:  "controller-1",
		Scope:     "run",
		ExpiresAt: now.Add(5 * time.Minute), // still in the future
		CreatedAt: now.Add(-time.Minute),
		RenewedAt: now,
	}
	require.NoError(t, st.UpsertRuntimeLease(context.Background(), activeLease))

	cfg := loomruntime.Config{
		HolderID:     "controller-2",
		LeaseTTL:     2 * time.Minute,
		PollInterval: 30 * time.Second,
		Now:          func() time.Time { return now },
	}
	ctrl := loomruntime.NewController(st, cfg)

	_, err := ctrl.RecoverExpiredLease(context.Background())
	assert.ErrorIs(t, err, loomruntime.ErrLeaseNotExpired)
}

func TestRecoverExpiredLease_FailsWhenNoLeaseExists(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	st := newMemStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{
		StoryID: "story-3",
		State:   "open",
	}))

	// No lease upserted — store is empty for this key.

	cfg := loomruntime.Config{
		HolderID:     "controller-2",
		LeaseTTL:     2 * time.Minute,
		PollInterval: 30 * time.Second,
		Now:          func() time.Time { return now },
	}
	ctrl := loomruntime.NewController(st, cfg)

	_, err := ctrl.RecoverExpiredLease(context.Background())
	assert.ErrorIs(t, err, loomruntime.ErrLeaseNotFound)
}

func TestRecoverExpiredLease_CommittedActionsReturnedToAvoidReplay(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	expiredAt := now.Add(-5 * time.Minute)

	st := newMemStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{
		StoryID: "story-4",
		State:   "open",
	}))

	// Seed an action that was committed before the recovery.
	require.NoError(t, st.WriteAction(context.Background(), store.Action{
		SessionID:    "story-4",
		OperationKey: "op:create-pr:story-4",
		StateBefore:  "open",
		StateAfter:   "pr_open",
		Event:        "create_pr",
		Detail:       "pr created",
		CreatedAt:    expiredAt.Add(-time.Minute),
	}))

	leaseKey := "run:story-4"
	require.NoError(t, st.UpsertRuntimeLease(context.Background(), expiredLease(leaseKey, "controller-1", expiredAt)))

	cfg := loomruntime.Config{
		HolderID:     "controller-2",
		LeaseTTL:     2 * time.Minute,
		PollInterval: 30 * time.Second,
		Now:          func() time.Time { return now },
	}
	ctrl := loomruntime.NewController(st, cfg)

	result, err := ctrl.RecoverExpiredLease(context.Background())
	require.NoError(t, err)

	require.Len(t, result.CommittedActions, 1)
	assert.Equal(t, "op:create-pr:story-4", result.CommittedActions[0].OperationKey)
}
