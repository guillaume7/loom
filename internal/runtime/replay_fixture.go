package runtime

import (
	"context"
	"encoding/json"
	"math"
	"time"

	"github.com/guillaume7/loom/internal/store"
)

const readAllReplayFixtureRecordsLimit = math.MaxInt

// ReplayFixture captures the persisted inputs needed to replay a runtime branch deterministically.
type ReplayFixture struct {
	SessionID    string                 `json:"session_id"`
	CapturedAt   time.Time              `json:"captured_at"`
	Checkpoint   store.Checkpoint       `json:"checkpoint"`
	Observations ObservationModel       `json:"observations"`
	Policies     PolicyAuditReport      `json:"policies"`
	Events       []store.ExternalEvent  `json:"events"`
	Decisions    []store.PolicyDecision `json:"decisions"`
	Actions      []store.Action         `json:"actions"`
}

// AssembleReplayFixture returns a deterministic replay fixture for the current session.
func AssembleReplayFixture(ctx context.Context, st store.Store, now time.Time) (ReplayFixture, error) {
	cp, err := st.ReadCheckpoint(ctx)
	if err != nil {
		return ReplayFixture{}, err
	}

	sessionID := RunIdentifier(cp)

	observations, err := assembleObservationModelWithLimit(ctx, st, readAllReplayFixtureRecordsLimit)
	if err != nil {
		return ReplayFixture{}, err
	}

	policies, err := assemblePolicyAuditReportWithLimit(ctx, st, readAllReplayFixtureRecordsLimit)
	if err != nil {
		return ReplayFixture{}, err
	}

	events, err := st.ReadExternalEvents(ctx, sessionID, readAllReplayFixtureRecordsLimit)
	if err != nil {
		return ReplayFixture{}, err
	}

	decisions, err := st.ReadPolicyDecisions(ctx, sessionID, readAllReplayFixtureRecordsLimit)
	if err != nil {
		return ReplayFixture{}, err
	}

	actions, err := st.ReadActions(ctx, readAllReplayFixtureRecordsLimit)
	if err != nil {
		return ReplayFixture{}, err
	}

	filteredActions := make([]store.Action, 0, len(actions))
	for _, action := range actions {
		if action.SessionID != sessionID {
			continue
		}
		filteredActions = append(filteredActions, action)
	}

	return ReplayFixture{
		SessionID:    sessionID,
		CapturedAt:   now.UTC(),
		Checkpoint:   cp,
		Observations: observations,
		Policies:     policies,
		Events:       events,
		Decisions:    decisions,
		Actions:      filteredActions,
	}, nil
}

// MarshalReplayFixture serializes the fixture as stable indented JSON for debugging and regression tests.
func MarshalReplayFixture(fixture ReplayFixture) ([]byte, error) {
	return json.MarshalIndent(fixture, "", "  ")
}
