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

func TestDetectResumeConflict_NoConflictWhenObservationsFresh(t *testing.T) {
	st := newMemStore()
	now := time.Now().UTC()
	cfg := loomruntime.Config{
		HolderID: "controller-1",
		LeaseTTL: 2 * time.Minute,
		Now:      func() time.Time { return now },
	}
	c := loomruntime.NewController(st, cfg)

	model := loomruntime.ObservationModel{
		SessionID: "default",
		Summaries: loomruntime.ObservationSummaries{
			CI: &loomruntime.CISummary{
				RecordedAt: now.Add(-10 * time.Second),
			},
			Review: &loomruntime.ReviewSummary{
				RecordedAt: now.Add(-10 * time.Second),
			},
		},
	}
	wake := store.WakeSchedule{WakeKind: "poll_ci", DueAt: now}

	conflict, err := c.DetectResumeConflict(context.Background(), wake, 0, model)
	require.NoError(t, err)
	assert.Nil(t, conflict)
}

func TestDetectResumeConflict_StaleObservationDetected(t *testing.T) {
	st := newMemStore()
	now := time.Now().UTC()
	cfg := loomruntime.Config{
		HolderID: "controller-1",
		LeaseTTL: 2 * time.Minute,
		Now:      func() time.Time { return now },
	}
	c := loomruntime.NewController(st, cfg)

	// RecordedAt is 3× DefaultObservationStaleAfter in the past.
	staleTime := now.Add(-3 * loomruntime.DefaultObservationStaleAfter)
	model := loomruntime.ObservationModel{
		SessionID: "default",
		Summaries: loomruntime.ObservationSummaries{
			CI: &loomruntime.CISummary{
				RecordedAt: staleTime,
			},
		},
	}
	wake := store.WakeSchedule{WakeKind: "poll_ci", DueAt: now}

	conflict, err := c.DetectResumeConflict(context.Background(), wake, 0, model)
	require.NoError(t, err)
	require.NotNil(t, conflict)
	assert.Equal(t, loomruntime.ResumeConflictStaleObservation, conflict.Kind)
	assert.Equal(t, loomruntime.PolicyOutcomeBlock, conflict.Outcome)
}

func TestDetectResumeConflict_LockContentionDetected(t *testing.T) {
	st := newMemStore()
	now := time.Now().UTC()

	// controller-1 acquires the PR lock first.
	cfg1 := loomruntime.Config{
		HolderID: "controller-1",
		LeaseTTL: 2 * time.Minute,
		Now:      func() time.Time { return now },
	}
	c1 := loomruntime.NewController(st, cfg1)
	_, err := c1.AcquirePRLock(context.Background(), 42)
	require.NoError(t, err)

	// controller-2 attempts DetectResumeConflict with fresh observations (no stale conflict).
	cfg2 := loomruntime.Config{
		HolderID: "controller-2",
		LeaseTTL: 2 * time.Minute,
		Now:      func() time.Time { return now },
	}
	c2 := loomruntime.NewController(st, cfg2)

	model := loomruntime.ObservationModel{
		SessionID: "default",
		Summaries: loomruntime.ObservationSummaries{},
	}
	wake := store.WakeSchedule{WakeKind: "poll_ci", DueAt: now}

	conflict, err := c2.DetectResumeConflict(context.Background(), wake, 42, model)
	require.NoError(t, err)
	require.NotNil(t, conflict)
	assert.Equal(t, loomruntime.ResumeConflictLockContention, conflict.Kind)
	assert.Equal(t, loomruntime.PolicyOutcomeWait, conflict.Outcome)
	assert.Equal(t, "controller-1", conflict.ConflictHolder)
}

func TestDetectResumeConflict_SupersededWakeDetected(t *testing.T) {
	st := newMemStore()
	now := time.Now().UTC()
	cfg := loomruntime.Config{
		HolderID: "controller-1",
		LeaseTTL: 2 * time.Minute,
		Now:      func() time.Time { return now },
	}
	c := loomruntime.NewController(st, cfg)

	// ClaimedAt is 3× DefaultLeaseTTL in the past — clearly older than one lease TTL.
	claimedAt := now.Add(-3 * loomruntime.DefaultLeaseTTL)
	wake := store.WakeSchedule{
		WakeKind:  "poll_ci",
		DueAt:     claimedAt,
		ClaimedAt: claimedAt,
	}

	// Checkpoint.UpdatedAt is now, which is claimedAt + 3×DefaultLeaseTTL > DefaultLeaseTTL.
	model := loomruntime.ObservationModel{
		SessionID: "default",
		Checkpoint: loomruntime.CheckpointObservation{
			UpdatedAt: now,
		},
		Summaries: loomruntime.ObservationSummaries{
			// Fresh summaries so staleness check doesn't trigger first.
			CI: &loomruntime.CISummary{
				RecordedAt: now.Add(-10 * time.Second),
			},
			Review: &loomruntime.ReviewSummary{
				RecordedAt: now.Add(-10 * time.Second),
			},
		},
	}

	conflict, err := c.DetectResumeConflict(context.Background(), wake, 0, model)
	require.NoError(t, err)
	require.NotNil(t, conflict)
	assert.Equal(t, loomruntime.ResumeConflictSupersededWake, conflict.Kind)
	assert.Equal(t, loomruntime.PolicyOutcomeWait, conflict.Outcome)
}

func TestDetectResumeConflict_NoPRConflictAcquiresLock(t *testing.T) {
	st := newMemStore()
	now := time.Now().UTC()
	cfg1 := loomruntime.Config{
		HolderID: "controller-1",
		LeaseTTL: 2 * time.Minute,
		Now:      func() time.Time { return now },
	}
	c1 := loomruntime.NewController(st, cfg1)

	model := loomruntime.ObservationModel{
		SessionID: "default",
		Summaries: loomruntime.ObservationSummaries{
			CI: &loomruntime.CISummary{
				RecordedAt: now.Add(-10 * time.Second),
			},
			Review: &loomruntime.ReviewSummary{
				RecordedAt: now.Add(-10 * time.Second),
			},
		},
	}
	// Unclaimed wake (ClaimedAt zero).
	wake := store.WakeSchedule{WakeKind: "poll_ci", DueAt: now}

	conflict, err := c1.DetectResumeConflict(context.Background(), wake, 42, model)
	require.NoError(t, err)
	assert.Nil(t, conflict)

	// PR lock should now be held by controller-1.
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

func TestRecordResumeConflict_WritesExternalEventAndPolicyDecision(t *testing.T) {
	st := newMemStore()
	now := time.Now().UTC()
	cfg := loomruntime.Config{
		HolderID: "controller-1",
		LeaseTTL: 2 * time.Minute,
		Now:      func() time.Time { return now },
	}
	c := loomruntime.NewController(st, cfg)

	conflict := loomruntime.ResumeConflict{
		Kind:    loomruntime.ResumeConflictStaleObservation,
		Outcome: loomruntime.PolicyOutcomeBlock,
		Reason:  "observation_model_stale",
	}
	sessionID := "sess-001"
	correlationID := "corr-001"

	err := c.RecordResumeConflict(context.Background(), conflict, sessionID, correlationID)
	require.NoError(t, err)

	// Verify ExternalEvent was written.
	events, err := st.ReadExternalEvents(context.Background(), sessionID, 10)
	require.NoError(t, err)
	require.Len(t, events, 1)
	ev := events[0]
	assert.Equal(t, sessionID, ev.SessionID)
	assert.Equal(t, "runtime", ev.EventSource)
	assert.Equal(t, correlationID, ev.CorrelationID)

	var payload map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(ev.Payload), &payload))
	assert.Equal(t, string(loomruntime.ResumeConflictStaleObservation), payload["kind"])
	assert.Equal(t, string(loomruntime.PolicyOutcomeBlock), payload["outcome"])
	assert.Equal(t, "observation_model_stale", payload["reason"])

	// Verify PolicyDecision was written.
	decisions, err := st.ReadPolicyDecisions(context.Background(), sessionID, 10)
	require.NoError(t, err)
	require.Len(t, decisions, 1)
	pd := decisions[0]
	assert.Equal(t, sessionID, pd.SessionID)
	assert.Equal(t, correlationID, pd.CorrelationID)
	assert.Equal(t, string(loomruntime.PolicyOutcomeBlock), pd.Verdict)
}
