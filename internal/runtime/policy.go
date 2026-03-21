package runtime

import (
	"strings"
	"time"
)

type PolicyDecisionName string

const (
	PolicyDecisionCIReadiness     PolicyDecisionName = "ci_readiness"
	PolicyDecisionReviewReadiness PolicyDecisionName = "review_readiness"
	PolicyDecisionMergeReadiness  PolicyDecisionName = "merge_readiness"
)

type PolicyOutcome string

const (
	PolicyOutcomeContinue PolicyOutcome = "continue"
	PolicyOutcomeWait     PolicyOutcome = "wait"
	PolicyOutcomeRetry    PolicyOutcome = "retry"
	PolicyOutcomeEscalate PolicyOutcome = "escalate"
	PolicyOutcomeBlock    PolicyOutcome = "block"
)

const DefaultObservationStaleAfter = 2 * DefaultPollInterval

type PolicyEvaluation struct {
	Name    PolicyDecisionName
	Outcome PolicyOutcome
	Reason  string
}

type CIReadinessInput struct {
	Checkpoint  CheckpointObservation
	Observation *CIObservation
	EvaluatedAt time.Time
	StaleAfter  time.Duration
}

type ReviewReadinessInput struct {
	Checkpoint  CheckpointObservation
	Observation *ReviewObservation
	EvaluatedAt time.Time
	StaleAfter  time.Duration
}

type MergeReadinessInput struct {
	Model       ObservationModel
	EvaluatedAt time.Time
	StaleAfter  time.Duration
}

// EvaluateCIReadiness determines if CI checks are ready to allow progression.
func EvaluateCIReadiness(input CIReadinessInput) PolicyEvaluation {
	if input.Observation == nil {
		return PolicyEvaluation{Name: PolicyDecisionCIReadiness, Outcome: PolicyOutcomeRetry, Reason: "ci_observation_missing"}
	}
	if observationStale(input.Observation.ObservedAt, input.EvaluatedAt, input.StaleAfter) {
		return PolicyEvaluation{Name: PolicyDecisionCIReadiness, Outcome: PolicyOutcomeRetry, Reason: "ci_observation_stale"}
	}
	if ciObservationFailed(*input.Observation) {
		return PolicyEvaluation{Name: PolicyDecisionCIReadiness, Outcome: PolicyOutcomeEscalate, Reason: "ci_failed"}
	}
	if ciObservationSucceeded(*input.Observation) {
		return PolicyEvaluation{Name: PolicyDecisionCIReadiness, Outcome: PolicyOutcomeContinue, Reason: "ci_green"}
	}
	return PolicyEvaluation{Name: PolicyDecisionCIReadiness, Outcome: PolicyOutcomeWait, Reason: "ci_pending"}
}

// EvaluateReviewReadiness determines if the code review is ready to allow progression.
func EvaluateReviewReadiness(input ReviewReadinessInput) PolicyEvaluation {
	if input.Observation == nil {
		return PolicyEvaluation{Name: PolicyDecisionReviewReadiness, Outcome: PolicyOutcomeRetry, Reason: "review_observation_missing"}
	}
	if observationStale(input.Observation.ObservedAt, input.EvaluatedAt, input.StaleAfter) {
		return PolicyEvaluation{Name: PolicyDecisionReviewReadiness, Outcome: PolicyOutcomeRetry, Reason: "review_observation_stale"}
	}

	switch strings.ToUpper(strings.TrimSpace(input.Observation.Status)) {
	case "APPROVED":
		return PolicyEvaluation{Name: PolicyDecisionReviewReadiness, Outcome: PolicyOutcomeContinue, Reason: "review_approved"}
	case "CHANGES_REQUESTED":
		return PolicyEvaluation{Name: PolicyDecisionReviewReadiness, Outcome: PolicyOutcomeBlock, Reason: "changes_requested"}
	default:
		return PolicyEvaluation{Name: PolicyDecisionReviewReadiness, Outcome: PolicyOutcomeWait, Reason: "review_pending"}
	}
}

// EvaluateMergeReadiness determines if both CI and review evidence are present and current enough to allow merging.
func EvaluateMergeReadiness(input MergeReadinessInput) PolicyEvaluation {
	ciObservation, ciObservedAt, ciFresh := latestCIState(input.Model, input.EvaluatedAt, input.StaleAfter)
	reviewObservation, reviewObservedAt, reviewFresh := latestReviewState(input.Model, input.EvaluatedAt, input.StaleAfter)

	if ciObservedAt.IsZero() || reviewObservedAt.IsZero() {
		return PolicyEvaluation{Name: PolicyDecisionMergeReadiness, Outcome: PolicyOutcomeBlock, Reason: "merge_observations_incomplete"}
	}
	if !ciFresh || !reviewFresh {
		return PolicyEvaluation{Name: PolicyDecisionMergeReadiness, Outcome: PolicyOutcomeBlock, Reason: "merge_observations_stale"}
	}
	if ciObservation != "success" {
		if ciObservation == "pending" {
			return PolicyEvaluation{Name: PolicyDecisionMergeReadiness, Outcome: PolicyOutcomeWait, Reason: "merge_waiting_on_ci"}
		}
		return PolicyEvaluation{Name: PolicyDecisionMergeReadiness, Outcome: PolicyOutcomeBlock, Reason: "merge_blocked_by_ci"}
	}

	switch reviewObservation {
	case "approved":
		return PolicyEvaluation{Name: PolicyDecisionMergeReadiness, Outcome: PolicyOutcomeContinue, Reason: "merge_ready"}
	case "pending":
		return PolicyEvaluation{Name: PolicyDecisionMergeReadiness, Outcome: PolicyOutcomeWait, Reason: "merge_waiting_on_review"}
	default:
		return PolicyEvaluation{Name: PolicyDecisionMergeReadiness, Outcome: PolicyOutcomeBlock, Reason: "merge_blocked_by_review"}
	}
}

func observationStale(observedAt, evaluatedAt time.Time, staleAfter time.Duration) bool {
	if observedAt.IsZero() {
		return true
	}
	if staleAfter <= 0 {
		staleAfter = DefaultObservationStaleAfter
	}
	if evaluatedAt.IsZero() {
		evaluatedAt = observedAt
	}
	return evaluatedAt.Sub(observedAt) > staleAfter
}

func ciObservationSucceeded(observation CIObservation) bool {
	conclusion := normalizeCIConclusion(observation.Conclusion, observation)
	return conclusion == "success"
}

func ciObservationFailed(observation CIObservation) bool {
	conclusion := normalizeCIConclusion(observation.Conclusion, observation)
	switch conclusion {
	case "failure", "error", "cancelled", "timed_out":
		return true
	default:
		return false
	}
}

func normalizeCIConclusion(conclusion string, observation CIObservation) string {
	normalized := strings.ToLower(strings.TrimSpace(conclusion))
	if normalized != "" {
		return normalized
	}
	if len(observation.FailedChecks) > 0 {
		return "failure"
	}
	if observation.TotalChecks > 0 && observation.GreenChecks == observation.TotalChecks {
		return "success"
	}
	if len(observation.PendingChecks) > 0 || observation.TotalChecks == 0 {
		return "pending"
	}
	return "pending"
}

func latestCIState(model ObservationModel, evaluatedAt time.Time, staleAfter time.Duration) (string, time.Time, bool) {
	if len(model.CI) > 0 {
		observation := model.CI[0]
		return normalizeCIConclusion(observation.Conclusion, observation), observation.ObservedAt, !observationStale(observation.ObservedAt, evaluatedAt, staleAfter)
	}
	if model.Summaries.CI != nil {
		return strings.ToLower(strings.TrimSpace(model.Summaries.CI.Conclusion)), model.Summaries.CI.RecordedAt, !observationStale(model.Summaries.CI.RecordedAt, evaluatedAt, staleAfter)
	}
	return "", time.Time{}, false
}

func latestReviewState(model ObservationModel, evaluatedAt time.Time, staleAfter time.Duration) (string, time.Time, bool) {
	if len(model.Review) > 0 {
		observation := model.Review[0]
		return normalizeReviewStatus(observation.Status), observation.ObservedAt, !observationStale(observation.ObservedAt, evaluatedAt, staleAfter)
	}
	if model.Summaries.Review != nil {
		return normalizeReviewStatus(model.Summaries.Review.Status), model.Summaries.Review.RecordedAt, !observationStale(model.Summaries.Review.RecordedAt, evaluatedAt, staleAfter)
	}
	return "", time.Time{}, false
}

func normalizeReviewStatus(status string) string {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "APPROVED":
		return "approved"
	case "CHANGES_REQUESTED":
		return "changes_requested"
	default:
		return "pending"
	}
}
