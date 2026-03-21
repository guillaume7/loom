package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/guillaume7/loom/internal/fsm"
	loomgh "github.com/guillaume7/loom/internal/github"
	"github.com/guillaume7/loom/internal/store"
)

const (
	pollEventSource    = "poll"
	pollDecisionKind   = "runtime_poll"
	pollReviewer       = "copilot"
	pollWaitEvent      = "poll_waiting"
	pollDrivenBy       = "poll_observation"
	pollRecordedReason = "poll_observation_recorded"
	pollDuplicateEvent = "wake_duplicate_skipped"
	pollDedupeKind     = "runtime_resume_dedupe"
	pollDedupeDrivenBy = "resume_deduplication"
)

var (
	ErrNoDueWake               = errors.New("no due wake")
	ErrUnsupportedPollResume   = errors.New("poll-driven resumption unsupported for current checkpoint state")
	ErrMissingPollPRNumber     = errors.New("poll-driven resumption requires a persisted PR number")
	ErrMissingPollHeadSHA      = errors.New("poll-driven resumption requires a PR head SHA")
	ErrMissingPollingGitHubAPI = errors.New("github polling client is required")
)

type PollingClient interface {
	GetPR(ctx context.Context, prNumber int) (*loomgh.PR, error)
	GetCheckRuns(ctx context.Context, sha string) ([]*loomgh.CheckRun, error)
	GetReviewStatus(ctx context.Context, prNumber int) (string, error)
}

type ciPollSummary struct {
	TotalChecks   int      `json:"total_checks"`
	GreenChecks   int      `json:"green_checks"`
	PendingChecks []string `json:"pending_checks,omitempty"`
	FailedChecks  []string `json:"failed_checks,omitempty"`
}

type branchPollSummary struct {
	BaseRef string `json:"base_ref,omitempty"`
	HeadRef string `json:"head_ref,omitempty"`
	HeadSHA string `json:"head_sha,omitempty"`
}

func (s ciPollSummary) allGreen() bool {
	return s.TotalChecks > 0 && s.GreenChecks == s.TotalChecks
}

type pollObservation struct {
	SessionID              string            `json:"session_id"`
	CorrelationID          string            `json:"correlation_id"`
	WakeKind               string            `json:"wake_kind"`
	PolicyDecision         string            `json:"policy_decision,omitempty"`
	PolicyOutcome          string            `json:"policy_outcome,omitempty"`
	PolicyReason           string            `json:"policy_reason,omitempty"`
	MergePolicyDecision    string            `json:"merge_policy_decision,omitempty"`
	MergePolicyOutcome     string            `json:"merge_policy_outcome,omitempty"`
	MergePolicyReason      string            `json:"merge_policy_reason,omitempty"`
	PersistedVerdict       string            `json:"-"` // not persisted to JSON; used for PolicyDecision record only
	PreviousState          string            `json:"previous_state"`
	NewState               string            `json:"new_state"`
	DecisionVerdict        string            `json:"decision_verdict"`
	ActionEvent            string            `json:"action_event"`
	Outcome                string            `json:"outcome"`
	PRNumber               int               `json:"pr_number,omitempty"`
	RetryCount             int               `json:"retry_count"`
	ResumeState            string            `json:"resume_state,omitempty"`
	ReviewStatus           string            `json:"review_status,omitempty"`
	Draft                  *bool             `json:"draft,omitempty"`
	ForcedReadyForReview   bool              `json:"forced_ready_for_review,omitempty"`
	Branch                 branchPollSummary `json:"branch,omitempty"`
	CI                     ciPollSummary     `json:"ci,omitempty"`
}

type pollEvaluation struct {
	record   store.RuntimeControlRecord
	next     store.Checkpoint
	nextWake store.WakeSchedule
	verdict  string
}

type duplicateWakeObservation struct {
	SessionID     string `json:"session_id"`
	CorrelationID string `json:"correlation_id"`
	WakeKind      string `json:"wake_kind"`
	DedupeKey     string `json:"dedupe_key"`
	OperationKey  string `json:"operation_key"`
	WorkflowState string `json:"workflow_state"`
	Reason        string `json:"reason"`
	ClaimedAt     string `json:"claimed_at,omitempty"`
	CreatedAt     string `json:"created_at,omitempty"`
}

func (c *Controller) ProcessDueWake(ctx context.Context, gh PollingClient) (Lifecycle, error) {
	if gh == nil {
		return Lifecycle{}, ErrMissingPollingGitHubAPI
	}

	cp, err := c.store.ReadCheckpoint(ctx)
	if err != nil {
		return Lifecycle{}, err
	}

	lifecycle, err := c.startWithCheckpoint(ctx, cp)
	if err != nil {
		return Lifecycle{}, err
	}
	if lifecycle.Controller != ControllerStateWakeDue {
		return lifecycle, ErrNoDueWake
	}

	sessionID := RunIdentifier(cp)
	wakeDedupeKey := fmt.Sprintf("%s:%s", LeaseKey(cp), lifecycle.NextWakeKind)
	wake, ok, err := c.readWakeByDedupeKey(ctx, sessionID, wakeDedupeKey)
	if err != nil {
		return Lifecycle{}, err
	}
	if !ok {
		return lifecycle, ErrNoDueWake
	}

	operationKey := pollResumeOperationKey(sessionID, wake)
	if _, err := c.store.ReadActionByOperationKey(ctx, operationKey); err == nil {
		return c.recordDuplicateWakeSkip(ctx, wake, operationKey, "wake_already_processed")
	} else if !errors.Is(err, store.ErrActionNotFound) {
		return Lifecycle{}, err
	}

	now := c.cfg.Now().UTC()
	evaluation, err := c.evaluateDueWake(ctx, cp, gh, now, wake)
	if err != nil {
		return Lifecycle{}, err
	}
	if err := c.writePollEvaluation(ctx, evaluation.record); err != nil {
		if errors.Is(err, store.ErrDuplicateOperationKey) {
			return c.recordDuplicateWakeSkip(ctx, wake, operationKey, "duplicate_operation_key")
		}
		return Lifecycle{}, err
	}
	if evaluation.nextWake.WakeKind != "" {
		if err := c.store.UpsertWakeSchedule(ctx, evaluation.nextWake); err != nil {
			return Lifecycle{}, err
		}
	}

	updatedLifecycle, err := c.snapshotFromCheckpoint(ctx, evaluation.next)
	if err != nil {
		return Lifecycle{}, err
	}
	updatedLifecycle.DrivenBy = pollDrivenBy
	updatedLifecycle.Reason = pollRecordedReason
	if evaluation.next.State == string(fsm.StatePaused) {
		updatedLifecycle.ResumeState = evaluation.next.ResumeState
	}
	return updatedLifecycle, nil
}

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

func (c *Controller) recordDuplicateWakeSkip(ctx context.Context, wake store.WakeSchedule, operationKey string, reason string) (Lifecycle, error) {
	now := c.cfg.Now().UTC()
	cp, err := c.store.ReadCheckpoint(ctx)
	if err != nil {
		return Lifecycle{}, err
	}

	sessionID := RunIdentifier(cp)
	correlationID := fmt.Sprintf("wake_dedupe:%s:%s:%d", sessionID, wake.WakeKind, now.UnixNano())
	observation := duplicateWakeObservation{
		SessionID:     sessionID,
		CorrelationID: correlationID,
		WakeKind:      wake.WakeKind,
		DedupeKey:     wake.DedupeKey,
		OperationKey:  operationKey,
		WorkflowState: cp.State,
		Reason:        reason,
	}
	if !wake.ClaimedAt.IsZero() {
		observation.ClaimedAt = wake.ClaimedAt.UTC().Format(time.RFC3339Nano)
	}
	if !wake.CreatedAt.IsZero() {
		observation.CreatedAt = wake.CreatedAt.UTC().Format(time.RFC3339Nano)
	}
	payload, err := json.Marshal(observation)
	if err != nil {
		return Lifecycle{}, err
	}

	if err := c.store.WriteExternalEvent(ctx, store.ExternalEvent{
		SessionID:     sessionID,
		EventSource:   "runtime",
		EventKind:     pollDuplicateEvent,
		CorrelationID: correlationID,
		Payload:       string(payload),
		ObservedAt:    now,
	}); err != nil {
		return Lifecycle{}, err
	}
	if err := c.store.WritePolicyDecision(ctx, store.PolicyDecision{
		SessionID:     sessionID,
		CorrelationID: correlationID,
		DecisionKind:  pollDedupeKind,
		Verdict:       "skip_duplicate",
		InputHash:     hashPollInput(payload),
		Detail:        string(payload),
		CreatedAt:     now,
	}); err != nil {
		return Lifecycle{}, err
	}

	updatedLifecycle, err := c.snapshotFromCheckpoint(ctx, cp)
	if err != nil {
		return Lifecycle{}, err
	}
	updatedLifecycle.DrivenBy = pollDedupeDrivenBy
	updatedLifecycle.Reason = reason
	if cp.State == string(fsm.StatePaused) {
		updatedLifecycle.ResumeState = cp.ResumeState
	}
	return updatedLifecycle, nil
}

func (c *Controller) writePollEvaluation(ctx context.Context, record store.RuntimeControlRecord) error {
	if writer, ok := c.store.(store.RuntimeControlWriter); ok {
		return writer.WriteRuntimeControl(ctx, record)
	}
	if err := c.store.WriteCheckpointAndAction(ctx, record.Checkpoint, record.Action); err != nil {
		return err
	}
	if err := c.store.WriteExternalEvent(ctx, record.ExternalEvent); err != nil {
		return err
	}
	return c.store.WritePolicyDecision(ctx, record.PolicyDecision)
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
