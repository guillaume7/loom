package runtime_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/guillaume7/loom/internal/runtime"
	"github.com/guillaume7/loom/internal/store"
)

func TestAssemblePolicyAuditReport_ReturnsChronologicalEntriesForSession(t *testing.T) {
	ctx := context.Background()
	ms := newMemStore()
	cp := store.Checkpoint{State: "awaiting_ready", PRNumber: 123}
	if err := ms.WriteCheckpoint(ctx, cp); err != nil {
		t.Fatalf("WriteCheckpoint failed: %v", err)
	}
	now := time.Now().UTC()
	ciObservation := map[string]interface{}{
		"session_id":         "default",
		"correlation_id":     "poll:default:poll_ci:1000",
		"wake_kind":          "poll_ci",
		"policy_decision":    "ci_readiness",
		"policy_outcome":     "continue",
		"policy_reason":      "ci_green",
		"previous_state":     "awaiting_ready",
		"new_state":          "awaiting_ready",
		"decision_verdict":   "continue",
		"retry_count":        0,
	}
	ciDetail, _ := json.Marshal(ciObservation)
	ciDecision := store.PolicyDecision{
		SessionID:     "default",
		CorrelationID: "poll:default:poll_ci:1000",
		DecisionKind:  "ci_readiness",
		Verdict:       "continue",
		InputHash:     "abc123",
		Detail:        string(ciDetail),
		CreatedAt:     now.Add(-2 * time.Second),
	}
	if err := ms.WritePolicyDecision(ctx, ciDecision); err != nil {
		t.Fatalf("WritePolicyDecision (CI) failed: %v", err)
	}
	dedupeObservation := map[string]interface{}{
		"session_id":     "default",
		"correlation_id": "poll:default:dedupe:2000",
	}
	dedupeDetail, _ := json.Marshal(dedupeObservation)
	dedupeDecision := store.PolicyDecision{
		SessionID:     "default",
		CorrelationID: "poll:default:dedupe:2000",
		DecisionKind:  "runtime_resume_dedupe",
		Verdict:       "skipped",
		InputHash:     "def456",
		Detail:        string(dedupeDetail),
		CreatedAt:     now.Add(-1 * time.Second),
	}
	if err := ms.WritePolicyDecision(ctx, dedupeDecision); err != nil {
		t.Fatalf("WritePolicyDecision (dedupe) failed: %v", err)
	}
	reviewObservation := map[string]interface{}{
		"session_id":         "default",
		"correlation_id":     "poll:default:poll_review:3000",
		"wake_kind":          "poll_review",
		"policy_decision":    "review_readiness",
		"policy_outcome":     "wait",
		"policy_reason":      "review_pending",
		"previous_state":     "awaiting_ready",
		"new_state":          "awaiting_ready",
		"decision_verdict":   "wait",
		"retry_count":        0,
	}
	reviewDetail, _ := json.Marshal(reviewObservation)
	reviewDecision := store.PolicyDecision{
		SessionID:     "default",
		CorrelationID: "poll:default:poll_review:3000",
		DecisionKind:  "review_readiness",
		Verdict:       "wait",
		InputHash:     "ghi789",
		Detail:        string(reviewDetail),
		CreatedAt:     now,
	}
	if err := ms.WritePolicyDecision(ctx, reviewDecision); err != nil {
		t.Fatalf("WritePolicyDecision (review) failed: %v", err)
	}
	report, err := runtime.AssemblePolicyAuditReport(ctx, ms)
	if err != nil {
		t.Fatalf("AssemblePolicyAuditReport failed: %v", err)
	}

	// Verify report session ID
	if report.SessionID != "default" {
		t.Errorf("Expected SessionID 'default', got %q", report.SessionID)
	}

	// Should have 2 entries (CI and review), skipping dedupe
	if len(report.Entries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(report.Entries))
	}

	// Verify chronological order (oldest first)
	if len(report.Entries) >= 1 {
		if report.Entries[0].CorrelationID != "poll:default:poll_ci:1000" {
			t.Errorf("First entry should be CI, got %q", report.Entries[0].CorrelationID)
		}
		if !report.Entries[0].RecordedAt.Equal(now.Add(-2 * time.Second)) {
			t.Errorf("First entry timestamp mismatch")
		}
	}

	if len(report.Entries) >= 2 {
		if report.Entries[1].CorrelationID != "poll:default:poll_review:3000" {
			t.Errorf("Second entry should be review, got %q", report.Entries[1].CorrelationID)
		}
		if !report.Entries[1].RecordedAt.Equal(now) {
			t.Errorf("Second entry timestamp mismatch")
		}
	}
}

func TestAssemblePolicyAuditReport_PopulatesAllFields(t *testing.T) {
	ctx := context.Background()
	ms := newMemStore()

	// Write checkpoint
	cp := store.Checkpoint{State: "awaiting_ready", PRNumber: 456}
	if err := ms.WriteCheckpoint(ctx, cp); err != nil {
		t.Fatalf("WriteCheckpoint failed: %v", err)
	}

	now := time.Now().UTC()

	// Create a merge decision with both policy and merge policy reasons
	observation := map[string]interface{}{
		"session_id":            "default",
		"correlation_id":        "poll:default:poll_merge:5000",
		"wake_kind":             "poll_merge",
		"policy_decision":       "merge_readiness",
		"policy_outcome":        "continue",
		"policy_reason":         "primary_ready",
		"merge_policy_decision": "merge_gate",
		"merge_policy_outcome":  "continue",
		"merge_policy_reason":   "all_checks_passed",
		"previous_state":        "awaiting_ready",
		"new_state":             "merging",
		"decision_verdict":      "continue",
		"retry_count":           2,
	}
	detail, _ := json.Marshal(observation)
	testHash := "test_input_hash_abc123def456"

	decision := store.PolicyDecision{
		SessionID:     "default",
		CorrelationID: "poll:default:poll_merge:5000",
		DecisionKind:  "merge_readiness",
		Verdict:       "continue",
		InputHash:     testHash,
		Detail:        string(detail),
		CreatedAt:     now,
	}

	if err := ms.WritePolicyDecision(ctx, decision); err != nil {
		t.Fatalf("WritePolicyDecision failed: %v", err)
	}

	// Assemble the audit report
	report, err := runtime.AssemblePolicyAuditReport(ctx, ms)
	if err != nil {
		t.Fatalf("AssemblePolicyAuditReport failed: %v", err)
	}

	if len(report.Entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(report.Entries))
	}

	entry := report.Entries[0]

	// Verify all fields
	if entry.SessionID != "default" {
		t.Errorf("SessionID: expected 'default', got %q", entry.SessionID)
	}

	if entry.CorrelationID != "poll:default:poll_merge:5000" {
		t.Errorf("CorrelationID: expected 'poll:default:poll_merge:5000', got %q", entry.CorrelationID)
	}

	if entry.DecisionKind != "merge_readiness" {
		t.Errorf("DecisionKind: expected 'merge_readiness', got %q", entry.DecisionKind)
	}

	if entry.Verdict != "continue" {
		t.Errorf("Verdict: expected 'continue', got %q", entry.Verdict)
	}

	// Reason should include both policy and merge reasons
	expectedReason := "primary_ready (merge: all_checks_passed)"
	if entry.Reason != expectedReason {
		t.Errorf("Reason: expected %q, got %q", expectedReason, entry.Reason)
	}

	if entry.InputHash != testHash {
		t.Errorf("InputHash: expected %q, got %q", testHash, entry.InputHash)
	}

	if entry.StateBefore != "awaiting_ready" {
		t.Errorf("StateBefore: expected 'awaiting_ready', got %q", entry.StateBefore)
	}

	if entry.StateAfter != "merging" {
		t.Errorf("StateAfter: expected 'merging', got %q", entry.StateAfter)
	}

	if !entry.RecordedAt.Equal(now) {
		t.Errorf("RecordedAt timestamp mismatch: expected %v, got %v", now, entry.RecordedAt)
	}
}
