package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/guillaume7/loom/internal/store"
)

// ErrLeaseNotExpired is returned when RecoverExpiredLease is called but the
// lease is still active.
var ErrLeaseNotExpired = errors.New("runtime: cannot recover: lease is not expired")

// ErrLeaseNotFound is returned when RecoverExpiredLease is called but no
// lease exists for the run.
var ErrLeaseNotFound = errors.New("runtime: cannot recover: no lease exists for run")

const readAllCommittedActionsLimit = math.MaxInt

// LeaseRecoveryResult describes the outcome of an explicit lease recovery attempt.
type LeaseRecoveryResult struct {
	// LeaseKey is the key that was recovered.
	LeaseKey string
	// PreviousHolderID is the holder whose lease expired.
	PreviousHolderID string
	// LeaseExpiredAt is when the previous lease expired.
	LeaseExpiredAt time.Time
	// CommittedActions contains all actions already committed before recovery,
	// so the recovering controller can skip any that were already done.
	CommittedActions []store.Action
	// AuditID is the session-scoped correlation ID for the recovery record.
	AuditID string
}

// RecoverExpiredLease attempts to claim ownership of a run whose previous
// controller's lease has expired. Returns ErrLeaseNotExpired if the lease is
// still active. Returns ErrLeaseNotFound if no lease exists. On success,
// writes a PolicyDecision audit record and returns the list of committed
// actions to allow the recovering controller to avoid repeating side effects.
func (c *Controller) RecoverExpiredLease(ctx context.Context) (LeaseRecoveryResult, error) {
	cp, err := c.store.ReadCheckpoint(ctx)
	if err != nil {
		return LeaseRecoveryResult{}, err
	}

	now := c.cfg.Now().UTC()
	key := LeaseKey(cp)

	existing, err := c.store.ReadRuntimeLease(ctx, key)
	if err != nil {
		if errors.Is(err, store.ErrRuntimeLeaseNotFound) {
			return LeaseRecoveryResult{}, ErrLeaseNotFound
		}
		return LeaseRecoveryResult{}, err
	}

	// AC1: Lease is expired when ExpiresAt is in the past.
	if existing.ExpiresAt.After(now) {
		return LeaseRecoveryResult{}, ErrLeaseNotExpired
	}

	// AC2: Collect already-committed actions to report to caller so they are
	// not re-executed.
	committed, err := c.store.ReadActions(ctx, readAllCommittedActionsLimit)
	if err != nil {
		return LeaseRecoveryResult{}, err
	}
	sessionID := RunIdentifier(cp)
	filteredCommitted := make([]store.Action, 0, len(committed))
	for _, action := range committed {
		if action.SessionID != sessionID {
			continue
		}
		filteredCommitted = append(filteredCommitted, action)
	}

	// Claim the lease under the recovering holder.
	newLease := store.RuntimeLease{
		LeaseKey:  key,
		HolderID:  c.cfg.HolderID,
		Scope:     controllerLeaseScopeRun,
		ExpiresAt: now.Add(c.cfg.LeaseTTL),
		CreatedAt: existing.CreatedAt,
		RenewedAt: now,
	}
	if err := c.store.UpsertRuntimeLease(ctx, newLease); err != nil {
		return LeaseRecoveryResult{}, err
	}

	// AC3: Write audit record for this recovery attempt.
	auditID := fmt.Sprintf("lease_recovery:%s:%s", key, now.Format(time.RFC3339))
	detail, _ := json.Marshal(map[string]any{
		"previous_holder":   existing.HolderID,
		"lease_expired_at":  existing.ExpiresAt.Format(time.RFC3339),
		"recovery_holder":   c.cfg.HolderID,
		"committed_actions": len(filteredCommitted),
	})
	auditRecord := store.PolicyDecision{
		SessionID:     sessionID,
		CorrelationID: auditID,
		DecisionKind:  "lease_recovery",
		Verdict:       "recovered",
		Detail:        string(detail),
		CreatedAt:     now,
	}
	if err := c.store.WritePolicyDecision(ctx, auditRecord); err != nil {
		return LeaseRecoveryResult{}, err
	}

	return LeaseRecoveryResult{
		LeaseKey:         key,
		PreviousHolderID: existing.HolderID,
		LeaseExpiredAt:   existing.ExpiresAt,
		CommittedActions: filteredCommitted,
		AuditID:          auditID,
	}, nil
}
