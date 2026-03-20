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

func (s ciPollSummary) allGreen() bool {
	return s.TotalChecks > 0 && s.GreenChecks == s.TotalChecks
}

type pollObservation struct {
	SessionID            string        `json:"session_id"`
	CorrelationID        string        `json:"correlation_id"`
	WakeKind             string        `json:"wake_kind"`
	PreviousState        string        `json:"previous_state"`
	NewState             string        `json:"new_state"`
	DecisionVerdict      string        `json:"decision_verdict"`
	ActionEvent          string        `json:"action_event"`
	Outcome              string        `json:"outcome"`
	PRNumber             int           `json:"pr_number,omitempty"`
	RetryCount           int           `json:"retry_count"`
	ResumeState          string        `json:"resume_state,omitempty"`
	ReviewStatus         string        `json:"review_status,omitempty"`
	Draft                *bool         `json:"draft,omitempty"`
	ForcedReadyForReview bool          `json:"forced_ready_for_review,omitempty"`
	CI                   ciPollSummary `json:"ci,omitempty"`
}

type pollEvaluation struct {
	record   store.RuntimeControlRecord
	next     store.Checkpoint
	nextWake store.WakeSchedule
	verdict  string
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

	now := c.cfg.Now().UTC()
	evaluation, err := c.evaluateDueWake(ctx, cp, gh, now, lifecycle.NextWakeKind)
	if err != nil {
		return Lifecycle{}, err
	}
	if err := c.writePollEvaluation(ctx, evaluation.record); err != nil {
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

func (c *Controller) evaluateDueWake(ctx context.Context, cp store.Checkpoint, gh PollingClient, now time.Time, wakeKind string) (pollEvaluation, error) {
	sessionID := RunIdentifier(cp)
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
		if strings.TrimSpace(pr.HeadSHA) == "" {
			return pollEvaluation{}, ErrMissingPollHeadSHA
		}
		summary, err := collectCIPollSummary(ctx, gh, pr.HeadSHA)
		if err != nil {
			return pollEvaluation{}, err
		}
		observation.CI = summary
		if summary.allGreen() {
			observation.Outcome = "ci_green"
			observation.ActionEvent = string(fsm.EventCIGreen)
			observation.DecisionVerdict = "resume"
			nextCheckpoint.State = string(fsm.StateReviewing)
			nextCheckpoint.ResumeState = ""
			nextCheckpoint.RetryCount = 0
			break
		}
		if len(summary.FailedChecks) > 0 {
			observation.Outcome = "ci_failed"
			observation.ActionEvent = string(fsm.EventCIRed)
			observation.DecisionVerdict = "debug"
			nextCheckpoint.State = string(fsm.StateDebugging)
			nextCheckpoint.ResumeState = ""
			nextCheckpoint.RetryCount = 0
			break
		}

		nextRetryCount := cp.RetryCount + 1
		observation.Outcome = "ci_pending"
		observation.ActionEvent = string(fsm.EventTimeout)
		if nextRetryCount > fsm.DefaultConfig().MaxRetriesAwaitingCI {
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

	case fsm.StateReviewing:
		if cp.PRNumber <= 0 {
			return pollEvaluation{}, ErrMissingPollPRNumber
		}
		status, err := gh.GetReviewStatus(ctx, cp.PRNumber)
		if err != nil {
			return pollEvaluation{}, err
		}
		normalizedStatus := strings.ToUpper(strings.TrimSpace(status))
		observation.ReviewStatus = normalizedStatus
		nextCheckpoint.ResumeState = ""
		nextCheckpoint.RetryCount = 0
		switch normalizedStatus {
		case "APPROVED":
			observation.Outcome = "review_approved"
			observation.ActionEvent = string(fsm.EventReviewApproved)
			observation.DecisionVerdict = "resume"
			nextCheckpoint.State = string(fsm.StateMerging)
		case "CHANGES_REQUESTED":
			observation.Outcome = "changes_requested"
			observation.ActionEvent = string(fsm.EventReviewChangesRequested)
			observation.DecisionVerdict = "address_feedback"
			nextCheckpoint.State = string(fsm.StateAddressingFeedback)
		default:
			observation.Outcome = "review_pending"
			observation.ActionEvent = pollWaitEvent
			observation.DecisionVerdict = "await"
			nextCheckpoint.State = cp.State
		}

	default:
		return pollEvaluation{}, fmt.Errorf("%w: %s", ErrUnsupportedPollResume, cp.State)
	}

	observation.NewState = nextCheckpoint.State
	observation.ResumeState = nextCheckpoint.ResumeState
	payload, err := json.Marshal(observation)
	if err != nil {
		return pollEvaluation{}, err
	}

	operationKey := fmt.Sprintf("poll_resume:%s:%s:%d", sessionID, wakeKind, now.UnixNano())
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
			DecisionKind:  pollDecisionKind,
			Verdict:       observation.DecisionVerdict,
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
