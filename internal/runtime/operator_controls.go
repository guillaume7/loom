package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/guillaume7/loom/internal/fsm"
	"github.com/guillaume7/loom/internal/store"
)

const (
	ManualOverridePause        = "pause"
	ManualOverrideResume       = "resume"
	manualOverrideEventSource  = "operator"
	manualOverrideDecisionKind = "operator_override"
	manualOverrideLimit        = 50
)

var (
	ErrNothingToPause      = errors.New("nothing to pause")
	ErrNothingToResume      = errors.New("nothing to resume")
	errInvalidManualControl = errors.New("invalid manual override action")
)

type ManualOverrideRequest struct {
	Action        string
	Source        string
	RequestedBy   string
	Reason        string
	CorrelationID string
	OperationKey  string
	At            time.Time
}

type manualOverrideDetail struct {
	Action        string `json:"action"`
	Source        string `json:"source,omitempty"`
	RequestedBy   string `json:"requested_by,omitempty"`
	Reason        string `json:"reason,omitempty"`
	SessionID     string `json:"session_id"`
	CorrelationID string `json:"correlation_id"`
	PreviousState string `json:"previous_state,omitempty"`
	NewState      string `json:"new_state,omitempty"`
	ResumeState   string `json:"resume_state,omitempty"`
}

func (c *Controller) ApplyManualOverride(ctx context.Context, request ManualOverrideRequest) (Lifecycle, error) {
	action := strings.TrimSpace(request.Action)
	if action != ManualOverridePause && action != ManualOverrideResume {
		return Lifecycle{}, fmt.Errorf("%w: %q", errInvalidManualControl, request.Action)
	}

	now := request.At.UTC()
	if now.IsZero() {
		now = c.cfg.Now().UTC()
	}

	cp, err := c.store.ReadCheckpoint(ctx)
	if err != nil {
		return Lifecycle{}, err
	}
	sessionID := RunIdentifier(cp)

	switch action {
	case ManualOverridePause:
		return c.applyPauseOverride(ctx, cp, sessionID, request, now)
	case ManualOverrideResume:
		return c.applyResumeOverride(ctx, cp, sessionID, request, now)
	default:
		return Lifecycle{}, fmt.Errorf("%w: %q", errInvalidManualControl, request.Action)
	}
}

func (c *Controller) applyPauseOverride(ctx context.Context, cp store.Checkpoint, sessionID string, request ManualOverrideRequest, now time.Time) (Lifecycle, error) {
	resumeState := resumableState(cp)
	if resumeState == "" {
		return Lifecycle{}, ErrNothingToPause
	}

	correlationID := strings.TrimSpace(request.CorrelationID)
	if correlationID == "" {
		correlationID = manualOverrideCorrelationID(sessionID, ManualOverridePause, now)
	}

	nextCheckpoint := cp
	nextCheckpoint.State = string(fsm.StatePaused)
	nextCheckpoint.ResumeState = resumeState
	nextCheckpoint.UpdatedAt = now

	record, err := manualOverrideRecord(nextCheckpoint, sessionID, request, correlationID, cp.State, nextCheckpoint.State, nextCheckpoint.ResumeState, now)
	if err != nil {
		return Lifecycle{}, err
	}
	if err := c.writeManualOverride(ctx, record); err != nil {
		return Lifecycle{}, err
	}

	return c.snapshotFromCheckpoint(ctx, nextCheckpoint)
}

func (c *Controller) applyResumeOverride(ctx context.Context, cp store.Checkpoint, sessionID string, request ManualOverrideRequest, now time.Time) (Lifecycle, error) {
	if cp.State != string(fsm.StatePaused) {
		return Lifecycle{}, ErrNothingToResume
	}

	resumeState, err := inferResumeState(ctx, c.store, cp, sessionID)
	if err != nil {
		return Lifecycle{}, err
	}

	correlationID := strings.TrimSpace(request.CorrelationID)
	if correlationID == "" {
		correlationID, err = latestPauseCorrelationID(ctx, c.store, sessionID)
		if err != nil {
			return Lifecycle{}, err
		}
		if correlationID == "" {
			correlationID = manualOverrideCorrelationID(sessionID, ManualOverrideResume, now)
		}
	}

	nextCheckpoint := cp
	nextCheckpoint.State = resumeState
	nextCheckpoint.ResumeState = ""
	nextCheckpoint.UpdatedAt = now

	lifecycle, err := c.startWithCheckpoint(ctx, nextCheckpoint)
	if err != nil {
		c.releaseLease(ctx, nextCheckpoint, now)
		return Lifecycle{}, err
	}

	record, err := manualOverrideRecord(nextCheckpoint, sessionID, request, correlationID, cp.State, nextCheckpoint.State, resumeState, now)
	if err != nil {
		c.releaseLease(ctx, nextCheckpoint, now)
		return Lifecycle{}, err
	}
	if err := c.writeManualOverride(ctx, record); err != nil {
		c.releaseLease(ctx, nextCheckpoint, now)
		return Lifecycle{}, err
	}

	return lifecycle, nil
}

func (c *Controller) writeManualOverride(ctx context.Context, record store.RuntimeControlRecord) error {
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

func manualOverrideRecord(nextCheckpoint store.Checkpoint, sessionID string, request ManualOverrideRequest, correlationID string, previousState string, newState string, resumeState string, now time.Time) (store.RuntimeControlRecord, error) {
	detail := manualOverrideDetail{
		Action:        strings.TrimSpace(request.Action),
		Source:        strings.TrimSpace(request.Source),
		RequestedBy:   strings.TrimSpace(request.RequestedBy),
		Reason:        strings.TrimSpace(request.Reason),
		SessionID:     sessionID,
		CorrelationID: correlationID,
		PreviousState: previousState,
		NewState:      newState,
		ResumeState:   resumeState,
	}
	payload, err := json.Marshal(detail)
	if err != nil {
		return store.RuntimeControlRecord{}, err
	}

	action := strings.TrimSpace(request.Action)
	operationKey := strings.TrimSpace(request.OperationKey)
	if operationKey == "" {
		operationKey = fmt.Sprintf("manual_override:%s:%s:%d", sessionID, action, now.UnixNano())
	}
	eventKind := fmt.Sprintf("manual_override.%s", action)
	detailText := string(payload)

	return store.RuntimeControlRecord{
		Checkpoint: nextCheckpoint,
		Action: store.Action{
			SessionID:    sessionID,
			OperationKey: operationKey,
			StateBefore:  previousState,
			StateAfter:   newState,
			Event:        strings.ReplaceAll(eventKind, ".", "_"),
			Detail:       detailText,
			CreatedAt:    now,
		},
		ExternalEvent: store.ExternalEvent{
			SessionID:     sessionID,
			EventSource:   manualOverrideEventSource,
			EventKind:     eventKind,
			CorrelationID: correlationID,
			Payload:       detailText,
			ObservedAt:    now,
		},
		PolicyDecision: store.PolicyDecision{
			SessionID:     sessionID,
			CorrelationID: correlationID,
			DecisionKind:  manualOverrideDecisionKind,
			Verdict:       action,
			InputHash:     fmt.Sprintf("manual_override:%s:%s:%s", sessionID, action, correlationID),
			Detail:        detailText,
			CreatedAt:     now,
		},
	}, nil
}

func latestPauseCorrelationID(ctx context.Context, st store.Store, sessionID string) (string, error) {
	decisions, err := st.ReadPolicyDecisions(ctx, sessionID, manualOverrideLimit)
	if err != nil {
		return "", err
	}
	for _, decision := range decisions {
		if decision.DecisionKind == manualOverrideDecisionKind && decision.Verdict == ManualOverridePause && decision.CorrelationID != "" {
			return decision.CorrelationID, nil
		}
	}

	events, err := st.ReadExternalEvents(ctx, sessionID, manualOverrideLimit)
	if err != nil {
		return "", err
	}
	for _, event := range events {
		if event.EventSource == manualOverrideEventSource && event.EventKind == "manual_override.pause" && event.CorrelationID != "" {
			return event.CorrelationID, nil
		}
	}
	return "", nil
}

func inferResumeState(ctx context.Context, st store.Store, cp store.Checkpoint, sessionID string) (string, error) {
	if cp.ResumeState != "" && cp.ResumeState != string(fsm.StatePaused) {
		return cp.ResumeState, nil
	}

	actions, err := st.ReadActions(ctx, 200)
	if err != nil {
		return "", err
	}
	if len(actions) == 0 {
		return "", errors.New("paused checkpoint has no resume state and no action history to infer from")
	}

	if state := inferResumeStateFromActions(actions, sessionID, true); state != "" {
		return state, nil
	}
	if state := inferResumeStateFromActions(actions, sessionID, false); state != "" {
		return state, nil
	}

	return "", errors.New("paused checkpoint has no resumable state")
}

func inferResumeStateFromActions(actions []store.Action, sessionID string, requireSessionMatch bool) string {
	for _, action := range actions {
		if requireSessionMatch && sessionID != "" && action.SessionID != sessionID {
			continue
		}
		if action.StateAfter == string(fsm.StatePaused) {
			if action.StateBefore != "" && action.StateBefore != string(fsm.StatePaused) {
				return action.StateBefore
			}
			continue
		}
		if action.StateAfter != "" {
			return action.StateAfter
		}
	}
	return ""
}

func resumableState(cp store.Checkpoint) string {
	if cp.State == string(fsm.StatePaused) {
		return cp.ResumeState
	}
	return cp.State
}

func manualOverrideCorrelationID(sessionID string, action string, now time.Time) string {
	if sessionID == "" {
		sessionID = defaultRunIdentifier
	}
	return fmt.Sprintf("manual:%s:%s:%d", sessionID, action, now.UnixNano())
}

func (c *Controller) releaseLease(ctx context.Context, cp store.Checkpoint, now time.Time) {
	lease, err := c.store.ReadRuntimeLease(ctx, LeaseKey(cp))
	if err != nil {
		return
	}
	if lease.HolderID != c.cfg.HolderID {
		return
	}
	lease.ExpiresAt = now
	lease.RenewedAt = now
	_ = c.store.UpsertRuntimeLease(ctx, lease)
}