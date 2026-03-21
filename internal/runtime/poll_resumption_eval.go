package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/guillaume7/loom/internal/fsm"
	"github.com/guillaume7/loom/internal/store"
)

func (c *Controller) evaluateDueWake(ctx context.Context, cp store.Checkpoint, gh PollingClient, now time.Time, wake store.WakeSchedule) (pollEvaluation, error) {
	sessionID := RunIdentifier(cp)
	wakeKind := wake.WakeKind
	correlationID := fmt.Sprintf("poll:%s:%s:%d", sessionID, wakeKind, now.UnixNano())
	observation := pollObservation{
		SessionID:     sessionID,
		CorrelationID: correlationID,
		WakeKind:      wakeKind,
		PreviousState: cp.State,
		PRNumber:      cp.PRNumber,
		RetryCount:    cp.RetryCount,
	}
	nextCheckpoint := cp
	nextCheckpoint.UpdatedAt = now

	switch fsm.State(cp.State) {
	case fsm.StateAwaitingReady:
		if cp.PRNumber <= 0 {
			return pollEvaluation{}, ErrMissingPollPRNumber
		}
		pr, err := gh.GetPR(ctx, cp.PRNumber)
		if err != nil {
			return pollEvaluation{}, err
		}
		if pr == nil {
			return pollEvaluation{}, fmt.Errorf("PR #%d not found", cp.PRNumber)
		}
		draft := pr.Draft
		observation.Draft = &draft
		observation.Branch = branchPollSummary{
			BaseRef: strings.TrimSpace(pr.BaseRef),
			HeadRef: strings.TrimSpace(pr.HeadRef),
			HeadSHA: strings.TrimSpace(pr.HeadSHA),
		}
		if !pr.Draft {
			observation.Outcome = "ready_for_review"
			observation.ActionEvent = string(fsm.EventPRReady)
			observation.DecisionVerdict = "resume"
			nextCheckpoint.State = string(fsm.StateAwaitingCI)
			nextCheckpoint.ResumeState = ""
			nextCheckpoint.RetryCount = 0
			break
		}

		nextRetryCount := cp.RetryCount + 1
		observation.Outcome = "draft"
		observation.ActionEvent = string(fsm.EventTimeout)
		if nextRetryCount > fsm.DefaultConfig().MaxRetriesAwaitingReady {
			observation.Outcome = "draft_retry_exhausted"
			observation.DecisionVerdict = "pause"
			nextCheckpoint.State = string(fsm.StatePaused)
			nextCheckpoint.ResumeState = cp.State
			nextCheckpoint.RetryCount = 0
		} else {
			observation.DecisionVerdict = "await"
			nextCheckpoint.State = cp.State
			nextCheckpoint.ResumeState = ""
			nextCheckpoint.RetryCount = nextRetryCount
		}
		observation.RetryCount = nextCheckpoint.RetryCount

	case fsm.StateAwaitingCI:
		if cp.PRNumber <= 0 {
			return pollEvaluation{}, ErrMissingPollPRNumber
		}
		pr, err := gh.GetPR(ctx, cp.PRNumber)
		if err != nil {
			return pollEvaluation{}, err
		}
		if pr == nil {
			return pollEvaluation{}, fmt.Errorf("PR #%d not found", cp.PRNumber)
		}
		observation.Branch = branchPollSummary{
			BaseRef: strings.TrimSpace(pr.BaseRef),
			HeadRef: strings.TrimSpace(pr.HeadRef),
			HeadSHA: strings.TrimSpace(pr.HeadSHA),
		}
		if strings.TrimSpace(pr.HeadSHA) == "" {
			return pollEvaluation{}, ErrMissingPollHeadSHA
		}
		summary, err := collectCIPollSummary(ctx, gh, pr.HeadSHA)
		if err != nil {
			return pollEvaluation{}, err
		}
		observation.CI = summary
		decision := EvaluateCIReadiness(CIReadinessInput{
			Checkpoint: checkpointObservationFromCheckpoint(cp),
			Observation: &CIObservation{
				Authority:     ObservationAuthorityAuthoritative,
				Source:        pollEventSource,
				CorrelationID: correlationID,
				ObservedAt:    now,
				EventKind:     wakeKind,
				TotalChecks:   summary.TotalChecks,
				GreenChecks:   summary.GreenChecks,
				PendingChecks: append([]string(nil), summary.PendingChecks...),
				FailedChecks:  append([]string(nil), summary.FailedChecks...),
			},
			EvaluatedAt: now,
			StaleAfter:  c.cfg.PollInterval,
		})
		applyCIReadinessDecision(cp, decision, &observation, &nextCheckpoint)

	case fsm.StateReviewing:
		if cp.PRNumber <= 0 {
			return pollEvaluation{}, ErrMissingPollPRNumber
		}
		pr, err := gh.GetPR(ctx, cp.PRNumber)
		if err != nil {
			return pollEvaluation{}, err
		}
		if pr == nil {
			return pollEvaluation{}, fmt.Errorf("PR #%d not found", cp.PRNumber)
		}
		observation.Branch = branchPollSummary{
			BaseRef: strings.TrimSpace(pr.BaseRef),
			HeadRef: strings.TrimSpace(pr.HeadRef),
			HeadSHA: strings.TrimSpace(pr.HeadSHA),
		}
		status, err := gh.GetReviewStatus(ctx, cp.PRNumber)
		if err != nil {
			return pollEvaluation{}, err
		}
		normalizedStatus := strings.ToUpper(strings.TrimSpace(status))
		observation.ReviewStatus = normalizedStatus
		freshReviewObs := &ReviewObservation{
			Authority:     ObservationAuthorityAuthoritative,
			Source:        pollEventSource,
			CorrelationID: correlationID,
			ObservedAt:    now,
			EventKind:     wakeKind,
			Status:        normalizedStatus,
		}
		decision := EvaluateReviewReadiness(ReviewReadinessInput{
			Checkpoint:  checkpointObservationFromCheckpoint(cp),
			Observation: freshReviewObs,
			EvaluatedAt: now,
			StaleAfter:  c.cfg.PollInterval,
		})
		historicalModel, err := AssembleObservationModel(ctx, c.store)
		if err != nil {
			return pollEvaluation{}, err
		}
		historicalModel.Review = append([]ReviewObservation{*freshReviewObs}, historicalModel.Review...)
		applyReviewReadinessDecision(cp, decision, &observation, &nextCheckpoint, historicalModel, now, c.cfg.PollInterval)

	default:
		return pollEvaluation{}, fmt.Errorf("%w: %s", ErrUnsupportedPollResume, cp.State)
	}

	observation.NewState = nextCheckpoint.State
	observation.ResumeState = nextCheckpoint.ResumeState
	payload, err := json.Marshal(observation)
	if err != nil {
		return pollEvaluation{}, err
	}

	persistedVerdict := observation.PersistedVerdict
	if persistedVerdict == "" {
		persistedVerdict = observation.DecisionVerdict
	}

	operationKey := pollResumeOperationKey(sessionID, wake)
	record := store.RuntimeControlRecord{
		Checkpoint: nextCheckpoint,
		Action: store.Action{
			SessionID:    sessionID,
			OperationKey: operationKey,
			StateBefore:  cp.State,
			StateAfter:   nextCheckpoint.State,
			Event:        observation.ActionEvent,
			Detail:       string(payload),
			CreatedAt:    now,
		},
		ExternalEvent: store.ExternalEvent{
			SessionID:     sessionID,
			EventSource:   pollEventSource,
			EventKind:     wakeKind,
			CorrelationID: correlationID,
			Payload:       string(payload),
			ObservedAt:    now,
		},
		PolicyDecision: store.PolicyDecision{
			SessionID:     sessionID,
			CorrelationID: correlationID,
			DecisionKind:  pollDecisionKindForObservation(observation),
			Verdict:       persistedVerdict,
			InputHash:     hashPollInput(payload),
			Detail:        string(payload),
			CreatedAt:     now,
		},
	}

	var nextWake store.WakeSchedule
	if wakeKind := inferWakeKind(nextCheckpoint.State); wakeKind != "" {
		nextWake = store.WakeSchedule{
			SessionID: sessionID,
			WakeKind:  wakeKind,
			DueAt:     now.Add(c.cfg.PollInterval),
			DedupeKey: fmt.Sprintf("%s:%s", LeaseKey(nextCheckpoint), wakeKind),
		}
	}

	return pollEvaluation{record: record, next: nextCheckpoint, nextWake: nextWake, verdict: observation.DecisionVerdict}, nil
}

func checkpointObservationFromCheckpoint(cp store.Checkpoint) CheckpointObservation {
	return CheckpointObservation{
		Authority:   ObservationAuthorityAuthoritative,
		State:       cp.State,
		ResumeState: cp.ResumeState,
		Phase:       cp.Phase,
		PRNumber:    cp.PRNumber,
		IssueNumber: cp.IssueNumber,
		RetryCount:  cp.RetryCount,
		UpdatedAt:   cp.UpdatedAt.UTC(),
	}
}

func applyCIReadinessDecision(cp store.Checkpoint, decision PolicyEvaluation, observation *pollObservation, nextCheckpoint *store.Checkpoint) {
	observation.PolicyDecision = string(decision.Name)
	observation.PolicyOutcome = string(decision.Outcome)
	observation.PolicyReason = decision.Reason
	nextRetryCount := cp.RetryCount + 1

	switch decision.Outcome {
	case PolicyOutcomeContinue:
		observation.Outcome = "ci_green"
		observation.ActionEvent = string(fsm.EventCIGreen)
		observation.DecisionVerdict = "resume"
		observation.PersistedVerdict = string(PolicyOutcomeContinue)
		nextCheckpoint.State = string(fsm.StateReviewing)
		nextCheckpoint.ResumeState = ""
		nextCheckpoint.RetryCount = 0
	case PolicyOutcomeEscalate:
		observation.Outcome = "ci_failed"
		observation.ActionEvent = string(fsm.EventCIRed)
		observation.DecisionVerdict = "debug"
		observation.PersistedVerdict = string(PolicyOutcomeEscalate)
		nextCheckpoint.State = string(fsm.StateDebugging)
		nextCheckpoint.ResumeState = ""
		nextCheckpoint.RetryCount = 0
	default:
		observation.Outcome = decision.Reason
		observation.ActionEvent = string(fsm.EventTimeout)
		if nextRetryCount > fsm.DefaultConfig().MaxRetriesAwaitingCI {
			observation.DecisionVerdict = "pause"
			observation.PersistedVerdict = "pause"
			nextCheckpoint.State = string(fsm.StatePaused)
			nextCheckpoint.ResumeState = cp.State
			nextCheckpoint.RetryCount = 0
		} else {
			observation.DecisionVerdict = "await"
			observation.PersistedVerdict = string(PolicyOutcomeWait)
			nextCheckpoint.State = cp.State
			nextCheckpoint.ResumeState = ""
			nextCheckpoint.RetryCount = nextRetryCount
		}
	}
	observation.RetryCount = nextCheckpoint.RetryCount
}

func applyReviewReadinessDecision(cp store.Checkpoint, decision PolicyEvaluation, observation *pollObservation, nextCheckpoint *store.Checkpoint, model ObservationModel, evaluatedAt time.Time, staleAfter time.Duration) {
	observation.PolicyDecision = string(decision.Name)
	observation.PolicyOutcome = string(decision.Outcome)
	observation.PolicyReason = decision.Reason
	nextCheckpoint.ResumeState = ""
	nextCheckpoint.RetryCount = 0

	switch decision.Outcome {
	case PolicyOutcomeContinue:
		mergeDecision := EvaluateMergeReadiness(MergeReadinessInput{
			Model:       model,
			EvaluatedAt: evaluatedAt,
			StaleAfter:  staleAfter,
		})
		observation.MergePolicyDecision = string(mergeDecision.Name)
		observation.MergePolicyOutcome = string(mergeDecision.Outcome)
		observation.MergePolicyReason = mergeDecision.Reason
		switch mergeDecision.Outcome {
		case PolicyOutcomeContinue:
			observation.Outcome = "review_approved"
			observation.ActionEvent = string(fsm.EventReviewApproved)
			observation.DecisionVerdict = "resume"
			observation.PolicyOutcome = string(PolicyOutcomeContinue)
			observation.PersistedVerdict = string(PolicyOutcomeContinue)
			nextCheckpoint.State = string(fsm.StateMerging)
		case PolicyOutcomeWait:
			observation.Outcome = mergeDecision.Reason
			observation.ActionEvent = pollWaitEvent
			observation.DecisionVerdict = "await"
			observation.PolicyOutcome = string(PolicyOutcomeWait)
			observation.PersistedVerdict = string(PolicyOutcomeWait)
			nextCheckpoint.State = cp.State
		default:
			observation.Outcome = mergeDecision.Reason
			observation.ActionEvent = pollWaitEvent
			observation.DecisionVerdict = "await"
			observation.PolicyOutcome = string(PolicyOutcomeBlock)
			observation.PersistedVerdict = string(PolicyOutcomeBlock)
			nextCheckpoint.State = cp.State
		}
	case PolicyOutcomeBlock:
		observation.Outcome = "changes_requested"
		observation.ActionEvent = string(fsm.EventReviewChangesRequested)
		observation.DecisionVerdict = "address_feedback"
		observation.PersistedVerdict = string(PolicyOutcomeBlock)
		nextCheckpoint.State = string(fsm.StateAddressingFeedback)
	default:
		observation.Outcome = decision.Reason
		observation.ActionEvent = pollWaitEvent
		observation.DecisionVerdict = "await"
		observation.PersistedVerdict = string(PolicyOutcomeWait)
		nextCheckpoint.State = cp.State
	}
}

func pollDecisionKindForObservation(observation pollObservation) string {
	if observation.PolicyDecision != "" {
		return observation.PolicyDecision
	}
	return pollDecisionKind
}

func pollResumeOperationKey(sessionID string, wake store.WakeSchedule) string {
	return fmt.Sprintf("poll_resume:%s:%s:%s", sessionID, wake.DedupeKey, wakeWindowKey(wake))
}

func wakeWindowKey(wake store.WakeSchedule) string {
	switch {
	case !wake.CreatedAt.IsZero():
		return fmt.Sprintf("created:%d", wake.CreatedAt.UTC().UnixNano())
	case !wake.DueAt.IsZero():
		return fmt.Sprintf("due:%d", wake.DueAt.UTC().UnixNano())
	default:
		return "unknown"
	}
}

func collectCIPollSummary(ctx context.Context, gh PollingClient, sha string) (ciPollSummary, error) {
	checks, err := gh.GetCheckRuns(ctx, sha)
	if err != nil {
		return ciPollSummary{}, err
	}

	summary := ciPollSummary{TotalChecks: len(checks)}
	for _, check := range checks {
		if check == nil {
			continue
		}
		name := strings.TrimSpace(check.Name)
		if name == "" {
			name = "unnamed"
		}

		status := strings.ToLower(strings.TrimSpace(check.Status))
		conclusion := strings.ToLower(strings.TrimSpace(check.Conclusion))
		if status != "completed" {
			summary.PendingChecks = append(summary.PendingChecks, name)
			continue
		}
		if conclusion == "success" {
			summary.GreenChecks++
			continue
		}
		if conclusion == "" {
			summary.PendingChecks = append(summary.PendingChecks, name)
			continue
		}
		summary.FailedChecks = append(summary.FailedChecks, name)
	}

	sort.Strings(summary.PendingChecks)
	sort.Strings(summary.FailedChecks)
	return summary, nil
}

func hashPollInput(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}
