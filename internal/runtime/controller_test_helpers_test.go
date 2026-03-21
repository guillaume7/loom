package runtime_test

import (
	"context"
	"time"

	loomgh "github.com/guillaume7/loom/internal/github"
	"github.com/guillaume7/loom/internal/store"
)

type pollingGitHubClientMock struct {
	pr                 *loomgh.PR
	checkRuns          []*loomgh.CheckRun
	reviewStatus       string
	getPRCalls         int
	getCheckRunsCalls  int
	getReviewCalls     int
	markReadyCalls     int
	requestReviewCalls int
	lastRequestedPR    int
	lastReviewer       string
}

func (m *pollingGitHubClientMock) GetPR(_ context.Context, prNumber int) (*loomgh.PR, error) {
	m.getPRCalls++
	if m.pr == nil {
		m.pr = &loomgh.PR{Number: prNumber}
	}
	return m.pr, nil
}

func (m *pollingGitHubClientMock) GetCheckRuns(_ context.Context, _ string) ([]*loomgh.CheckRun, error) {
	m.getCheckRunsCalls++
	return m.checkRuns, nil
}

func (m *pollingGitHubClientMock) GetReviewStatus(_ context.Context, _ int) (string, error) {
	m.getReviewCalls++
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

type duplicatePollWriteStore struct {
	*memStore
	duplicateTriggered bool
	cachedAction       store.Action
}

func newMemStore() *memStore {
	return &memStore{empty: true, runtimeLeases: make(map[string]store.RuntimeLease)}
}

func newDuplicatePollWriteStore() *duplicatePollWriteStore {
	return &duplicatePollWriteStore{memStore: newMemStore()}
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

func (s *memStore) ReadActionByOperationKey(_ context.Context, operationKey string) (store.Action, error) {
	for _, action := range s.actions {
		if action.OperationKey == operationKey {
			return action, nil
		}
	}
	return store.Action{}, store.ErrActionNotFound
}

func (s *memStore) ReadActions(_ context.Context, limit int) ([]store.Action, error) {
	if limit <= 0 {
		return []store.Action{}, nil
	}
	if len(s.actions) == 0 {
		return []store.Action{}, nil
	}
	if limit > len(s.actions) {
		limit = len(s.actions)
	}
	actions := make([]store.Action, 0, limit)
	for index := len(s.actions) - 1; index >= len(s.actions)-limit; index-- {
		actions = append(actions, s.actions[index])
	}
	return actions, nil
}

func (s *memStore) UpsertWakeSchedule(_ context.Context, wake store.WakeSchedule) error {
	if s.wakeErr != nil {
		return s.wakeErr
	}
	if wake.CreatedAt.IsZero() {
		wake.CreatedAt = time.Now().UTC()
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

func (s *memStore) ReadExternalEvents(_ context.Context, sessionID string, _ int) ([]store.ExternalEvent, error) {
	result := make([]store.ExternalEvent, 0, len(s.events))
	for index := len(s.events) - 1; index >= 0; index-- {
		event := s.events[index]
		if sessionID != "" && event.SessionID != sessionID {
			continue
		}
		result = append(result, event)
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

func (s *memStore) ReadPolicyDecisions(_ context.Context, sessionID string, _ int) ([]store.PolicyDecision, error) {
	result := make([]store.PolicyDecision, 0, len(s.decisions))
	for index := len(s.decisions) - 1; index >= 0; index-- {
		decision := s.decisions[index]
		if sessionID != "" && decision.SessionID != sessionID {
			continue
		}
		result = append(result, decision)
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

func (s *duplicatePollWriteStore) WriteCheckpointAndAction(_ context.Context, cp store.Checkpoint, action store.Action) error {
	s.cp = cp
	s.empty = false
	action.ID = int64(len(s.actions) + 1)
	s.actions = append(s.actions, action)
	s.cachedAction = action
	s.duplicateTriggered = true
	return store.ErrDuplicateOperationKey
}

func (s *duplicatePollWriteStore) ReadActionByOperationKey(_ context.Context, operationKey string) (store.Action, error) {
	if s.duplicateTriggered && s.cachedAction.OperationKey == operationKey {
		return s.cachedAction, nil
	}
	return s.memStore.ReadActionByOperationKey(context.Background(), operationKey)
}
