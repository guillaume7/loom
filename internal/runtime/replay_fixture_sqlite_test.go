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

func TestAssembleReplayFixture_WithSQLiteStoreAndLargeReplayLimit(t *testing.T) {
	ctx := context.Background()
	st, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, st.Close())
	})

	now := time.Date(2026, 3, 21, 11, 0, 0, 0, time.UTC)
	require.NoError(t, st.WriteCheckpoint(ctx, store.Checkpoint{
		State:     "AWAITING_CI",
		Phase:     2,
		PRNumber:  84,
		UpdatedAt: now.Add(-2 * time.Minute),
	}))

	correlationID := "corr-sqlite-1"
	payload, err := json.Marshal(map[string]any{
		"session_id":       "default",
		"correlation_id":   correlationID,
		"wake_kind":        "poll_ci",
		"policy_decision":  "ci_readiness",
		"policy_outcome":   "continue",
		"policy_reason":    "ci_green",
		"previous_state":   "AWAITING_CI",
		"new_state":        "AWAITING_CI",
		"decision_verdict": "continue",
		"pr_number":        84,
		"ci": map[string]any{
			"total_checks": 2,
			"green_checks": 2,
		},
	})
	require.NoError(t, err)

	require.NoError(t, st.WriteExternalEvent(ctx, store.ExternalEvent{
		SessionID:     "default",
		EventSource:   "poll",
		EventKind:     "poll_ci",
		CorrelationID: correlationID,
		Payload:       string(payload),
		ObservedAt:    now.Add(-time.Minute),
	}))
	require.NoError(t, st.WritePolicyDecision(ctx, store.PolicyDecision{
		SessionID:     "default",
		CorrelationID: correlationID,
		DecisionKind:  "ci_readiness",
		Verdict:       "continue",
		InputHash:     "hash-sqlite-1",
		Detail:        string(payload),
		CreatedAt:     now.Add(-30 * time.Second),
	}))
	require.NoError(t, st.WriteAction(ctx, store.Action{
		SessionID:    "default",
		OperationKey: "op-sqlite-1",
		StateBefore:  "AWAITING_CI",
		StateAfter:   "AWAITING_CI",
		Event:        "runtime.poll",
		Detail:       "poll_ci continue",
		CreatedAt:    now,
	}))

	fixture, err := loomruntime.AssembleReplayFixture(ctx, st, now)
	require.NoError(t, err)

	assert.Equal(t, "default", fixture.SessionID)
	assert.Equal(t, "default", fixture.Observations.SessionID)
	assert.Equal(t, "default", fixture.Policies.SessionID)
	require.Len(t, fixture.Events, 1)
	require.Len(t, fixture.Decisions, 1)
	require.Len(t, fixture.Actions, 1)
	require.Len(t, fixture.Observations.CI, 1)
	require.Len(t, fixture.Policies.Entries, 1)
	assert.Equal(t, correlationID, fixture.Events[0].CorrelationID)
	assert.Equal(t, correlationID, fixture.Decisions[0].CorrelationID)
	assert.Equal(t, "op-sqlite-1", fixture.Actions[0].OperationKey)
	assert.Equal(t, correlationID, fixture.Observations.CI[0].CorrelationID)
	assert.Equal(t, correlationID, fixture.Policies.Entries[0].CorrelationID)
}
