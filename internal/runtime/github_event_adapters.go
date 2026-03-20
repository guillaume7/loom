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

const githubEventSource = "github"

var ErrMissingGitHubEventType = errors.New("github event type is required")

type GitHubEvent struct {
	DeliveryID string          `json:"delivery_id,omitempty"`
	EventType  string          `json:"event_type"`
	Action     string          `json:"action,omitempty"`
	Repository string          `json:"repository,omitempty"`
	PRNumber   int             `json:"pr_number,omitempty"`
	Payload    json.RawMessage `json:"payload,omitempty"`
	ReceivedAt time.Time       `json:"received_at,omitempty"`
}

type GitHubEventAdapter struct {
	Name             string
	EventTypes       []string
	CheckpointStates []string
	WakeKind         string
	RequiresPRMatch  bool
}

type GitHubEventReceipt struct {
	SessionID        string
	CorrelationID    string
	ObservedAt       time.Time
	Adapter          string
	WakeScheduled    bool
	WakeKind         string
	WakeAt           time.Time
	FallbackWakeKind string
}

type githubEventObservation struct {
	SessionID        string          `json:"session_id"`
	CorrelationID    string          `json:"correlation_id"`
	DeliveryID       string          `json:"delivery_id,omitempty"`
	EventType        string          `json:"event_type"`
	Action           string          `json:"action,omitempty"`
	Repository       string          `json:"repository,omitempty"`
	PRNumber         int             `json:"pr_number,omitempty"`
	WorkflowState    string          `json:"workflow_state,omitempty"`
	Adapter          string          `json:"adapter,omitempty"`
	FallbackWakeKind string          `json:"fallback_wake_kind,omitempty"`
	Payload          json.RawMessage `json:"payload,omitempty"`
}

var defaultGitHubEventAdapters = []GitHubEventAdapter{
	{
		Name:             "pr_ready",
		EventTypes:       []string{"pull_request"},
		CheckpointStates: []string{string(fsm.StateAwaitingReady)},
		WakeKind:         "poll_pr_ready",
		RequiresPRMatch:  true,
	},
	{
		Name:             "ci",
		EventTypes:       []string{"check_run", "check_suite", "status"},
		CheckpointStates: []string{string(fsm.StateAwaitingCI)},
		WakeKind:         "poll_ci",
		RequiresPRMatch:  true,
	},
	{
		Name:             "review",
		EventTypes:       []string{"pull_request_review", "pull_request_review_thread"},
		CheckpointStates: []string{string(fsm.StateReviewing)},
		WakeKind:         "poll_review",
		RequiresPRMatch:  true,
	},
}

func DefaultGitHubEventAdapters() []GitHubEventAdapter {
	adapters := make([]GitHubEventAdapter, len(defaultGitHubEventAdapters))
	copy(adapters, defaultGitHubEventAdapters)
	return adapters
}

func (c *Controller) ObserveGitHubEvent(ctx context.Context, event GitHubEvent) (GitHubEventReceipt, error) {
	eventType := strings.TrimSpace(event.EventType)
	if eventType == "" {
		return GitHubEventReceipt{}, ErrMissingGitHubEventType
	}

	cp, err := c.store.ReadCheckpoint(ctx)
	if err != nil {
		return GitHubEventReceipt{}, err
	}

	observedAt := event.ReceivedAt.UTC()
	if observedAt.IsZero() {
		observedAt = c.cfg.Now().UTC()
	}

	sessionID := RunIdentifier(cp)
	fallbackWakeKind := inferWakeKind(cp.State)
	adapter, matched := resolveGitHubEventAdapter(cp, event)
	correlationID := githubEventCorrelationID(sessionID, event, observedAt)
	observation := githubEventObservation{
		SessionID:        sessionID,
		CorrelationID:    correlationID,
		DeliveryID:       strings.TrimSpace(event.DeliveryID),
		EventType:        eventType,
		Action:           strings.TrimSpace(event.Action),
		Repository:       strings.TrimSpace(event.Repository),
		PRNumber:         event.PRNumber,
		WorkflowState:    cp.State,
		FallbackWakeKind: fallbackWakeKind,
		Payload:          event.Payload,
	}
	if matched {
		observation.Adapter = adapter.Name
	}

	payload, err := json.Marshal(observation)
	if err != nil {
		return GitHubEventReceipt{}, err
	}

	if err := c.store.WriteExternalEvent(ctx, store.ExternalEvent{
		SessionID:     sessionID,
		EventSource:   githubEventSource,
		EventKind:     eventType,
		ExternalID:    strings.TrimSpace(event.DeliveryID),
		CorrelationID: correlationID,
		Payload:       string(payload),
		ObservedAt:    observedAt,
	}); err != nil {
		return GitHubEventReceipt{}, err
	}

	receipt := GitHubEventReceipt{
		SessionID:        sessionID,
		CorrelationID:    correlationID,
		ObservedAt:       observedAt,
		FallbackWakeKind: fallbackWakeKind,
	}
	if !matched {
		return receipt, nil
	}

	wake := store.WakeSchedule{
		SessionID: sessionID,
		WakeKind:  adapter.WakeKind,
		DueAt:     observedAt,
		DedupeKey: fmt.Sprintf("%s:%s", LeaseKey(cp), adapter.WakeKind),
		Payload:   string(payload),
	}
	if existingWake, ok, err := c.readWakeByDedupeKey(ctx, sessionID, wake.DedupeKey); err != nil {
		return receipt, err
	} else if ok {
		wake.ClaimedAt = existingWake.ClaimedAt
		wake.CreatedAt = existingWake.CreatedAt
	}
	if err := c.store.UpsertWakeSchedule(ctx, wake); err != nil {
		return receipt, err
	}

	receipt.Adapter = adapter.Name
	receipt.WakeScheduled = true
	receipt.WakeKind = adapter.WakeKind
	receipt.WakeAt = observedAt
	return receipt, nil
}

func resolveGitHubEventAdapter(cp store.Checkpoint, event GitHubEvent) (GitHubEventAdapter, bool) {
	eventType := strings.ToLower(strings.TrimSpace(event.EventType))
	state := strings.TrimSpace(cp.State)
	for _, adapter := range defaultGitHubEventAdapters {
		if !containsFold(adapter.EventTypes, eventType) {
			continue
		}
		if !containsFold(adapter.CheckpointStates, state) {
			continue
		}
		if adapter.RequiresPRMatch {
			if cp.PRNumber <= 0 || event.PRNumber <= 0 || cp.PRNumber != event.PRNumber {
				continue
			}
		}
		return adapter, true
	}
	return GitHubEventAdapter{}, false
}

func githubEventCorrelationID(sessionID string, event GitHubEvent, observedAt time.Time) string {
	deliveryID := strings.TrimSpace(event.DeliveryID)
	if deliveryID == "" {
		deliveryID = fmt.Sprintf("%s:%d", strings.TrimSpace(event.EventType), observedAt.UnixNano())
	}
	return fmt.Sprintf("github:%s:%s", sessionID, deliveryID)
}

func containsFold(values []string, want string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), want) {
			return true
		}
	}
	return false
}

func (c *Controller) readWakeByDedupeKey(ctx context.Context, sessionID, dedupeKey string) (store.WakeSchedule, bool, error) {
	wakes, err := c.store.ReadWakeSchedules(ctx, sessionID, defaultWakeScanLimit)
	if err != nil {
		return store.WakeSchedule{}, false, err
	}
	for _, wake := range wakes {
		if wake.DedupeKey == dedupeKey {
			return wake, true, nil
		}
	}
	return store.WakeSchedule{}, false, nil
}
