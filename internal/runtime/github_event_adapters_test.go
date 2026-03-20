package runtime_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	loomruntime "github.com/guillaume7/loom/internal/runtime"
	"github.com/guillaume7/loom/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestControllerObserveGitHubEvent_SchedulesMatchingWakeWithoutMutatingCheckpoint(t *testing.T) {
	now := time.Date(2026, 3, 20, 15, 0, 0, 0, time.UTC)
	st := newMemStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "AWAITING_CI", Phase: 2, PRNumber: 42}))
	require.NoError(t, st.UpsertWakeSchedule(context.Background(), store.WakeSchedule{
		SessionID: "default",
		WakeKind:  "poll_ci",
		DueAt:     now.Add(time.Minute),
		DedupeKey: "run:default:poll_ci",
	}))

	controller := loomruntime.NewController(st, loomruntime.Config{Now: func() time.Time { return now }})
	receipt, err := controller.ObserveGitHubEvent(context.Background(), loomruntime.GitHubEvent{
		DeliveryID: "evt-1",
		EventType:  "check_suite",
		PRNumber:   42,
		Payload:    json.RawMessage(`{"check_suite":{"conclusion":"success"}}`),
	})
	require.NoError(t, err)

	assert.Equal(t, "ci", receipt.Adapter)
	assert.True(t, receipt.WakeScheduled)
	assert.Equal(t, "poll_ci", receipt.WakeKind)
	assert.Equal(t, "poll_ci", receipt.FallbackWakeKind)
	assert.Equal(t, now, receipt.WakeAt)

	cp, err := st.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "AWAITING_CI", cp.State)
	assert.Equal(t, 42, cp.PRNumber)

	require.Len(t, st.events, 1)
	assert.Equal(t, "github", st.events[0].EventSource)
	assert.Equal(t, "check_suite", st.events[0].EventKind)

	wakes, err := st.ReadWakeSchedules(context.Background(), "default", 10)
	require.NoError(t, err)
	require.Len(t, wakes, 1)
	assert.Equal(t, "poll_ci", wakes[0].WakeKind)
	assert.Equal(t, now, wakes[0].DueAt)
	assert.Equal(t, "run:default:poll_ci", wakes[0].DedupeKey)
	assert.NotEmpty(t, wakes[0].Payload)
}

func TestControllerObserveGitHubEvent_StoresReplayableObservationPayload(t *testing.T) {
	now := time.Date(2026, 3, 20, 15, 1, 0, 0, time.UTC)
	st := newMemStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "REVIEWING", Phase: 2, PRNumber: 42}))

	controller := loomruntime.NewController(st, loomruntime.Config{Now: func() time.Time { return now }})
	_, err := controller.ObserveGitHubEvent(context.Background(), loomruntime.GitHubEvent{
		DeliveryID: "evt-2",
		EventType:  "pull_request_review",
		Action:     "submitted",
		Repository: "guillaume7/loom",
		PRNumber:   42,
		Payload:    json.RawMessage(`{"review":{"state":"approved"},"sender":{"login":"octocat"}}`),
	})
	require.NoError(t, err)

	require.Len(t, st.events, 1)
	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(st.events[0].Payload), &payload))
	assert.Equal(t, "pull_request_review", payload["event_type"])
	assert.Equal(t, "review", payload["adapter"])
	assert.Equal(t, "REVIEWING", payload["workflow_state"])

	rawPayload, ok := payload["payload"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "approved", rawPayload["review"].(map[string]any)["state"])
	assert.Equal(t, "octocat", rawPayload["sender"].(map[string]any)["login"])
}

func TestControllerObserveGitHubEvent_PreservesClaimedWakeOnDedupeUpdate(t *testing.T) {
	now := time.Date(2026, 3, 20, 15, 1, 30, 0, time.UTC)
	claimedAt := now.Add(-20 * time.Second)
	createdAt := now.Add(-2 * time.Minute)
	st := newMemStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "AWAITING_CI", Phase: 2, PRNumber: 42}))
	require.NoError(t, st.UpsertWakeSchedule(context.Background(), store.WakeSchedule{
		SessionID: "default",
		WakeKind:  "poll_ci",
		DueAt:     now.Add(time.Minute),
		DedupeKey: "run:default:poll_ci",
		Payload:   `{"attempt":1}`,
		ClaimedAt: claimedAt,
		CreatedAt: createdAt,
	}))

	controller := loomruntime.NewController(st, loomruntime.Config{Now: func() time.Time { return now }})
	receipt, err := controller.ObserveGitHubEvent(context.Background(), loomruntime.GitHubEvent{
		DeliveryID: "evt-claimed",
		EventType:  "check_suite",
		PRNumber:   42,
		Payload:    json.RawMessage(`{"check_suite":{"conclusion":"success"}}`),
	})
	require.NoError(t, err)
	assert.True(t, receipt.WakeScheduled)

	wakes, err := st.ReadWakeSchedules(context.Background(), "default", 10)
	require.NoError(t, err)
	require.Len(t, wakes, 1)
	assert.Equal(t, now, wakes[0].DueAt)
	assert.Equal(t, claimedAt, wakes[0].ClaimedAt)
	assert.Equal(t, createdAt, wakes[0].CreatedAt)
	assert.Contains(t, wakes[0].Payload, "success")
}

func TestControllerObserveGitHubEvent_ReturnsMissingEventType(t *testing.T) {
	st := newMemStore()
	controller := loomruntime.NewController(st, loomruntime.Config{Now: time.Now})

	_, err := controller.ObserveGitHubEvent(context.Background(), loomruntime.GitHubEvent{})
	require.ErrorIs(t, err, loomruntime.ErrMissingGitHubEventType)
	assert.Empty(t, st.events)
	assert.Empty(t, st.wakes)
}

func TestControllerObserveGitHubEvent_ReturnsWakeStoreError(t *testing.T) {
	now := time.Date(2026, 3, 20, 15, 1, 45, 0, time.UTC)
	st := newMemStore()
	st.wakeErr = errors.New("wake store unavailable")
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "AWAITING_CI", Phase: 2, PRNumber: 42}))

	controller := loomruntime.NewController(st, loomruntime.Config{Now: func() time.Time { return now }})
	_, err := controller.ObserveGitHubEvent(context.Background(), loomruntime.GitHubEvent{
		DeliveryID: "evt-wake-error",
		EventType:  "check_suite",
		PRNumber:   42,
	})
	require.EqualError(t, err, "wake store unavailable")
	require.Len(t, st.events, 1)
	assert.Equal(t, "check_suite", st.events[0].EventKind)
}

func TestControllerObserveGitHubEvent_FallsBackToPollingWhenEventDoesNotMatch(t *testing.T) {
	now := time.Date(2026, 3, 20, 15, 2, 0, 0, time.UTC)
	st := newMemStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "AWAITING_CI", Phase: 2, PRNumber: 42}))

	controller := loomruntime.NewController(st, loomruntime.Config{
		HolderID:     "controller-1",
		LeaseTTL:     time.Minute,
		PollInterval: 30 * time.Second,
		Now:          func() time.Time { return now },
	})

	receipt, err := controller.ObserveGitHubEvent(context.Background(), loomruntime.GitHubEvent{
		DeliveryID: "evt-3",
		EventType:  "check_suite",
		PRNumber:   7,
		Payload:    json.RawMessage(`{"check_suite":{"conclusion":"success"}}`),
	})
	require.NoError(t, err)
	assert.False(t, receipt.WakeScheduled)
	assert.Equal(t, "poll_ci", receipt.FallbackWakeKind)
	require.Len(t, st.events, 1)
	assert.Empty(t, st.wakes)

	lifecycle, err := controller.Start(context.Background())
	require.NoError(t, err)
	assert.Equal(t, loomruntime.ControllerStateSleeping, lifecycle.Controller)
	assert.Equal(t, "poll_ci", lifecycle.NextWakeKind)
	assert.Equal(t, now.Add(30*time.Second), lifecycle.NextWakeAt)

	wakes, err := st.ReadWakeSchedules(context.Background(), "default", 10)
	require.NoError(t, err)
	require.Len(t, wakes, 1)
	assert.Equal(t, "poll_ci", wakes[0].WakeKind)
	assert.Equal(t, now.Add(30*time.Second), wakes[0].DueAt)
}
