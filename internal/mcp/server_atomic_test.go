package mcp_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/guillaume7/loom/internal/fsm"
	"github.com/guillaume7/loom/internal/mcp"
	loomruntime "github.com/guillaume7/loom/internal/runtime"
	"github.com/guillaume7/loom/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --------------------------------------------------------------------------
// Atomicity failure-window tests (TH2.E2.US3 fix)
// --------------------------------------------------------------------------

// transientFailStore fails WriteCheckpointAndAction on the first call only,
// then delegates to the underlying memStore. This simulates a transient I/O
// error that leaves no partial writes in the store.
type transientFailStore struct {
	memStore
	failed bool
}

func newTransientFailStore() *transientFailStore {
	return &transientFailStore{memStore: memStore{empty: true}}
}

func (s *transientFailStore) WriteCheckpointAndAction(ctx context.Context, cp store.Checkpoint, action store.Action) error {
	s.memStore.mu.Lock()
	if !s.failed {
		s.failed = true
		s.memStore.mu.Unlock()
		return errors.New("simulated transient write failure")
	}
	s.memStore.mu.Unlock()
	return s.memStore.WriteCheckpointAndAction(ctx, cp, action)
}

// TestLoomCheckpoint_AtomicWriteFailure_LeavesStoreConsistent verifies that
// when WriteCheckpointAndAction fails, neither the checkpoint nor the action
// log entry is persisted — eliminating the partial-write window.
func TestLoomCheckpoint_AtomicWriteFailure_LeavesStoreConsistent(t *testing.T) {
	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newTransientFailStore()
	s := mcp.NewServer(machine, st, nil)
	mcpSvr := s.MCPServer()

	// First call: WriteCheckpointAndAction fails.
	result := callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{
		"action":        "start",
		"operation_key": "checkpoint:IDLE->SCANNING:attempt1",
	})
	assert.True(t, result.IsError, "expected tool error on first attempt")

	// After the failure the store must be pristine: no checkpoint, no action entry.
	cp, err := st.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Empty(t, cp.State, "checkpoint must not be persisted after failed WriteCheckpointAndAction")

	_, lookupErr := st.ReadActionByOperationKey(context.Background(), "checkpoint:IDLE->SCANNING:attempt1")
	assert.ErrorIs(t, lookupErr, store.ErrActionNotFound,
		"action log entry must not be persisted after failed WriteCheckpointAndAction")
}

// TestLoomCheckpoint_AtomicWriteFailure_StoreWriteFailure_WithOperationKey verifies
// that a store write failure on the idempotent path (with operation_key) returns an
// error, consistent with the non-idempotent path behaviour.
func TestLoomCheckpoint_AtomicWriteFailure_StoreWriteFailure_WithOperationKey(t *testing.T) {
	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newFailingStore()
	s := mcp.NewServer(machine, st, nil)
	mcpSvr := s.MCPServer()

	result := callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{
		"action":        "start",
		"operation_key": "checkpoint:IDLE->SCANNING",
	})
	assert.True(t, result.IsError, "expected tool error when store write fails on idempotent path")
}

// TestLoomCheckpoint_SameProcessRetry_AfterTransientWriteFailure verifies the
// rollback property: when WriteCheckpointAndAction fails with a non-duplicate
// error, the in-memory FSM is rolled back to its pre-transition state so that
// a same-process retry with the same operation_key can successfully fire the
// event again and persist the checkpoint.
//
// Without a rollback the FSM would be stuck at newState after the first
// failure, causing the second attempt to fail with an invalid-transition error.
func TestLoomCheckpoint_SameProcessRetry_AfterTransientWriteFailure(t *testing.T) {
	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newTransientFailStore()
	s := mcp.NewServer(machine, st, nil)
	mcpSvr := s.MCPServer()

	args := map[string]interface{}{
		"action":        "start",
		"operation_key": "checkpoint:IDLE->SCANNING",
	}

	// First call: WriteCheckpointAndAction fails → FSM must be rolled back.
	result1 := callTool(t, mcpSvr, "loom_checkpoint", args)
	assert.True(t, result1.IsError, "expected tool error on first attempt (transient failure)")

	// Verify FSM was rolled back: store still empty, no action persisted.
	cp, err := st.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Empty(t, cp.State, "store checkpoint must remain empty after rollback")

	// Second call: same operation_key, same server process, store now succeeds.
	// The FSM must be at IDLE again (rolled back) so 'start' fires correctly.
	result2 := callTool(t, mcpSvr, "loom_checkpoint", args)
	assert.False(t, result2.IsError, "expected success on retry after rollback")

	var got mcp.CheckpointResult
	require.NoError(t, json.Unmarshal([]byte(toolText(t, result2)), &got))
	assert.Equal(t, "IDLE", got.PreviousState, "retry must fire from rolled-back IDLE state")
	assert.Equal(t, "SCANNING", got.NewState)

	// Verify the action was committed on the successful retry.
	action, lookupErr := st.ReadActionByOperationKey(context.Background(), "checkpoint:IDLE->SCANNING")
	require.NoError(t, lookupErr)
	assert.Equal(t, "IDLE", action.StateBefore)
	assert.Equal(t, "SCANNING", action.StateAfter)
}

// TestLoomCheckpoint_SameProcessRetry_CountersRolledBack verifies that retry
// counters (e.g. awaitingPRRetries) are also restored on rollback, preventing
// premature budget exhaustion when a timeout self-loop write fails transiently.
func TestLoomCheckpoint_SameProcessRetry_CountersRolledBack(t *testing.T) {
	// Build a machine already at AWAITING_PR so we can test timeout self-loops.
	m := fsm.NewMachine(fsm.DefaultConfig())
	if _, err := m.Transition(fsm.EventStart); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Transition(fsm.EventPhaseIdentified); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Transition(fsm.EventCopilotAssigned); err != nil {
		t.Fatal(err)
	}

	st := newTransientFailStore()
	s := mcp.NewServer(m, st, nil)
	mcpSvr := s.MCPServer()

	timeoutArgs := map[string]interface{}{
		"action":        "timeout",
		"operation_key": "checkpoint:AWAITING_PR->AWAITING_PR:t1",
	}

	// First call: write fails → rollback → awaitingPRRetries must be 0 again.
	r1 := callTool(t, mcpSvr, "loom_checkpoint", timeoutArgs)
	assert.True(t, r1.IsError, "expected error on first attempt")

	// Second call: write succeeds this time (transient store only fails once).
	r2 := callTool(t, mcpSvr, "loom_checkpoint", timeoutArgs)
	assert.False(t, r2.IsError, "expected success on retry")

	var got mcp.CheckpointResult
	require.NoError(t, json.Unmarshal([]byte(toolText(t, r2)), &got))
	// AWAITING_PR --timeout--> AWAITING_PR (within budget)
	assert.Equal(t, "AWAITING_PR", got.PreviousState)
	assert.Equal(t, "AWAITING_PR", got.NewState)
}

// TestLoomCheckpoint_BudgetExhaustion_RetryAfterPersistFailure_EmitsOnce
// verifies that elicitation side effects are emitted only after durable
// persistence succeeds, preventing duplicate prompts across retries.
func TestLoomCheckpoint_BudgetExhaustion_RetryAfterPersistFailure_EmitsOnce(t *testing.T) {
	cfg := fsm.DefaultConfig()
	cfg.MaxRetriesAwaitingPR = 0

	m := fsm.NewMachine(cfg)
	if _, err := m.Transition(fsm.EventStart); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Transition(fsm.EventPhaseIdentified); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Transition(fsm.EventCopilotAssigned); err != nil {
		t.Fatal(err)
	}

	st := newTransientFailStore()
	s := mcp.NewServer(m, st, nil)
	mcpSvr := s.MCPServer()

	args := map[string]interface{}{
		"action":        "timeout",
		"operation_key": "checkpoint:AWAITING_PR->AWAITING_PR:budget-exhaustion",
	}

	sess := newTestSession(nextSessionID())
	require.NoError(t, mcpSvr.RegisterSession(context.Background(), sess))
	initializeSessionWithCapabilities(t, mcpSvr, sess, map[string]interface{}{
		"experimental": map[string]interface{}{
			"elicitation": true,
		},
	})

	first := callToolOnSession(t, mcpSvr, sess, "loom_checkpoint", args)
	assert.True(t, first.IsError, "expected first attempt to fail due to transient persistence failure")
	assert.Empty(t, drainNotifications(sess), "elicitation must not emit before persistence succeeds")

	second := callToolOnSession(t, mcpSvr, sess, "loom_checkpoint", args)
	assert.False(t, second.IsError, "expected retry to succeed")

	notes := drainNotifications(sess)
	require.Len(t, notes, 1, "exactly one elicitation must be emitted after successful persistence")
	assert.Equal(t, "loom/elicitation", notes[0].Method)
	assert.Equal(t, "elicitation", notes[0].Params.AdditionalFields["type"])

	third := callToolOnSession(t, mcpSvr, sess, "loom_checkpoint", args)
	assert.False(t, third.IsError, "expected idempotent replay to succeed")
	assert.Empty(t, drainNotifications(sess), "idempotent replay must not emit duplicate elicitation")
}

// duplicateOnWriteStore allows ReadActionByOperationKey to succeed after the
// first WriteCheckpointAndAction commits, then refuses the second write to
// simulate a concurrent duplicate.
type duplicateOnWriteStore struct {
	memStore
	firstDone bool
}

func newDuplicateOnWriteStore() *duplicateOnWriteStore {
	return &duplicateOnWriteStore{memStore: memStore{empty: true}}
}

func (s *duplicateOnWriteStore) WriteCheckpointAndAction(ctx context.Context, cp store.Checkpoint, action store.Action) error {
	s.memStore.mu.Lock()
	defer s.memStore.mu.Unlock()
	if s.firstDone {
		return store.ErrDuplicateOperationKey
	}
	// Commit both writes using the internal (unlocked) logic.
	for _, existing := range s.memStore.actions {
		if existing.OperationKey == action.OperationKey {
			return store.ErrDuplicateOperationKey
		}
	}
	s.memStore.cp = cp
	s.memStore.empty = false
	if action.CreatedAt.IsZero() {
		action.CreatedAt = time.Now().UTC()
	}
	action.ID = int64(len(s.memStore.actions) + 1)
	s.memStore.actions = append(s.memStore.actions, action)
	s.firstDone = true
	return nil
}

// TestLoomCheckpoint_AtomicDuplicateOnWrite_ReturnsCachedResult verifies that
// when WriteCheckpointAndAction returns ErrDuplicateOperationKey (concurrent
// duplicate), the handler looks up the previously committed result and returns
// it successfully — safe replay without a double transition.
func TestLoomCheckpoint_AtomicDuplicateOnWrite_ReturnsCachedResult(t *testing.T) {
	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newDuplicateOnWriteStore()
	s := mcp.NewServer(machine, st, nil)
	mcpSvr := s.MCPServer()

	first := callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{
		"action":        "start",
		"operation_key": "checkpoint:IDLE->SCANNING",
	})
	require.False(t, first.IsError, "first call must succeed")

	// Now the initial ReadActionByOperationKey lookup won't find the entry
	// (simulating a race where the lookup happens before the other goroutine
	// committed), but WriteCheckpointAndAction returns ErrDuplicateOperationKey.
	// We reset firstDone so the next outer lookup succeeds but the write is
	// rejected — exercising the ErrDuplicateOperationKey branch.
	// To do this properly we need to call with a *new* server that hasn't
	// cached the lookup, but the same store that already has the action.
	machine2 := fsm.NewMachine(fsm.DefaultConfig())
	s2 := mcp.NewServer(machine2, st, nil)
	mcpSvr2 := s2.MCPServer()

	// Reset firstDone so the idempotent lookup is bypassed (cold server),
	// but the write is rejected as a duplicate.
	st.firstDone = false
	// Write the action so the second lookup finds it after ErrDuplicateOperationKey.
	st.firstDone = true // next write will return ErrDuplicateOperationKey

	second := callTool(t, mcpSvr2, "loom_checkpoint", map[string]interface{}{
		"action":        "start",
		"operation_key": "checkpoint:IDLE->SCANNING",
	})
	require.False(t, second.IsError, "second call must succeed via ErrDuplicateOperationKey replay")
	assert.Equal(t, toolText(t, first), toolText(t, second), "replayed result must match original")
}

func TestLoomCheckpoint_IdempotentDuplicateWithoutCachedResult_RollsBackFSM(t *testing.T) {
	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newDuplicateWithoutCachedResultStore()
	s := mcp.NewServer(machine, st, nil)
	mcpSvr := s.MCPServer()

	result := callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{
		"action":        "start",
		"operation_key": "checkpoint:IDLE->SCANNING",
	})
	assert.True(t, result.IsError, "expected tool error for duplicate without cached result")
	assert.Contains(t, toolText(t, result), "duplicate operation key without cached result")

	next := callTool(t, mcpSvr, "loom_next_step", nil)
	assert.False(t, next.IsError)

	var got mcp.NextStepResult
	require.NoError(t, json.Unmarshal([]byte(toolText(t, next)), &got))
	assert.Equal(t, "IDLE", got.State, "FSM must be rolled back when duplicate branch cannot return cached result")
}

func TestReadOnlyTools_SkipIdempotencyLookup(t *testing.T) {
	s, mcpSvr := newTestServer(t)
	st := s.Store().(*memStore)

	getState := callTool(t, mcpSvr, "loom_get_state", nil)
	heartbeat := callTool(t, mcpSvr, "loom_heartbeat", nil)

	assert.False(t, getState.IsError)
	assert.False(t, heartbeat.IsError)

	st.mu.Lock()
	defer st.mu.Unlock()
	assert.Equal(t, 0, st.readActionLookupCalls)
}

func TestLoomHeartbeat_ReturnsCurrentState(t *testing.T) {
	_, mcpSvr := newTestServer(t)

	result := callTool(t, mcpSvr, "loom_heartbeat", nil)
	assert.False(t, result.IsError)

	var got mcp.HeartbeatResult
	require.NoError(t, json.Unmarshal([]byte(toolText(t, result)), &got))

	assert.Equal(t, "IDLE", got.State)
	// IDLE is a non-gate state: Wait must be false, RetryInSeconds must be 0.
	assert.False(t, got.Wait)
	assert.Equal(t, 0, got.RetryInSeconds)
}

func TestLoomGetState_ReturnsState(t *testing.T) {
	_, mcpSvr := newTestServer(t)

	result := callTool(t, mcpSvr, "loom_get_state", nil)
	assert.False(t, result.IsError)

	var got mcp.GetStateResult
	require.NoError(t, json.Unmarshal([]byte(toolText(t, result)), &got))

	assert.NotEmpty(t, got.State)
}

func TestLoomAbort_RejectsMissingRecoverableState(t *testing.T) {
	_, mcpSvr := newTestServer(t)

	result := callTool(t, mcpSvr, "loom_abort", nil)
	assert.True(t, result.IsError)
	assert.Contains(t, toolText(t, result), loomruntime.ErrNothingToPause.Error())
	
}

func TestLoomAbort_WritesAuditedOperatorPauseRecords(t *testing.T) {
	s, mcpSvr := newTestServer(t)
	st := s.Store().(*memStore)
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "AWAITING_CI", Phase: 2, PRNumber: 42}))

	result, _ := callToolWithSession(t, mcpSvr, "loom_abort", nil)
	assert.False(t, result.IsError)

	var got mcp.AbortResult
	require.NoError(t, json.Unmarshal([]byte(toolText(t, result)), &got))

	assert.Equal(t, "PAUSED", got.State)

	cp, err := st.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "PAUSED", cp.State)
	assert.Equal(t, "AWAITING_CI", cp.ResumeState)

	st.mu.Lock()
	defer st.mu.Unlock()
	require.Len(t, st.actions, 1)
	assert.Equal(t, "manual_override_pause", st.actions[0].Event)
	assert.Equal(t, "AWAITING_CI", st.actions[0].StateBefore)
	assert.Equal(t, "PAUSED", st.actions[0].StateAfter)

	require.Len(t, st.externalEvents, 1)
	assert.Equal(t, "operator", st.externalEvents[0].EventSource)
	assert.Equal(t, "manual_override.pause", st.externalEvents[0].EventKind)

	require.Len(t, st.policyDecisions, 1)
	assert.Equal(t, "operator_override", st.policyDecisions[0].DecisionKind)
	assert.Equal(t, "pause", st.policyDecisions[0].Verdict)
	assert.Equal(t, st.externalEvents[0].CorrelationID, st.policyDecisions[0].CorrelationID)
}

func TestServer_RaceCondition(t *testing.T) {
	// Verify that concurrent loom_heartbeat / loom_get_state calls do not
	// trigger the data-race detector. No state transitions occur so only
	// read paths are exercised.
	//
	// callToolConcurrent is used instead of callTool because require.*
	// calls runtime.Goexit which must only be invoked from the test goroutine.
	_, mcpSvr := newTestServer(t)

	const goroutines = 8
	const callsEach = 5

	results := make(chan bool, goroutines*callsEach)

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		g := g
		go func() {
			defer wg.Done()
			for j := 0; j < callsEach; j++ {
				var ok bool
				if (g+j)%2 == 0 {
					ok = callToolConcurrent(mcpSvr, "loom_heartbeat", nil)
				} else {
					ok = callToolConcurrent(mcpSvr, "loom_get_state", nil)
				}
				results <- ok
			}
		}()
	}

	wg.Wait()
	close(results)

	for ok := range results {
		assert.True(t, ok, "concurrent tool call returned unexpected result")
	}
}
