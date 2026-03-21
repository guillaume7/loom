package runtime

import (
	"context"
	"encoding/json"
	"time"

	"github.com/guillaume7/loom/internal/store"
)

const defaultPolicyAuditLimit = 1000

// PolicyAuditEntry is a decoded, human-readable view of a single persisted policy decision.
type PolicyAuditEntry struct {
	SessionID     string
	CorrelationID string
	DecisionKind  string // e.g. "ci_readiness", "review_readiness", "merge_readiness"
	Verdict       string // the policy outcome: "continue", "block", "wait", "retry", "escalate"
	Reason        string // human-readable reason: "ci_green", "review_approved", etc.
	InputHash     string // SHA-256 of the input snapshot for replay matching
	StateBefore   string // previous FSM state
	StateAfter    string // new FSM state
	RecordedAt    time.Time
}

// PolicyAuditReport groups policy audit entries for a single session run.
type PolicyAuditReport struct {
	SessionID string
	Entries   []PolicyAuditEntry
}

// AssemblePolicyAuditReport reads all policy decisions for the current session and returns them as a chronological audit report.
func AssemblePolicyAuditReport(ctx context.Context, st store.Store) (PolicyAuditReport, error) {
	return assemblePolicyAuditReportWithLimit(ctx, st, defaultPolicyAuditLimit)
}

func assemblePolicyAuditReportWithLimit(ctx context.Context, st store.Store, limit int) (PolicyAuditReport, error) {
	// Read the checkpoint to get the session ID
	cp, err := st.ReadCheckpoint(ctx)
	if err != nil {
		return PolicyAuditReport{}, err
	}

	sessionID := RunIdentifier(cp)

	decisions, err := st.ReadPolicyDecisions(ctx, sessionID, limit)
	if err != nil {
		return PolicyAuditReport{}, err
	}

	// Convert decisions to audit entries, skipping non-policy records
	var entries []PolicyAuditEntry
	for i := len(decisions) - 1; i >= 0; i-- {
		decision := decisions[i]
		entry, ok := decodePolicyDecision(decision)
		if !ok {
			continue
		}
		entries = append(entries, entry)
	}

	return PolicyAuditReport{
		SessionID: sessionID,
		Entries:   entries,
	}, nil
}

// decodePolicyDecision converts a store.PolicyDecision to a human-readable PolicyAuditEntry.
// Returns false for non-policy decision kinds like dedupe or operator overrides.
func decodePolicyDecision(decision store.PolicyDecision) (PolicyAuditEntry, bool) {
	// Skip non-policy decision kinds
	switch decision.DecisionKind {
	case "runtime_resume_dedupe", "operator_override":
		return PolicyAuditEntry{}, false
	}

	// Decode the pollObservation JSON from the Detail field
	var observation pollObservation
	if err := json.Unmarshal([]byte(decision.Detail), &observation); err != nil {
		return PolicyAuditEntry{}, false
	}

	// Determine the reason to display
	reason := observation.PolicyReason
	if observation.MergePolicyReason != "" {
		// For merge decisions, append the merge reason
		if reason != "" {
			reason = reason + " (merge: " + observation.MergePolicyReason + ")"
		} else {
			reason = observation.MergePolicyReason
		}
	}

	return PolicyAuditEntry{
		SessionID:     decision.SessionID,
		CorrelationID: decision.CorrelationID,
		DecisionKind:  decision.DecisionKind,
		Verdict:       decision.Verdict,
		Reason:        reason,
		InputHash:     decision.InputHash,
		StateBefore:   observation.PreviousState,
		StateAfter:    observation.NewState,
		RecordedAt:    decision.CreatedAt,
	}, true
}
