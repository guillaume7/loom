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
	"github.com/stretchr/testify/require"
)

func TestReplayRegressionFixtures(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)

	type replayCase struct {
		fixtureID string
		branch    string
		setup     func(t *testing.T, st *memStore, now time.Time)
		assert    func(t *testing.T, fixture loomruntime.ReplayFixture, fixtureID string, branch string)
	}

	testCases := []replayCase{
		{
			fixtureID: "fixture.wakeup.poll-ci.continue",
			branch:    "wake-up",
			setup: func(t *testing.T, st *memStore, now time.Time) {
				t.Helper()
				require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{
					StoryID:   "story-wakeup",
					State:     "AWAITING_CI",
					Phase:     2,
					PRNumber:  42,
					UpdatedAt: now.Add(-2 * time.Minute),
				}))
				pollPayload := marshalMap(t, map[string]any{
					"session_id":       "story-wakeup",
					"correlation_id":   "wake-ci-1",
					"wake_kind":        "poll_ci",
					"policy_decision":  "ci_readiness",
					"policy_outcome":   "continue",
					"policy_reason":    "ci_green",
					"previous_state":   "AWAITING_CI",
					"new_state":        "AWAITING_CI",
					"decision_verdict": "continue",
					"pr_number":        42,
					"ci": map[string]any{
						"total_checks": 2,
						"green_checks": 2,
					},
				})
				require.NoError(t, st.WriteExternalEvent(context.Background(), store.ExternalEvent{
					SessionID:     "story-wakeup",
					EventSource:   "poll",
					EventKind:     "poll_ci",
					CorrelationID: "wake-ci-1",
					Payload:       pollPayload,
					ObservedAt:    now.Add(-time.Minute),
				}))
				require.NoError(t, st.WritePolicyDecision(context.Background(), store.PolicyDecision{
					SessionID:     "story-wakeup",
					CorrelationID: "wake-ci-1",
					DecisionKind:  "ci_readiness",
					Verdict:       "continue",
					InputHash:     "hash-wake-ci",
					Detail:        pollPayload,
					CreatedAt:     now.Add(-45 * time.Second),
				}))
				require.NoError(t, st.WriteAction(context.Background(), store.Action{
					SessionID:    "story-wakeup",
					OperationKey: "wake-action-1",
					StateBefore:  "AWAITING_CI",
					StateAfter:   "AWAITING_CI",
					Event:        "runtime.poll",
					Detail:       "poll_ci continue",
					CreatedAt:    now.Add(-30 * time.Second),
				}))
			},
			assert: func(t *testing.T, fixture loomruntime.ReplayFixture, fixtureID string, branch string) {
				t.Helper()
				msg := fixtureMsg(fixtureID, branch)
				require.Equalf(t, "story-wakeup", fixture.SessionID, "%s session id mismatch", msg)
				require.NotEmptyf(t, fixture.Observations.CI, "%s expected CI observations from poll wake", msg)
				requirePolicyEntry(t, fixture, "ci_readiness", "continue", fixtureID, branch)
				requireEvent(t, fixture, "poll_ci", "wake-ci-1", fixtureID, branch)
			},
		},
		{
			fixtureID: "fixture.policy.merge.blocked",
			branch:    "policy",
			setup: func(t *testing.T, st *memStore, now time.Time) {
				t.Helper()
				require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{
					StoryID:   "story-policy",
					State:     "REVIEWING",
					Phase:     3,
					PRNumber:  77,
					UpdatedAt: now.Add(-2 * time.Minute),
				}))
				decisionPayload := marshalMap(t, map[string]any{
					"session_id":            "story-policy",
					"correlation_id":        "policy-merge-1",
					"wake_kind":             "poll_review",
					"policy_decision":       "review_readiness",
					"policy_outcome":        "continue",
					"policy_reason":         "review_approved",
					"merge_policy_decision": "merge_gate",
					"merge_policy_outcome":  "block",
					"merge_policy_reason":   "merge_blocked_by_ci",
					"previous_state":        "REVIEWING",
					"new_state":             "REVIEWING",
					"decision_verdict":      "block",
					"pr_number":             77,
				})
				require.NoError(t, st.WritePolicyDecision(context.Background(), store.PolicyDecision{
					SessionID:     "story-policy",
					CorrelationID: "policy-merge-1",
					DecisionKind:  "merge_readiness",
					Verdict:       "block",
					InputHash:     "hash-merge-block",
					Detail:        decisionPayload,
					CreatedAt:     now.Add(-50 * time.Second),
				}))
				require.NoError(t, st.WriteAction(context.Background(), store.Action{
					SessionID:    "story-policy",
					OperationKey: "policy-action-1",
					StateBefore:  "REVIEWING",
					StateAfter:   "REVIEWING",
					Event:        "runtime.policy",
					Detail:       "merge gate blocked",
					CreatedAt:    now.Add(-40 * time.Second),
				}))
			},
			assert: func(t *testing.T, fixture loomruntime.ReplayFixture, fixtureID string, branch string) {
				t.Helper()
				msg := fixtureMsg(fixtureID, branch)
				require.Equalf(t, "story-policy", fixture.SessionID, "%s session id mismatch", msg)
				requireDecision(t, fixture, "merge_readiness", "block", fixtureID, branch)
				requirePolicyEntry(t, fixture, "merge_readiness", "block", fixtureID, branch)
				require.Truef(t, len(fixture.Policies.Entries) > 0, "%s expected policy audit entries", msg)
			},
		},
		{
			fixtureID: "fixture.locking.resume-conflict.wait",
			branch:    "locking",
			setup: func(t *testing.T, st *memStore, now time.Time) {
				t.Helper()
				require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{
					StoryID:   "story-lock",
					State:     "AWAITING_CI",
					Phase:     2,
					PRNumber:  91,
					UpdatedAt: now,
				}))
				conflictDetail := marshalMap(t, map[string]any{
					"kind":            "lock_contention",
					"outcome":         "wait",
					"reason":          "pr_mutation_lock_held_by_other",
					"conflict_holder": "controller-2",
				})
				require.NoError(t, st.WriteExternalEvent(context.Background(), store.ExternalEvent{
					SessionID:     "story-lock",
					EventSource:   "runtime",
					EventKind:     "resume_conflict_detected",
					CorrelationID: "resume-conflict-1",
					Payload:       conflictDetail,
					ObservedAt:    now.Add(-time.Minute),
				}))
				require.NoError(t, st.WritePolicyDecision(context.Background(), store.PolicyDecision{
					SessionID:     "story-lock",
					CorrelationID: "resume-conflict-1",
					DecisionKind:  "resume_conflict",
					Verdict:       "wait",
					Detail:        conflictDetail,
					CreatedAt:     now.Add(-30 * time.Second),
				}))
				require.NoError(t, st.UpsertRuntimeLease(context.Background(), store.RuntimeLease{
					LeaseKey:  loomruntime.PRLockKey(91),
					HolderID:  "controller-2",
					Scope:     loomruntime.PRLockScope,
					ExpiresAt: now.Add(2 * time.Minute),
					CreatedAt: now.Add(-time.Minute),
					RenewedAt: now,
				}))
			},
			assert: func(t *testing.T, fixture loomruntime.ReplayFixture, fixtureID string, branch string) {
				t.Helper()
				msg := fixtureMsg(fixtureID, branch)
				require.Equalf(t, "story-lock", fixture.SessionID, "%s session id mismatch", msg)
				requireDecision(t, fixture, "resume_conflict", "wait", fixtureID, branch)
				event := requireEvent(t, fixture, "resume_conflict_detected", "resume-conflict-1", fixtureID, branch)
				require.Containsf(t, event.Payload, "lock_contention", "%s expected lock contention payload", msg)
			},
		},
		{
			fixtureID: "fixture.recovery.lease-recovered",
			branch:    "recovery",
			setup: func(t *testing.T, st *memStore, now time.Time) {
				t.Helper()
				require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{
					StoryID:   "story-recovery",
					State:     "AWAITING_CI",
					Phase:     2,
					PRNumber:  58,
					UpdatedAt: now.Add(-2 * time.Minute),
				}))
				recoveryDetail := marshalMap(t, map[string]any{
					"previous_holder":   "controller-1",
					"lease_expired_at":  now.Add(-time.Minute).Format(time.RFC3339),
					"recovery_holder":   "controller-2",
					"committed_actions": 1,
				})
				require.NoError(t, st.WritePolicyDecision(context.Background(), store.PolicyDecision{
					SessionID:     "story-recovery",
					CorrelationID: "lease_recovery:run:story-recovery:2026-03-21T12:00:00Z",
					DecisionKind:  "lease_recovery",
					Verdict:       "recovered",
					Detail:        recoveryDetail,
					CreatedAt:     now.Add(-30 * time.Second),
				}))
				require.NoError(t, st.WriteAction(context.Background(), store.Action{
					SessionID:    "story-recovery",
					OperationKey: "recovery-action-1",
					StateBefore:  "AWAITING_CI",
					StateAfter:   "AWAITING_CI",
					Event:        "runtime.recovered_lease",
					Detail:       "lease recovered and resumed",
					CreatedAt:    now.Add(-25 * time.Second),
				}))
			},
			assert: func(t *testing.T, fixture loomruntime.ReplayFixture, fixtureID string, branch string) {
				t.Helper()
				msg := fixtureMsg(fixtureID, branch)
				requireDecision(t, fixture, "lease_recovery", "recovered", fixtureID, branch)
				decision := requireDecision(t, fixture, "lease_recovery", "recovered", fixtureID, branch)
				require.Containsf(t, decision.Detail, "recovery_holder", "%s expected recovery detail payload", msg)
				requireAction(t, fixture, "runtime.recovered_lease", "recovery-action-1", fixtureID, branch)
			},
		},
		{
			fixtureID: "fixture.bounded-job-failure.timeout-retry",
			branch:    "bounded-job-failure",
			setup: func(t *testing.T, st *memStore, now time.Time) {
				t.Helper()
				require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{
					StoryID:   "story-agent-failure",
					State:     "CODING",
					Phase:     4,
					PRNumber:  0,
					UpdatedAt: now.Add(-2 * time.Minute),
				}))
				failedDetail := marshalMap(t, map[string]any{
					"story_id":     "TH3.E5.US4",
					"pid":          54321,
					"exit_code":    1,
					"failure_kind": "timeout",
					"outcome":      "retry",
					"lock_state":   "recoverable",
					"observed_at":  now.Format(time.RFC3339),
				})
				require.NoError(t, st.WriteAction(context.Background(), store.Action{
					SessionID:    "story-agent-failure",
					OperationKey: "background_agent_failed:story-agent-failure:TH3.E5.US4:54321:1",
					StateBefore:  "background_agent_exited",
					StateAfter:   "background_agent_failed",
					Event:        "background_agent_failed",
					Detail:       failedDetail,
					CreatedAt:    now.Add(-20 * time.Second),
				}))
				require.NoError(t, st.WritePolicyDecision(context.Background(), store.PolicyDecision{
					SessionID:     "story-agent-failure",
					CorrelationID: "agent-failure-1",
					DecisionKind:  "agent_job_failure",
					Verdict:       "retry",
					Detail:        failedDetail,
					CreatedAt:     now.Add(-15 * time.Second),
				}))
			},
			assert: func(t *testing.T, fixture loomruntime.ReplayFixture, fixtureID string, branch string) {
				t.Helper()
				msg := fixtureMsg(fixtureID, branch)
				action := requireAction(t, fixture, "background_agent_failed", "background_agent_failed:story-agent-failure:TH3.E5.US4:54321:1", fixtureID, branch)
				require.Containsf(t, action.Detail, "\"failure_kind\":\"timeout\"", "%s expected timeout failure kind", msg)
				requireDecision(t, fixture, "agent_job_failure", "retry", fixtureID, branch)
			},
		},
	}

	requiredBranches := map[string]bool{
		"wake-up":             false,
		"policy":              false,
		"locking":             false,
		"recovery":            false,
		"bounded-job-failure": false,
	}

	for _, tc := range testCases {
		tc := tc
		requiredBranches[tc.branch] = true
		t.Run(tc.fixtureID, func(t *testing.T) {
			st := newMemStore()
			tc.setup(t, st, now)

			fixture, err := loomruntime.AssembleReplayFixture(context.Background(), st, now)
			require.NoErrorf(t, err, "%s assemble replay fixture failed", fixtureMsg(tc.fixtureID, tc.branch))
			require.Equalf(t, now, fixture.CapturedAt, "%s captured_at mismatch", fixtureMsg(tc.fixtureID, tc.branch))

			tc.assert(t, fixture, tc.fixtureID, tc.branch)
		})
	}

	for branch, covered := range requiredBranches {
		require.Truef(t, covered, "branch coverage missing for %q", branch)
	}
}

func fixtureMsg(fixtureID string, branch string) string {
	return fmt.Sprintf("fixture=%s branch=%s", fixtureID, branch)
}

func marshalMap(t *testing.T, v map[string]any) string {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	return string(data)
}

func requireEvent(t *testing.T, fixture loomruntime.ReplayFixture, eventKind string, correlationID string, fixtureID string, branch string) store.ExternalEvent {
	t.Helper()
	for _, event := range fixture.Events {
		if event.EventKind == eventKind && event.CorrelationID == correlationID {
			return event
		}
	}
	require.Failf(t, "missing replay event", "%s expected event kind=%s correlation_id=%s", fixtureMsg(fixtureID, branch), eventKind, correlationID)
	return store.ExternalEvent{}
}

func requireDecision(t *testing.T, fixture loomruntime.ReplayFixture, decisionKind string, verdict string, fixtureID string, branch string) store.PolicyDecision {
	t.Helper()
	for _, decision := range fixture.Decisions {
		if decision.DecisionKind == decisionKind && decision.Verdict == verdict {
			return decision
		}
	}
	require.Failf(t, "missing replay decision", "%s expected decision kind=%s verdict=%s", fixtureMsg(fixtureID, branch), decisionKind, verdict)
	return store.PolicyDecision{}
}

func requireAction(t *testing.T, fixture loomruntime.ReplayFixture, event string, operationKey string, fixtureID string, branch string) store.Action {
	t.Helper()
	for _, action := range fixture.Actions {
		if action.Event == event && action.OperationKey == operationKey {
			return action
		}
	}
	require.Failf(t, "missing replay action", "%s expected action event=%s operation_key=%s", fixtureMsg(fixtureID, branch), event, operationKey)
	return store.Action{}
}

func requirePolicyEntry(t *testing.T, fixture loomruntime.ReplayFixture, decisionKind string, verdict string, fixtureID string, branch string) loomruntime.PolicyAuditEntry {
	t.Helper()
	for _, entry := range fixture.Policies.Entries {
		if entry.DecisionKind == decisionKind && strings.EqualFold(entry.Verdict, verdict) {
			return entry
		}
	}
	require.Failf(t, "missing replay policy audit entry", "%s expected policy entry decision_kind=%s verdict=%s", fixtureMsg(fixtureID, branch), decisionKind, verdict)
	return loomruntime.PolicyAuditEntry{}
}
