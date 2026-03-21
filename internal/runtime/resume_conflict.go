package runtime

import (
	"context"
	"encoding/json"
	"time"

	"github.com/guillaume7/loom/internal/store"
)

// ResumeConflictKind names fixed conflict categories.
type ResumeConflictKind string

const (
	// ResumeConflictStaleObservation is reported when the wake-up is based on
	// observations that are too old to be trusted for the current decision.
	ResumeConflictStaleObservation ResumeConflictKind = "stale_observation"
	// ResumeConflictLockContention is reported when a required mutation lock
	// is already held by another controller instance.
	ResumeConflictLockContention ResumeConflictKind = "lock_contention"
	// ResumeConflictSupersededWake is reported when the wake-up event has been
	// made obsolete by a newer state or event before execution.
	ResumeConflictSupersededWake ResumeConflictKind = "superseded_wake"

	resumeConflictDecisionKind = "resume_conflict"
	resumeConflictEventKind    = "resume_conflict_detected"
)

// ResumeConflict describes a detected conflict that prevents normal resume processing.
type ResumeConflict struct {
	// Kind identifies which conflict class was detected.
	Kind ResumeConflictKind
	// Outcome is the explicit policy outcome assigned to this conflict.
	// Never empty; always one of: wait, escalate, or block.
	Outcome PolicyOutcome
	// Reason is the human-readable explanation of the conflict.
	Reason string
	// ConflictHolder is set (non-empty) when Kind==ResumeConflictLockContention,
	// identifying which holder holds the lock.
	ConflictHolder string
}

// DetectResumeConflict inspects current runtime state and the wake-up to decide
// whether a resume should proceed or yield to a conflict resolution.
//
// It returns a non-nil *ResumeConflict when:
//   - The observation model is stale (oldest CI or review observation exceeds the
//     stale threshold relative to now)
//   - A PR-scoped mutation lock is held by another controller (when prNumber > 0)
//   - The wake-up was superseded (ClaimedAt is set but the checkpoint has advanced
//     past the wake's claim time by more than DefaultLeaseTTL)
//
// Returns nil if no conflict is detected.
func (c *Controller) DetectResumeConflict(ctx context.Context, wake store.WakeSchedule, prNumber int, model ObservationModel) (*ResumeConflict, error) {
	now := c.cfg.Now().UTC()

	// Check for stale observation.
	if isSummaryStale(model, now) {
		return &ResumeConflict{
			Kind:    ResumeConflictStaleObservation,
			Outcome: PolicyOutcomeBlock,
			Reason:  "observation_model_stale",
		}, nil
	}

	// Check for superseded wake (wake was claimed but checkpoint has since advanced).
	if isWakeSuperseded(wake, model) {
		return &ResumeConflict{
			Kind:    ResumeConflictSupersededWake,
			Outcome: PolicyOutcomeWait,
			Reason:  "wake_superseded_by_newer_state",
		}, nil
	}

	// Check for PR-scoped lock contention (only when a PR is in scope).
	if prNumber > 0 {
		result, err := c.AcquirePRLock(ctx, prNumber)
		if err != nil {
			return nil, err
		}
		if !result.Acquired {
			return &ResumeConflict{
				Kind:           ResumeConflictLockContention,
				Outcome:        PolicyOutcomeWait,
				Reason:         "pr_mutation_lock_held_by_other",
				ConflictHolder: result.ContentionBy,
			}, nil
		}
	}

	return nil, nil
}

// RecordResumeConflict persists an audit record for the detected conflict.
// It writes both an ExternalEvent and a PolicyDecision so the conflict is
// visible to the audit trail and replay mechanisms.
func (c *Controller) RecordResumeConflict(ctx context.Context, conflict ResumeConflict, sessionID string, correlationID string) error {
	now := c.cfg.Now().UTC()
	detail, _ := json.Marshal(map[string]interface{}{
		"kind":            string(conflict.Kind),
		"outcome":         string(conflict.Outcome),
		"reason":          conflict.Reason,
		"conflict_holder": conflict.ConflictHolder,
	})
	if err := c.store.WriteExternalEvent(ctx, store.ExternalEvent{
		SessionID:     sessionID,
		EventSource:   "runtime",
		EventKind:     resumeConflictEventKind,
		CorrelationID: correlationID,
		Payload:       string(detail),
		ObservedAt:    now,
	}); err != nil {
		return err
	}
	return c.store.WritePolicyDecision(ctx, store.PolicyDecision{
		SessionID:     sessionID,
		CorrelationID: correlationID,
		DecisionKind:  resumeConflictDecisionKind,
		Verdict:       string(conflict.Outcome),
		Detail:        string(detail),
		CreatedAt:     now,
	})
}

// isSummaryStale returns true if either the CI or Review summary has a RecordedAt
// timestamp that is older than DefaultObservationStaleAfter relative to now.
// Returns false when both summaries are absent (no observations recorded yet).
func isSummaryStale(model ObservationModel, now time.Time) bool {
	threshold := now.Add(-DefaultObservationStaleAfter)
	ciPresent := model.Summaries.CI != nil && !model.Summaries.CI.RecordedAt.IsZero()
	reviewPresent := model.Summaries.Review != nil && !model.Summaries.Review.RecordedAt.IsZero()

	if !ciPresent && !reviewPresent {
		return false
	}
	if ciPresent && model.Summaries.CI.RecordedAt.Before(threshold) {
		return true
	}
	if reviewPresent && model.Summaries.Review.RecordedAt.Before(threshold) {
		return true
	}
	return false
}

// isWakeSuperseded returns true if the wake was claimed and the checkpoint's
// UpdatedAt timestamp is newer than the wake's ClaimedAt by more than DefaultLeaseTTL,
// indicating the wake predates the current checkpoint state.
func isWakeSuperseded(wake store.WakeSchedule, model ObservationModel) bool {
	if wake.ClaimedAt.IsZero() {
		return false
	}
	checkpointAt := model.Checkpoint.UpdatedAt
	if checkpointAt.IsZero() {
		return false
	}
	return checkpointAt.Sub(wake.ClaimedAt) > DefaultLeaseTTL
}
