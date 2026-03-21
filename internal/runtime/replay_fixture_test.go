package runtime_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	loomruntime "github.com/guillaume7/loom/internal/runtime"
	"github.com/guillaume7/loom/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssembleReplayFixture_CapturesCheckpointObservationsPoliciesAndActions(t *testing.T) {
	ctx := context.Background()
	st := newMemStore()
	now := time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC)

	cp := store.Checkpoint{
		StoryID:    "story-fixture",
		State:      "AWAITING_CI",
		Phase:      2,
		PRNumber:   42,
		UpdatedAt:  now.Add(-2 * time.Minute),
		RetryCount: 1,
	}
	require.NoError(t, st.WriteCheckpoint(ctx, cp))
	require.NoError(t, st.WriteExternalEvent(ctx, store.ExternalEvent{
		SessionID:     "story-fixture",
		EventSource:   "manual_override",
		EventKind:     "operator.pause",
		CorrelationID: "evt-1",
		Payload:       `{"action":"pause","session_id":"story-fixture","correlation_id":"evt-1","requested_by":"octocat","reason":"hold"}`,
		ObservedAt:    now.Add(-time.Minute),
	}))

	decisionDetail, err := json.Marshal(map[string]any{
		"session_id":       "story-fixture",
		"correlation_id":   "decision-1",
		"wake_kind":        "poll_ci",
		"policy_decision":  "ci_readiness",
		"policy_outcome":   "continue",
		"policy_reason":    "ci_green",
		"previous_state":   "AWAITING_CI",
		"new_state":        "AWAITING_CI",
		"decision_verdict": "continue",
		"pr_number":        42,
		"retry_count":      1,
		"ci": map[string]any{
			"total_checks": 2,
			"green_checks": 2,
		},
	})
	require.NoError(t, err)
	require.NoError(t, st.WritePolicyDecision(ctx, store.PolicyDecision{
		SessionID:     "story-fixture",
		CorrelationID: "decision-1",
		DecisionKind:  "ci_readiness",
		Verdict:       "continue",
		InputHash:     "hash-1",
		Detail:        string(decisionDetail),
		CreatedAt:     now,
	}))
	require.NoError(t, st.WriteAction(ctx, store.Action{
		SessionID:    "story-fixture",
		OperationKey: "op-1",
		StateBefore:  "AWAITING_CI",
		StateAfter:   "AWAITING_CI",
		Event:        "runtime.poll",
		Detail:       "poll_ci continue",
		CreatedAt:    now,
	}))

	fixture, err := loomruntime.AssembleReplayFixture(ctx, st, now)
	require.NoError(t, err)

	assert.Equal(t, "story-fixture", fixture.SessionID)
	assert.Equal(t, cp, fixture.Checkpoint)
	assert.Equal(t, now, fixture.CapturedAt)
	assert.Equal(t, "story-fixture", fixture.Observations.SessionID)
	assert.NotEmpty(t, fixture.Observations.Checkpoint.State)
	assert.Equal(t, "story-fixture", fixture.Policies.SessionID)
	assert.NotEmpty(t, fixture.Policies.Entries)
	assert.Len(t, fixture.Events, 1)
	assert.Len(t, fixture.Decisions, 1)
	assert.Len(t, fixture.Actions, 1)
	assert.Equal(t, "story-fixture", fixture.Actions[0].SessionID)
}

func TestAssembleReplayFixture_FiltersActionsToCurrentSession(t *testing.T) {
	ctx := context.Background()
	st := newMemStore()
	now := time.Date(2026, 3, 21, 10, 5, 0, 0, time.UTC)

	require.NoError(t, st.WriteCheckpoint(ctx, store.Checkpoint{
		StoryID:   "story-fixture",
		State:     "AWAITING_CI",
		Phase:     2,
		PRNumber:  42,
		UpdatedAt: now,
	}))
	require.NoError(t, st.WriteAction(ctx, store.Action{
		SessionID:    "story-fixture",
		OperationKey: "op-story",
		Event:        "runtime.poll",
		CreatedAt:    now,
	}))
	require.NoError(t, st.WriteAction(ctx, store.Action{
		SessionID:    "other-session",
		OperationKey: "op-other",
		Event:        "runtime.poll",
		CreatedAt:    now.Add(time.Second),
	}))

	fixture, err := loomruntime.AssembleReplayFixture(ctx, st, now)
	require.NoError(t, err)

	require.Len(t, fixture.Actions, 1)
	assert.Equal(t, "story-fixture", fixture.Actions[0].SessionID)
	assert.Equal(t, "op-story", fixture.Actions[0].OperationKey)
}

func TestAssembleReplayFixture_ReadsOlderSameSessionActionsBeyondRecentGlobalWindow(t *testing.T) {
	ctx := context.Background()
	st := newMemStore()
	now := time.Date(2026, 3, 21, 10, 7, 0, 0, time.UTC)

	require.NoError(t, st.WriteCheckpoint(ctx, store.Checkpoint{
		StoryID:   "story-fixture",
		State:     "AWAITING_CI",
		Phase:     2,
		PRNumber:  42,
		UpdatedAt: now,
	}))
	require.NoError(t, st.WriteAction(ctx, store.Action{
		SessionID:    "story-fixture",
		OperationKey: "op-story-old",
		Event:        "runtime.poll",
		CreatedAt:    now.Add(-2 * time.Hour),
	}))

	for i := range 1251 {
		require.NoError(t, st.WriteAction(ctx, store.Action{
			SessionID:    "other-session",
			OperationKey: fmt.Sprintf("op-other-%04d", i),
			Event:        "runtime.poll",
			CreatedAt:    now.Add(time.Duration(i) * time.Second),
		}))
	}

	fixture, err := loomruntime.AssembleReplayFixture(ctx, st, now)
	require.NoError(t, err)

	assert.Len(t, fixture.Actions, 1)
	assert.Equal(t, "story-fixture", fixture.Actions[0].SessionID)
	assert.Equal(t, "op-story-old", fixture.Actions[0].OperationKey)
}

func TestAssembleReplayFixture_ReadsFullSessionObservationHistoryBeyondDefaultWindow(t *testing.T) {
	ctx := context.Background()
	st := newMemStore()
	now := time.Date(2026, 3, 21, 10, 8, 0, 0, time.UTC)

	require.NoError(t, st.WriteCheckpoint(ctx, store.Checkpoint{
		StoryID:   "story-fixture",
		State:     "AWAITING_CI",
		Phase:     2,
		PRNumber:  42,
		UpdatedAt: now,
	}))

	const recordsPerKind = 121
	for i := range recordsPerKind {
		correlationID := fmt.Sprintf("corr-%03d", i)
		payload, err := json.Marshal(map[string]any{
			"session_id":       "story-fixture",
			"correlation_id":   correlationID,
			"wake_kind":        "poll_ci",
			"policy_decision":  "ci_readiness",
			"policy_outcome":   "continue",
			"policy_reason":    fmt.Sprintf("ci_green_%03d", i),
			"previous_state":   "AWAITING_CI",
			"new_state":        "AWAITING_CI",
			"decision_verdict": "continue",
			"pr_number":        42,
			"ci": map[string]any{
				"total_checks": 1,
				"green_checks": 1,
			},
		})
		require.NoError(t, err)

		require.NoError(t, st.WriteExternalEvent(ctx, store.ExternalEvent{
			SessionID:     "story-fixture",
			EventSource:   "poll",
			EventKind:     "poll_ci",
			CorrelationID: correlationID,
			Payload:       string(payload),
			ObservedAt:    now.Add(time.Duration(i) * time.Second),
		}))
		require.NoError(t, st.WritePolicyDecision(ctx, store.PolicyDecision{
			SessionID:     "story-fixture",
			CorrelationID: correlationID,
			DecisionKind:  "ci_readiness",
			Verdict:       "continue",
			InputHash:     fmt.Sprintf("hash-%03d", i),
			Detail:        string(payload),
			CreatedAt:     now.Add(time.Duration(i) * time.Second),
		}))
	}

	fixture, err := loomruntime.AssembleReplayFixture(ctx, st, now)
	require.NoError(t, err)

	assert.Len(t, fixture.Events, recordsPerKind)
	assert.Len(t, fixture.Decisions, recordsPerKind)
	assert.Len(t, fixture.Observations.CI, recordsPerKind)
	assert.Len(t, fixture.Policies.Entries, recordsPerKind)
	assert.Equal(t, "corr-120", fixture.Events[0].CorrelationID)
	assert.Equal(t, "corr-120", fixture.Decisions[0].CorrelationID)
}

func TestAssembleReplayFixture_ReadsFullPolicyAuditHistoryBeyondLegacyLimit(t *testing.T) {
	ctx := context.Background()
	st := newMemStore()
	now := time.Date(2026, 3, 21, 10, 9, 0, 0, time.UTC)

	require.NoError(t, st.WriteCheckpoint(ctx, store.Checkpoint{
		StoryID:   "story-fixture",
		State:     "AWAITING_CI",
		Phase:     2,
		PRNumber:  42,
		UpdatedAt: now,
	}))

	const decisionCount = 1006
	for i := range decisionCount {
		correlationID := fmt.Sprintf("policy-%04d", i)
		payload, err := json.Marshal(map[string]any{
			"session_id":       "story-fixture",
			"correlation_id":   correlationID,
			"wake_kind":        "poll_ci",
			"policy_decision":  "ci_readiness",
			"policy_outcome":   "continue",
			"policy_reason":    fmt.Sprintf("ci_green_%04d", i),
			"previous_state":   "AWAITING_CI",
			"new_state":        "AWAITING_CI",
			"decision_verdict": "continue",
			"pr_number":        42,
			"retry_count":      i,
			"ci": map[string]any{
				"total_checks": 1,
				"green_checks": 1,
			},
		})
		require.NoError(t, err)

		require.NoError(t, st.WritePolicyDecision(ctx, store.PolicyDecision{
			SessionID:     "story-fixture",
			CorrelationID: correlationID,
			DecisionKind:  "ci_readiness",
			Verdict:       "continue",
			InputHash:     fmt.Sprintf("hash-%04d", i),
			Detail:        string(payload),
			CreatedAt:     now.Add(time.Duration(i) * time.Second),
		}))
	}

	fixture, err := loomruntime.AssembleReplayFixture(ctx, st, now)
	require.NoError(t, err)

	assert.Len(t, fixture.Decisions, decisionCount)
	assert.Len(t, fixture.Policies.Entries, decisionCount)
	assert.Equal(t, "policy-0000", fixture.Policies.Entries[0].CorrelationID)
	assert.Equal(t, fmt.Sprintf("policy-%04d", decisionCount-1), fixture.Decisions[0].CorrelationID)
}

func TestMarshalReplayFixture_ProducesStableIndentedJSON(t *testing.T) {
	now := time.Date(2026, 3, 21, 10, 10, 0, 0, time.UTC)
	fixture := loomruntime.ReplayFixture{
		SessionID:  "story-fixture",
		CapturedAt: now,
		Checkpoint: store.Checkpoint{StoryID: "story-fixture", State: "AWAITING_CI", Phase: 2},
		Observations: loomruntime.ObservationModel{
			SessionID: "story-fixture",
			Checkpoint: loomruntime.CheckpointObservation{
				Authority: loomruntime.ObservationAuthorityAuthoritative,
				State:     "AWAITING_CI",
			},
		},
		Policies: loomruntime.PolicyAuditReport{
			SessionID: "story-fixture",
			Entries: []loomruntime.PolicyAuditEntry{{
				SessionID:    "story-fixture",
				DecisionKind: "ci_readiness",
				Verdict:      "continue",
			}},
		},
	}

	data, err := loomruntime.MarshalReplayFixture(fixture)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Contains(t, string(data), "\n  \"session_id\"")
	assert.Contains(t, string(data), "\"checkpoint\"")
	assert.Contains(t, string(data), "\"observations\"")
	assert.Contains(t, string(data), "\"policies\"")
}

func TestAssembleReplayFixture_DoesNotRequirePromptHistory(t *testing.T) {
	ctx := context.Background()
	st := newMemStore()
	now := time.Date(2026, 3, 21, 10, 15, 0, 0, time.UTC)

	require.NoError(t, st.WriteCheckpoint(ctx, store.Checkpoint{
		StoryID:   "story-no-prompt",
		State:     "REVIEWING",
		Phase:     2,
		PRNumber:  77,
		UpdatedAt: now,
	}))
	require.NoError(t, st.WriteExternalEvent(ctx, store.ExternalEvent{
		SessionID:     "story-no-prompt",
		EventSource:   "github",
		EventKind:     "pull_request_review",
		CorrelationID: "review-1",
		Payload:       `{"session_id":"story-no-prompt","correlation_id":"review-1","event_type":"pull_request_review","repository":"guillaume7/loom","pr_number":77,"payload":{"review":{"state":"approved"},"pull_request":{"head":{"ref":"feature/replay","sha":"abc123"},"base":{"ref":"main"}}}}`,
		ObservedAt:    now.Add(-time.Minute),
	}))

	decisionDetail, err := json.Marshal(map[string]any{
		"session_id":       "story-no-prompt",
		"correlation_id":   "decision-review-1",
		"wake_kind":        "poll_review",
		"policy_decision":  "review_readiness",
		"policy_outcome":   "continue",
		"policy_reason":    "review_approved",
		"previous_state":   "REVIEWING",
		"new_state":        "REVIEWING",
		"decision_verdict": "continue",
		"pr_number":        77,
		"review_status":    "APPROVED",
	})
	require.NoError(t, err)
	require.NoError(t, st.WritePolicyDecision(ctx, store.PolicyDecision{
		SessionID:     "story-no-prompt",
		CorrelationID: "decision-review-1",
		DecisionKind:  "review_readiness",
		Verdict:       "continue",
		InputHash:     "hash-review-1",
		Detail:        string(decisionDetail),
		CreatedAt:     now,
	}))
	require.NoError(t, st.WriteAction(ctx, store.Action{
		SessionID:    "story-no-prompt",
		OperationKey: "op-review-1",
		StateBefore:  "REVIEWING",
		StateAfter:   "REVIEWING",
		Event:        "runtime.review",
		Detail:       "approved",
		CreatedAt:    now,
	}))

	fixture, err := loomruntime.AssembleReplayFixture(ctx, st, now)
	require.NoError(t, err)

	assert.Equal(t, "story-no-prompt", fixture.SessionID)
	assert.NotEmpty(t, fixture.Events)
	assert.NotEmpty(t, fixture.Decisions)
	assert.NotEmpty(t, fixture.Actions)
	assert.Equal(t, "REVIEWING", fixture.Checkpoint.State)

	data, err := loomruntime.MarshalReplayFixture(fixture)
	require.NoError(t, err)
	assert.NotContains(t, strings.ToLower(string(data)), "prompt_history")
	assert.Contains(t, string(data), "story-no-prompt")
	assert.Contains(t, string(data), "review_readiness")
}
