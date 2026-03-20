package runtime_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	loomgh "github.com/guillaume7/loom/internal/github"
	loomruntime "github.com/guillaume7/loom/internal/runtime"
	"github.com/guillaume7/loom/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type pollingGitHubClientMock struct {
	pr                 *loomgh.PR
	checkRuns          []*loomgh.CheckRun
	reviewStatus       string
	markReadyCalls     int
	requestReviewCalls int
	lastRequestedPR    int
	lastReviewer       string
}

func (m *pollingGitHubClientMock) GetPR(_ context.Context, prNumber int) (*loomgh.PR, error) {
	if m.pr == nil {
		m.pr = &loomgh.PR{Number: prNumber}
	}
	return m.pr, nil
}

func (m *pollingGitHubClientMock) GetCheckRuns(_ context.Context, _ string) ([]*loomgh.CheckRun, error) {
	return m.checkRuns, nil
}

func (m *pollingGitHubClientMock) GetReviewStatus(_ context.Context, _ int) (string, error) {
	return m.reviewStatus, nil
}

func (m *pollingGitHubClientMock) MarkReadyForReview(_ context.Context, _ int) error {
	m.markReadyCalls++
	return nil
}

func (m *pollingGitHubClientMock) RequestReview(_ context.Context, prNumber int, reviewer string) error {
	m.requestReviewCalls++
	m.lastRequestedPR = prNumber
	m.lastReviewer = reviewer
	return nil
}

type memStore struct {
	cp                  store.Checkpoint
	actions             []store.Action
	wakes               []store.WakeSchedule
	events              []store.ExternalEvent
	decisions           []store.PolicyDecision
	runtimeLeases       map[string]store.RuntimeLease
	checkpointErr       error
	checkpointActionErr error
	externalEventErr    error
	policyDecisionErr   error
	wakeErr             error
	leaseErr            error
	empty               bool
}

func newMemStore() *memStore {
	return &memStore{empty: true, runtimeLeases: make(map[string]store.RuntimeLease)}
}

func (s *memStore) ReadCheckpoint(_ context.Context) (store.Checkpoint, error) {
	if s.empty {
		return store.Checkpoint{}, nil
	}
	return s.cp, nil
}

func (s *memStore) WriteCheckpoint(_ context.Context, cp store.Checkpoint) error {
	if s.checkpointErr != nil {
		return s.checkpointErr
	}
	s.cp = cp
	s.empty = false
	return nil
}

func (s *memStore) WriteAction(_ context.Context, action store.Action) error {
	action.ID = int64(len(s.actions) + 1)
	s.actions = append(s.actions, action)
	return nil
}

func (s *memStore) WriteCheckpointAndAction(_ context.Context, cp store.Checkpoint, action store.Action) error {
	if s.checkpointActionErr != nil {
		return s.checkpointActionErr
	}
	s.cp = cp
	s.empty = false
	action.ID = int64(len(s.actions) + 1)
	s.actions = append(s.actions, action)
	return nil
}

func (s *memStore) ReadActionByOperationKey(_ context.Context, _ string) (store.Action, error) {
	return store.Action{}, store.ErrActionNotFound
}

func (s *memStore) ReadActions(_ context.Context, _ int) ([]store.Action, error) {
	result := make([]store.Action, 0, len(s.actions))
	for index := len(s.actions) - 1; index >= 0; index-- {
		result = append(result, s.actions[index])
	}
	return result, nil
}

func (s *memStore) UpsertWakeSchedule(_ context.Context, wake store.WakeSchedule) error {
	if s.wakeErr != nil {
		return s.wakeErr
	}
	for index, existing := range s.wakes {
		if existing.DedupeKey == wake.DedupeKey {
			s.wakes[index] = wake
			return nil
		}
	}
	wake.ID = int64(len(s.wakes) + 1)
	s.wakes = append(s.wakes, wake)
	return nil
}

func (s *memStore) ReadWakeSchedules(_ context.Context, sessionID string, _ int) ([]store.WakeSchedule, error) {
	result := make([]store.WakeSchedule, 0, len(s.wakes))
	for _, wake := range s.wakes {
		if sessionID != "" && wake.SessionID != sessionID {
			continue
		}
		result = append(result, wake)
	}
	return result, nil
}

func (s *memStore) WriteExternalEvent(_ context.Context, event store.ExternalEvent) error {
	if s.externalEventErr != nil {
		return s.externalEventErr
	}
	event.ID = int64(len(s.events) + 1)
	s.events = append(s.events, event)
	return nil
}

func (s *memStore) ReadExternalEvents(_ context.Context, _ string, _ int) ([]store.ExternalEvent, error) {
	result := make([]store.ExternalEvent, 0, len(s.events))
	for index := len(s.events) - 1; index >= 0; index-- {
		result = append(result, s.events[index])
	}
	return result, nil
}

func (s *memStore) UpsertRuntimeLease(_ context.Context, lease store.RuntimeLease) error {
	if s.leaseErr != nil {
		return s.leaseErr
	}
	s.runtimeLeases[lease.LeaseKey] = lease
	return nil
}

func (s *memStore) ReadRuntimeLease(_ context.Context, leaseKey string) (store.RuntimeLease, error) {
	lease, ok := s.runtimeLeases[leaseKey]
	if !ok {
		return store.RuntimeLease{}, store.ErrRuntimeLeaseNotFound
	}
	return lease, nil
}

func (s *memStore) WritePolicyDecision(_ context.Context, decision store.PolicyDecision) error {
	if s.policyDecisionErr != nil {
		return s.policyDecisionErr
	}
	decision.ID = int64(len(s.decisions) + 1)
	s.decisions = append(s.decisions, decision)
	return nil
}

func (s *memStore) ReadPolicyDecisions(_ context.Context, _ string, _ int) ([]store.PolicyDecision, error) {
	result := make([]store.PolicyDecision, 0, len(s.decisions))
	for index := len(s.decisions) - 1; index >= 0; index-- {
		result = append(result, s.decisions[index])
	}
	return result, nil
}

func (s *memStore) DeleteAll(_ context.Context) error {
	s.cp = store.Checkpoint{}
	s.actions = nil
	s.wakes = nil
	s.events = nil
	s.decisions = nil
	s.runtimeLeases = make(map[string]store.RuntimeLease)
	s.empty = true
	return nil
}

func (s *memStore) Close() error { return nil }

func TestControllerStartClaimsDuePersistedWork(t *testing.T) {
	now := time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)
	st := newMemStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "AWAITING_CI", Phase: 2}))
	require.NoError(t, st.UpsertWakeSchedule(context.Background(), store.WakeSchedule{
		SessionID: "default",
		WakeKind:  "poll_ci",
		DueAt:     now.Add(-time.Minute),
		DedupeKey: "run:default:poll_ci",
	}))

	controller := loomruntime.NewController(st, loomruntime.Config{
		HolderID:     "controller-1",
		LeaseTTL:     time.Minute,
		PollInterval: time.Minute,
		Now:          func() time.Time { return now },
	})

	lifecycle, err := controller.Start(context.Background())
	require.NoError(t, err)

	assert.Equal(t, loomruntime.ControllerStateWakeDue, lifecycle.Controller)
	assert.Equal(t, "persisted_runtime_state", lifecycle.DrivenBy)
	assert.Equal(t, "controller-1", lifecycle.HolderID)
	assert.Equal(t, "run:default", lifecycle.LeaseKey)
	assert.Equal(t, "poll_ci", lifecycle.NextWakeKind)

	lease, err := st.ReadRuntimeLease(context.Background(), "run:default")
	require.NoError(t, err)
	assert.Equal(t, "controller-1", lease.HolderID)

	wakes, err := st.ReadWakeSchedules(context.Background(), "default", 10)
	require.NoError(t, err)
	require.Len(t, wakes, 1)
	assert.Equal(t, now, wakes[0].ClaimedAt)
}

func TestControllerStartReclaimsDueWakeAfterExpiredLease(t *testing.T) {
	initialClaimAt := time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)
	restartAt := initialClaimAt.Add(2 * time.Minute)
	st := newMemStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "AWAITING_CI", Phase: 2}))
	require.NoError(t, st.UpsertWakeSchedule(context.Background(), store.WakeSchedule{
		SessionID: "default",
		WakeKind:  "poll_ci",
		DueAt:     initialClaimAt.Add(-time.Minute),
		DedupeKey: "run:default:poll_ci",
		ClaimedAt: initialClaimAt,
	}))
	require.NoError(t, st.UpsertRuntimeLease(context.Background(), store.RuntimeLease{
		LeaseKey:  "run:default",
		HolderID:  "controller-1",
		Scope:     "run",
		ExpiresAt: initialClaimAt.Add(time.Minute),
		CreatedAt: initialClaimAt,
		RenewedAt: initialClaimAt,
	}))

	controller := loomruntime.NewController(st, loomruntime.Config{
		HolderID:     "controller-2",
		LeaseTTL:     time.Minute,
		PollInterval: time.Minute,
		Now:          func() time.Time { return restartAt },
	})

	lifecycle, err := controller.Start(context.Background())
	require.NoError(t, err)

	assert.Equal(t, loomruntime.ControllerStateWakeDue, lifecycle.Controller)
	assert.Equal(t, "persisted_wake_due", lifecycle.Reason)
	assert.Equal(t, "controller-2", lifecycle.HolderID)

	lease, err := st.ReadRuntimeLease(context.Background(), "run:default")
	require.NoError(t, err)
	assert.Equal(t, "controller-2", lease.HolderID)
	assert.Equal(t, restartAt.Add(time.Minute), lease.ExpiresAt)

	wakes, err := st.ReadWakeSchedules(context.Background(), "default", 10)
	require.NoError(t, err)
	require.Len(t, wakes, 1)
	assert.Equal(t, restartAt, wakes[0].ClaimedAt)
}

func TestControllerStartSleepsUntilNextPersistedWake(t *testing.T) {
	now := time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)
	st := newMemStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "AWAITING_CI", Phase: 2}))

	controller := loomruntime.NewController(st, loomruntime.Config{
		HolderID:     "controller-1",
		LeaseTTL:     time.Minute,
		PollInterval: 45 * time.Second,
		Now:          func() time.Time { return now },
	})

	lifecycle, err := controller.Start(context.Background())
	require.NoError(t, err)

	assert.Equal(t, loomruntime.ControllerStateSleeping, lifecycle.Controller)
	assert.Equal(t, "poll_ci", lifecycle.NextWakeKind)
	assert.Equal(t, now.Add(45*time.Second), lifecycle.NextWakeAt)
	assert.Equal(t, "persisted_runtime_state", lifecycle.DrivenBy)

	wakes, err := st.ReadWakeSchedules(context.Background(), "default", 10)
	require.NoError(t, err)
	require.Len(t, wakes, 1)
	assert.Equal(t, "poll_ci", wakes[0].WakeKind)
	assert.Equal(t, now.Add(45*time.Second), wakes[0].DueAt)
}

func TestControllerPendingWakesReturnsPersistedQueueForCurrentRun(t *testing.T) {
	now := time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)
	st := newMemStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "AWAITING_CI", Phase: 2, StoryID: "TH3.E2.US1"}))
	require.NoError(t, st.UpsertWakeSchedule(context.Background(), store.WakeSchedule{
		SessionID: "TH3.E2.US1",
		WakeKind:  "poll_ci",
		DueAt:     now.Add(time.Minute),
		DedupeKey: "run:TH3.E2.US1:poll_ci",
	}))
	require.NoError(t, st.UpsertWakeSchedule(context.Background(), store.WakeSchedule{
		SessionID: "TH3.E2.US1",
		WakeKind:  "poll_review",
		DueAt:     now.Add(2 * time.Minute),
		DedupeKey: "run:TH3.E2.US1:poll_review",
	}))
	require.NoError(t, st.UpsertWakeSchedule(context.Background(), store.WakeSchedule{
		SessionID: "other-story",
		WakeKind:  "poll_ci",
		DueAt:     now.Add(3 * time.Minute),
		DedupeKey: "run:other-story:poll_ci",
	}))

	controller := loomruntime.NewController(st, loomruntime.Config{Now: func() time.Time { return now }})
	wakes, err := controller.PendingWakes(context.Background())
	require.NoError(t, err)
	require.Len(t, wakes, 2)
	assert.Equal(t, "poll_ci", wakes[0].WakeKind)
	assert.Equal(t, "run:TH3.E2.US1:poll_ci", wakes[0].DedupeKey)
	assert.Equal(t, "poll_review", wakes[1].WakeKind)
	assert.Equal(t, "run:TH3.E2.US1:poll_review", wakes[1].DedupeKey)
}

func TestControllerSnapshotExposesActiveControllerProgress(t *testing.T) {
	now := time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)
	st := newMemStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "SCANNING", Phase: 1}))
	require.NoError(t, st.UpsertRuntimeLease(context.Background(), store.RuntimeLease{
		LeaseKey:  "run:default",
		HolderID:  "controller-1",
		Scope:     "run",
		ExpiresAt: now.Add(time.Minute),
		CreatedAt: now.Add(-time.Minute),
		RenewedAt: now,
	}))

	controller := loomruntime.NewController(st, loomruntime.Config{
		HolderID: "controller-1",
		Now:      func() time.Time { return now },
	})

	lifecycle, err := controller.Snapshot(context.Background())
	require.NoError(t, err)

	assert.Equal(t, loomruntime.ControllerStateClaimed, lifecycle.Controller)
	assert.Equal(t, "active_lease_present", lifecycle.Reason)
	assert.Equal(t, "controller-1", lifecycle.HolderID)
	assert.Equal(t, "run:default", lifecycle.LeaseKey)
	assert.Equal(t, now.Add(time.Minute), lifecycle.LeaseExpires)
	assert.Equal(t, "SCANNING", lifecycle.WorkflowState)
}

func TestControllerPauseOverridePreservesRecoverableStateAndAuditsIntent(t *testing.T) {
	now := time.Date(2026, 3, 20, 13, 0, 0, 0, time.UTC)
	st := newMemStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "AWAITING_CI", Phase: 2, PRNumber: 42}))
	require.NoError(t, st.UpsertWakeSchedule(context.Background(), store.WakeSchedule{
		SessionID: "default",
		WakeKind:  "poll_ci",
		DueAt:     now.Add(time.Minute),
		DedupeKey: "run:default:poll_ci",
	}))

	controller := loomruntime.NewController(st, loomruntime.Config{Now: func() time.Time { return now }})
	lifecycle, err := controller.ApplyManualOverride(context.Background(), loomruntime.ManualOverrideRequest{
		Action:      loomruntime.ManualOverridePause,
		Source:      "cli",
		RequestedBy: "test",
		Reason:      "operator requested pause",
	})
	require.NoError(t, err)

	assert.Equal(t, loomruntime.ControllerStatePaused, lifecycle.Controller)
	assert.Equal(t, "AWAITING_CI", lifecycle.ResumeState)
	assert.Len(t, st.wakes, 1)
	require.Len(t, st.actions, 1)
	assert.Equal(t, "manual_override_pause", st.actions[0].Event)
	require.Len(t, st.events, 1)
	assert.Equal(t, "operator", st.events[0].EventSource)
	assert.Equal(t, "manual_override.pause", st.events[0].EventKind)
	require.Len(t, st.decisions, 1)
	assert.Equal(t, "operator_override", st.decisions[0].DecisionKind)
	assert.Equal(t, "pause", st.decisions[0].Verdict)
	assert.Equal(t, st.events[0].CorrelationID, st.decisions[0].CorrelationID)

	cp, err := st.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "PAUSED", cp.State)
	assert.Equal(t, "AWAITING_CI", cp.ResumeState)
}

func TestControllerPauseOverrideRejectsMissingRecoverableState(t *testing.T) {
	now := time.Date(2026, 3, 20, 13, 5, 0, 0, time.UTC)
	st := newMemStore()
	controller := loomruntime.NewController(st, loomruntime.Config{Now: func() time.Time { return now }})

	_, err := controller.ApplyManualOverride(context.Background(), loomruntime.ManualOverrideRequest{
		Action:      loomruntime.ManualOverridePause,
		Source:      "cli",
		RequestedBy: "test",
		Reason:      "operator requested pause",
	})
	require.ErrorIs(t, err, loomruntime.ErrNothingToPause)
	assert.Empty(t, st.actions)
	assert.Empty(t, st.events)
	assert.Empty(t, st.decisions)
	assert.Empty(t, st.wakes)

	cp, readErr := st.ReadCheckpoint(context.Background())
	require.NoError(t, readErr)
	assert.Equal(t, store.Checkpoint{}, cp)
}

func TestControllerResumeOverrideReusesPauseCorrelation(t *testing.T) {
	now := time.Date(2026, 3, 20, 13, 10, 0, 0, time.UTC)
	st := newMemStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "PAUSED", ResumeState: "AWAITING_CI", Phase: 2, PRNumber: 42}))
	require.NoError(t, st.WritePolicyDecision(context.Background(), store.PolicyDecision{
		SessionID:     "default",
		CorrelationID: "corr-1",
		DecisionKind:  "operator_override",
		Verdict:       "pause",
		InputHash:     "pause-corr-1",
		CreatedAt:     now.Add(-time.Minute),
	}))

	controller := loomruntime.NewController(st, loomruntime.Config{Now: func() time.Time { return now }})
	lifecycle, err := controller.ApplyManualOverride(context.Background(), loomruntime.ManualOverrideRequest{
		Action:      loomruntime.ManualOverrideResume,
		Source:      "cli",
		RequestedBy: "test",
		Reason:      "operator requested resume",
	})
	require.NoError(t, err)

	assert.Equal(t, "AWAITING_CI", lifecycle.WorkflowState)
	require.Len(t, st.events, 1)
	assert.Equal(t, "corr-1", st.events[0].CorrelationID)
	require.Len(t, st.decisions, 2)
	assert.Equal(t, "resume", st.decisions[1].Verdict)
	assert.Equal(t, "corr-1", st.decisions[1].CorrelationID)

	cp, err := st.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "AWAITING_CI", cp.State)
	assert.Equal(t, "", cp.ResumeState)
}

func TestControllerResumeOverrideLeavesPausedCheckpointRecoverableWhenControllerStartFails(t *testing.T) {
	now := time.Date(2026, 3, 20, 13, 20, 0, 0, time.UTC)
	st := newMemStore()
	st.wakeErr = errors.New("wake store unavailable")
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "PAUSED", ResumeState: "AWAITING_CI", Phase: 2, PRNumber: 42}))

	controller := loomruntime.NewController(st, loomruntime.Config{
		HolderID:     "controller-1",
		PollInterval: time.Minute,
		Now:          func() time.Time { return now },
	})

	_, err := controller.ApplyManualOverride(context.Background(), loomruntime.ManualOverrideRequest{
		Action:        loomruntime.ManualOverrideResume,
		Source:        "cli",
		RequestedBy:   "test",
		Reason:        "operator requested resume",
		CorrelationID: "corr-1",
	})
	require.ErrorContains(t, err, "wake store unavailable")
	assert.Empty(t, st.actions)
	assert.Empty(t, st.events)
	assert.Empty(t, st.decisions)

	cp, readErr := st.ReadCheckpoint(context.Background())
	require.NoError(t, readErr)
	assert.Equal(t, "PAUSED", cp.State)
	assert.Equal(t, "AWAITING_CI", cp.ResumeState)

	lease, leaseErr := st.ReadRuntimeLease(context.Background(), "run:default")
	require.NoError(t, leaseErr)
	assert.Equal(t, "controller-1", lease.HolderID)
	assert.Equal(t, now, lease.ExpiresAt)
	assert.Equal(t, now, lease.RenewedAt)
}

func TestControllerProcessDueWake_PRReadyUsesPersistedCheckpointState(t *testing.T) {
	now := time.Date(2026, 3, 20, 14, 0, 0, 0, time.UTC)
	st := newMemStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "AWAITING_READY", Phase: 2, PRNumber: 42}))
	require.NoError(t, st.UpsertWakeSchedule(context.Background(), store.WakeSchedule{
		SessionID: "default",
		WakeKind:  "poll_pr_ready",
		DueAt:     now.Add(-time.Minute),
		DedupeKey: "run:default:poll_pr_ready",
	}))

	controller := loomruntime.NewController(st, loomruntime.Config{
		HolderID:     "controller-1",
		LeaseTTL:     time.Minute,
		PollInterval: 30 * time.Second,
		Now:          func() time.Time { return now },
	})

	gh := &pollingGitHubClientMock{pr: &loomgh.PR{Number: 42, Draft: false}}
	lifecycle, err := controller.ProcessDueWake(context.Background(), gh)
	require.NoError(t, err)

	assert.Equal(t, "AWAITING_CI", lifecycle.WorkflowState)
	assert.Equal(t, loomruntime.ControllerStateClaimed, lifecycle.Controller)
	assert.Equal(t, "poll_ci", lifecycle.NextWakeKind)
	assert.Equal(t, now.Add(30*time.Second), lifecycle.NextWakeAt)
	assert.Equal(t, "poll_observation", lifecycle.DrivenBy)
	assert.Equal(t, 0, gh.markReadyCalls)

	cp, err := st.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "AWAITING_CI", cp.State)
	assert.Zero(t, cp.RetryCount)

	require.Len(t, st.actions, 1)
	assert.Equal(t, "pr_ready", st.actions[0].Event)
	require.Len(t, st.events, 1)
	assert.Equal(t, "poll_pr_ready", st.events[0].EventKind)
	require.Len(t, st.decisions, 1)
	assert.Equal(t, "resume", st.decisions[0].Verdict)
}

func TestControllerProcessDueWake_CIPollWritesObservationAndSchedulesReviewPolling(t *testing.T) {
	now := time.Date(2026, 3, 20, 14, 5, 0, 0, time.UTC)
	st := newMemStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "AWAITING_CI", Phase: 2, PRNumber: 42}))
	require.NoError(t, st.UpsertWakeSchedule(context.Background(), store.WakeSchedule{
		SessionID: "default",
		WakeKind:  "poll_ci",
		DueAt:     now.Add(-time.Minute),
		DedupeKey: "run:default:poll_ci",
	}))

	controller := loomruntime.NewController(st, loomruntime.Config{
		HolderID:     "controller-1",
		LeaseTTL:     time.Minute,
		PollInterval: 45 * time.Second,
		Now:          func() time.Time { return now },
	})

	gh := &pollingGitHubClientMock{
		pr: &loomgh.PR{Number: 42, HeadSHA: "abc123"},
		checkRuns: []*loomgh.CheckRun{
			{Name: "build", Status: "completed", Conclusion: "success"},
			{Name: "lint", Status: "completed", Conclusion: "success"},
		},
	}
	lifecycle, err := controller.ProcessDueWake(context.Background(), gh)
	require.NoError(t, err)

	assert.Equal(t, "REVIEWING", lifecycle.WorkflowState)
	assert.Equal(t, loomruntime.ControllerStateClaimed, lifecycle.Controller)
	assert.Equal(t, "poll_review", lifecycle.NextWakeKind)
	assert.Equal(t, now.Add(45*time.Second), lifecycle.NextWakeAt)
	assert.Equal(t, "poll_observation", lifecycle.DrivenBy)
	assert.Zero(t, gh.requestReviewCalls)

	cp, err := st.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "REVIEWING", cp.State)

	require.Len(t, st.events, 1)
	assert.Equal(t, "poll_ci", st.events[0].EventKind)

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(st.events[0].Payload), &payload))
	assert.Equal(t, "ci_green", payload["outcome"])
	assert.Equal(t, "resume", payload["decision_verdict"])

	require.Len(t, st.decisions, 1)
	assert.Equal(t, "runtime_poll", st.decisions[0].DecisionKind)
	assert.Equal(t, "resume", st.decisions[0].Verdict)
}

func TestControllerProcessDueWake_CIGreenDoesNotMutateGitHubBeforePersistence(t *testing.T) {
	now := time.Date(2026, 3, 20, 14, 7, 0, 0, time.UTC)
	st := newMemStore()
	st.checkpointActionErr = errors.New("checkpoint write failed")
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "AWAITING_CI", Phase: 2, PRNumber: 42}))
	require.NoError(t, st.UpsertWakeSchedule(context.Background(), store.WakeSchedule{
		SessionID: "default",
		WakeKind:  "poll_ci",
		DueAt:     now.Add(-time.Minute),
		DedupeKey: "run:default:poll_ci",
	}))

	controller := loomruntime.NewController(st, loomruntime.Config{
		HolderID:     "controller-1",
		LeaseTTL:     time.Minute,
		PollInterval: 45 * time.Second,
		Now:          func() time.Time { return now },
	})

	gh := &pollingGitHubClientMock{
		pr:        &loomgh.PR{Number: 42, HeadSHA: "abc123"},
		checkRuns: []*loomgh.CheckRun{{Name: "build", Status: "completed", Conclusion: "success"}},
	}

	_, err := controller.ProcessDueWake(context.Background(), gh)
	require.ErrorContains(t, err, "checkpoint write failed")
	assert.Zero(t, gh.requestReviewCalls)

	cp, readErr := st.ReadCheckpoint(context.Background())
	require.NoError(t, readErr)
	assert.Equal(t, "AWAITING_CI", cp.State)
	assert.Empty(t, st.actions)
	assert.Empty(t, st.events)
	assert.Empty(t, st.decisions)
}

func TestControllerProcessDueWake_CIFailedTransitionsToDebugging(t *testing.T) {
	now := time.Date(2026, 3, 20, 14, 8, 0, 0, time.UTC)
	st := newMemStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "AWAITING_CI", Phase: 2, PRNumber: 42}))
	require.NoError(t, st.UpsertWakeSchedule(context.Background(), store.WakeSchedule{
		SessionID: "default",
		WakeKind:  "poll_ci",
		DueAt:     now.Add(-time.Minute),
		DedupeKey: "run:default:poll_ci",
	}))

	controller := loomruntime.NewController(st, loomruntime.Config{
		HolderID:     "controller-1",
		LeaseTTL:     time.Minute,
		PollInterval: 45 * time.Second,
		Now:          func() time.Time { return now },
	})

	gh := &pollingGitHubClientMock{
		pr:        &loomgh.PR{Number: 42, HeadSHA: "abc123"},
		checkRuns: []*loomgh.CheckRun{{Name: "build", Status: "completed", Conclusion: "failure"}},
	}

	lifecycle, err := controller.ProcessDueWake(context.Background(), gh)
	require.NoError(t, err)
	assert.Equal(t, "DEBUGGING", lifecycle.WorkflowState)
	assert.Empty(t, lifecycle.NextWakeKind)
	assert.Zero(t, gh.requestReviewCalls)

	cp, readErr := st.ReadCheckpoint(context.Background())
	require.NoError(t, readErr)
	assert.Equal(t, "DEBUGGING", cp.State)
	require.Len(t, st.decisions, 1)
	assert.Equal(t, "debug", st.decisions[0].Verdict)
}

func TestControllerProcessDueWake_PRReadyRetryBudgetExhaustionPauses(t *testing.T) {
	now := time.Date(2026, 3, 20, 14, 9, 0, 0, time.UTC)
	st := newMemStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{
		State:      "AWAITING_READY",
		Phase:      2,
		PRNumber:   42,
		RetryCount: 60,
	}))
	require.NoError(t, st.UpsertWakeSchedule(context.Background(), store.WakeSchedule{
		SessionID: "default",
		WakeKind:  "poll_pr_ready",
		DueAt:     now.Add(-time.Minute),
		DedupeKey: "run:default:poll_pr_ready",
	}))

	controller := loomruntime.NewController(st, loomruntime.Config{
		HolderID:     "controller-1",
		LeaseTTL:     time.Minute,
		PollInterval: 30 * time.Second,
		Now:          func() time.Time { return now },
	})

	gh := &pollingGitHubClientMock{pr: &loomgh.PR{Number: 42, Draft: true}}
	lifecycle, err := controller.ProcessDueWake(context.Background(), gh)
	require.NoError(t, err)
	assert.Equal(t, "PAUSED", lifecycle.WorkflowState)
	assert.Equal(t, loomruntime.ControllerStatePaused, lifecycle.Controller)
	assert.Equal(t, "AWAITING_READY", lifecycle.ResumeState)
	assert.Zero(t, gh.markReadyCalls)

	cp, readErr := st.ReadCheckpoint(context.Background())
	require.NoError(t, readErr)
	assert.Equal(t, "PAUSED", cp.State)
	assert.Equal(t, "AWAITING_READY", cp.ResumeState)
	require.Len(t, st.decisions, 1)
	assert.Equal(t, "pause", st.decisions[0].Verdict)
	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(st.decisions[0].Detail), &payload))
	assert.Equal(t, "draft_retry_exhausted", payload["outcome"])
}

func TestControllerProcessDueWake_CIRetryBudgetExhaustionPauses(t *testing.T) {
	now := time.Date(2026, 3, 20, 14, 9, 30, 0, time.UTC)
	st := newMemStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{
		State:      "AWAITING_CI",
		Phase:      2,
		PRNumber:   42,
		RetryCount: 20,
	}))
	require.NoError(t, st.UpsertWakeSchedule(context.Background(), store.WakeSchedule{
		SessionID: "default",
		WakeKind:  "poll_ci",
		DueAt:     now.Add(-time.Minute),
		DedupeKey: "run:default:poll_ci",
	}))

	controller := loomruntime.NewController(st, loomruntime.Config{
		HolderID:     "controller-1",
		LeaseTTL:     time.Minute,
		PollInterval: 30 * time.Second,
		Now:          func() time.Time { return now },
	})

	gh := &pollingGitHubClientMock{
		pr:        &loomgh.PR{Number: 42, HeadSHA: "abc123"},
		checkRuns: []*loomgh.CheckRun{{Name: "build", Status: "in_progress"}},
	}
	lifecycle, err := controller.ProcessDueWake(context.Background(), gh)
	require.NoError(t, err)
	assert.Equal(t, "PAUSED", lifecycle.WorkflowState)
	assert.Equal(t, loomruntime.ControllerStatePaused, lifecycle.Controller)
	assert.Equal(t, "AWAITING_CI", lifecycle.ResumeState)

	cp, readErr := st.ReadCheckpoint(context.Background())
	require.NoError(t, readErr)
	assert.Equal(t, "PAUSED", cp.State)
	assert.Equal(t, "AWAITING_CI", cp.ResumeState)
	require.Len(t, st.decisions, 1)
	assert.Equal(t, "pause", st.decisions[0].Verdict)
}

func TestControllerProcessDueWake_ReviewApprovedTransitionsToMerging(t *testing.T) {
	now := time.Date(2026, 3, 20, 14, 11, 0, 0, time.UTC)
	st := newMemStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "REVIEWING", Phase: 2, PRNumber: 42}))
	require.NoError(t, st.UpsertWakeSchedule(context.Background(), store.WakeSchedule{
		SessionID: "default",
		WakeKind:  "poll_review",
		DueAt:     now.Add(-time.Minute),
		DedupeKey: "run:default:poll_review",
	}))

	controller := loomruntime.NewController(st, loomruntime.Config{
		HolderID:     "controller-1",
		LeaseTTL:     time.Minute,
		PollInterval: time.Minute,
		Now:          func() time.Time { return now },
	})

	lifecycle, err := controller.ProcessDueWake(context.Background(), &pollingGitHubClientMock{reviewStatus: "APPROVED"})
	require.NoError(t, err)
	assert.Equal(t, "MERGING", lifecycle.WorkflowState)
	assert.Empty(t, lifecycle.NextWakeKind)

	cp, readErr := st.ReadCheckpoint(context.Background())
	require.NoError(t, readErr)
	assert.Equal(t, "MERGING", cp.State)
	require.Len(t, st.decisions, 1)
	assert.Equal(t, "resume", st.decisions[0].Verdict)
}

func TestControllerProcessDueWake_ReviewChangesRequestedTransitionsToAddressingFeedback(t *testing.T) {
	now := time.Date(2026, 3, 20, 14, 12, 0, 0, time.UTC)
	st := newMemStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "REVIEWING", Phase: 2, PRNumber: 42}))
	require.NoError(t, st.UpsertWakeSchedule(context.Background(), store.WakeSchedule{
		SessionID: "default",
		WakeKind:  "poll_review",
		DueAt:     now.Add(-time.Minute),
		DedupeKey: "run:default:poll_review",
	}))

	controller := loomruntime.NewController(st, loomruntime.Config{
		HolderID:     "controller-1",
		LeaseTTL:     time.Minute,
		PollInterval: time.Minute,
		Now:          func() time.Time { return now },
	})

	lifecycle, err := controller.ProcessDueWake(context.Background(), &pollingGitHubClientMock{reviewStatus: "CHANGES_REQUESTED"})
	require.NoError(t, err)
	assert.Equal(t, "ADDRESSING_FEEDBACK", lifecycle.WorkflowState)
	assert.Empty(t, lifecycle.NextWakeKind)

	cp, readErr := st.ReadCheckpoint(context.Background())
	require.NoError(t, readErr)
	assert.Equal(t, "ADDRESSING_FEEDBACK", cp.State)
	require.Len(t, st.decisions, 1)
	assert.Equal(t, "address_feedback", st.decisions[0].Verdict)
}

func TestControllerStartDoesNotScheduleUnsupportedPollWakeStates(t *testing.T) {
	now := time.Date(2026, 3, 20, 14, 13, 0, 0, time.UTC)
	st := newMemStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "DEBUGGING", Phase: 2, PRNumber: 42}))

	controller := loomruntime.NewController(st, loomruntime.Config{
		HolderID:     "controller-1",
		LeaseTTL:     time.Minute,
		PollInterval: 30 * time.Second,
		Now:          func() time.Time { return now },
	})

	lifecycle, err := controller.Start(context.Background())
	require.NoError(t, err)
	assert.Equal(t, loomruntime.ControllerStateResuming, lifecycle.Controller)
	assert.Empty(t, lifecycle.NextWakeKind)

	wakes, readErr := st.ReadWakeSchedules(context.Background(), "default", 10)
	require.NoError(t, readErr)
	assert.Empty(t, wakes)
}

func TestControllerProcessDueWake_ReviewPendingRecordsObservationWithoutPromptReplay(t *testing.T) {
	now := time.Date(2026, 3, 20, 14, 10, 0, 0, time.UTC)
	st := newMemStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "REVIEWING", Phase: 2, PRNumber: 42}))
	require.NoError(t, st.UpsertWakeSchedule(context.Background(), store.WakeSchedule{
		SessionID: "default",
		WakeKind:  "poll_review",
		DueAt:     now.Add(-time.Minute),
		DedupeKey: "run:default:poll_review",
	}))

	controller := loomruntime.NewController(st, loomruntime.Config{
		HolderID:     "controller-1",
		LeaseTTL:     time.Minute,
		PollInterval: time.Minute,
		Now:          func() time.Time { return now },
	})

	gh := &pollingGitHubClientMock{reviewStatus: "PENDING"}
	lifecycle, err := controller.ProcessDueWake(context.Background(), gh)
	require.NoError(t, err)

	assert.Equal(t, "REVIEWING", lifecycle.WorkflowState)
	assert.Equal(t, loomruntime.ControllerStateClaimed, lifecycle.Controller)
	assert.Equal(t, "poll_review", lifecycle.NextWakeKind)
	assert.Equal(t, now.Add(time.Minute), lifecycle.NextWakeAt)
	assert.Equal(t, "poll_observation", lifecycle.DrivenBy)

	cp, err := st.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "REVIEWING", cp.State)

	require.Len(t, st.actions, 1)
	assert.Equal(t, "poll_waiting", st.actions[0].Event)
	require.Len(t, st.events, 1)
	assert.Equal(t, "poll_review", st.events[0].EventKind)
	require.Len(t, st.decisions, 1)
	assert.Equal(t, "await", st.decisions[0].Verdict)
	assert.Equal(t, st.events[0].CorrelationID, st.decisions[0].CorrelationID)
}
