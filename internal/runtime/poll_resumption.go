package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
