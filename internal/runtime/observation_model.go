package runtime

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/guillaume7/loom/internal/store"
)

const defaultObservationLimit = 100

// AssembleObservationModel reads all persisted runtime records and assembles a typed observation model for policy evaluation.
func AssembleObservationModel(ctx context.Context, st store.Store) (ObservationModel, error) {
	cp, err := st.ReadCheckpoint(ctx)
	if err != nil {
		return ObservationModel{}, err
	}

	model := ObservationModel{
		SessionID: RunIdentifier(cp),
		Checkpoint: CheckpointObservation{
			Authority:   ObservationAuthorityAuthoritative,
			State:       cp.State,
			ResumeState: cp.ResumeState,
			Phase:       cp.Phase,
			PRNumber:    cp.PRNumber,
			IssueNumber: cp.IssueNumber,
			RetryCount:  cp.RetryCount,
			UpdatedAt:   cp.UpdatedAt.UTC(),
		},
	}

	events, err := st.ReadExternalEvents(ctx, model.SessionID, defaultObservationLimit)
	if err != nil {
		return ObservationModel{}, err
	}
	for _, event := range events {
		consumeObservationEvent(&model, event)
	}

	decisions, err := st.ReadPolicyDecisions(ctx, model.SessionID, defaultObservationLimit)
	if err != nil {
		return ObservationModel{}, err
	}
	for _, decision := range decisions {
		consumeObservationSummary(&model, decision)
	}

	if model.Summaries.CI == nil && len(model.CI) > 0 {
		latest := model.CI[0]
		model.Summaries.CI = &CISummary{
			Authority:     ObservationAuthorityDerived,
			CorrelationID: latest.CorrelationID,
			RecordedAt:    latest.ObservedAt,
			Conclusion:    latest.Conclusion,
			Verdict:       latest.Verdict,
			TotalChecks:   latest.TotalChecks,
			GreenChecks:   latest.GreenChecks,
		}
	}
	if model.Summaries.Review == nil && len(model.Review) > 0 {
		latest := model.Review[0]
		model.Summaries.Review = &ReviewSummary{
			Authority:     ObservationAuthorityDerived,
			CorrelationID: latest.CorrelationID,
			RecordedAt:    latest.ObservedAt,
			Status:        latest.Status,
			Verdict:       latest.Verdict,
		}
	}
	if model.Summaries.PR == nil && len(model.PR) > 0 {
		latest := model.PR[0]
		model.Summaries.PR = &PRSummary{
			Authority:     ObservationAuthorityDerived,
			CorrelationID: latest.CorrelationID,
			RecordedAt:    latest.ObservedAt,
			PRNumber:      latest.PRNumber,
			Draft:         latest.Draft,
			WorkflowState: latest.WorkflowState,
			Verdict:       latest.Verdict,
		}
	}
	if len(model.Branch) > 0 {
		latest := model.Branch[0]
		model.Summaries.Branch = &BranchSummary{
			Authority:     ObservationAuthorityDerived,
			CorrelationID: latest.CorrelationID,
			RecordedAt:    latest.ObservedAt,
			Repository:    latest.Repository,
			Ref:           latest.Ref,
			BaseRef:       latest.BaseRef,
			HeadRef:       latest.HeadRef,
			HeadSHA:       latest.HeadSHA,
		}
	}
	if model.Summaries.Operator == nil && len(model.Operator) > 0 {
		latest := model.Operator[0]
		model.Summaries.Operator = &OperatorSummary{
			Authority:     ObservationAuthorityDerived,
			CorrelationID: latest.CorrelationID,
			RecordedAt:    latest.ObservedAt,
			Action:        latest.Action,
			RequestedBy:   latest.RequestedBy,
			Reason:        latest.Reason,
		}
	}

	return model, nil
}

func consumeObservationEvent(model *ObservationModel, event store.ExternalEvent) {
	switch event.EventSource {
	case githubEventSource:
		consumeGitHubObservationEvent(model, event)
	case pollEventSource:
		consumePollObservationEvent(model, event)
	case manualOverrideEventSource:
		consumeOperatorObservationEvent(model, event)
	}
}

func consumeGitHubObservationEvent(model *ObservationModel, event store.ExternalEvent) {
	var observation githubEventObservation
	if err := json.Unmarshal([]byte(event.Payload), &observation); err != nil {
		return
	}
	if !isRelevantGitHubObservation(model, observation) {
		return
	}

	var payload githubPayloadSnapshot
	if len(observation.Payload) > 0 {
		_ = json.Unmarshal(observation.Payload, &payload)
	}

	repository := strings.TrimSpace(observation.Repository)
	if repository == "" {
		repository = strings.TrimSpace(payload.Repository.FullName)
	}
	if repository == "" {
		repository = strings.TrimSpace(payload.Repository.Name)
	}

	if branchObservation, ok := githubBranchObservation(event, observation, payload, repository); ok {
		model.Branch = append(model.Branch, branchObservation)
	}

	switch strings.ToLower(strings.TrimSpace(observation.EventType)) {
	case "check_run", "check_suite", "status":
		model.CI = append(model.CI, CIObservation{
			Authority:     ObservationAuthorityAuthoritative,
			Source:        event.EventSource,
			CorrelationID: event.CorrelationID,
			ObservedAt:    event.ObservedAt.UTC(),
			EventKind:     event.EventKind,
			Conclusion:    firstNonEmpty(payload.CheckSuite.Conclusion, payload.CheckRun.Conclusion, payload.State),
		})
	case "pull_request_review", "pull_request_review_thread":
		model.Review = append(model.Review, ReviewObservation{
			Authority:     ObservationAuthorityAuthoritative,
			Source:        event.EventSource,
			CorrelationID: event.CorrelationID,
			ObservedAt:    event.ObservedAt.UTC(),
			EventKind:     event.EventKind,
			Status:        strings.TrimSpace(payload.Review.State),
			Action:        strings.TrimSpace(observation.Action),
		})
	case "pull_request":
		model.PR = append(model.PR, PRObservation{
			Authority:     ObservationAuthorityAuthoritative,
			Source:        event.EventSource,
			CorrelationID: event.CorrelationID,
			ObservedAt:    event.ObservedAt.UTC(),
			EventKind:     event.EventKind,
			Action:        strings.TrimSpace(observation.Action),
			Adapter:       strings.TrimSpace(observation.Adapter),
			WorkflowState: strings.TrimSpace(observation.WorkflowState),
			PRNumber:      observation.PRNumber,
			Draft:         payload.PullRequest.Draft,
		})
	}
}

func consumePollObservationEvent(model *ObservationModel, event store.ExternalEvent) {
	var observation pollObservation
	if err := json.Unmarshal([]byte(event.Payload), &observation); err != nil {
		return
	}
	if !isObservationPRRelevant(model.Checkpoint.PRNumber, observation.PRNumber) {
		return
	}
	if branchObservation, ok := pollBranchObservation(event, observation); ok {
		model.Branch = append(model.Branch, branchObservation)
	}

	switch observation.WakeKind {
	case "poll_ci":
		model.CI = append(model.CI, CIObservation{
			Authority:     ObservationAuthorityAuthoritative,
			Source:        event.EventSource,
			CorrelationID: event.CorrelationID,
			ObservedAt:    event.ObservedAt.UTC(),
			EventKind:     event.EventKind,
			Verdict:       observation.DecisionVerdict,
			TotalChecks:   observation.CI.TotalChecks,
			GreenChecks:   observation.CI.GreenChecks,
			PendingChecks: append([]string(nil), observation.CI.PendingChecks...),
			FailedChecks:  append([]string(nil), observation.CI.FailedChecks...),
		})
	case "poll_review":
		model.Review = append(model.Review, ReviewObservation{
			Authority:     ObservationAuthorityAuthoritative,
			Source:        event.EventSource,
			CorrelationID: event.CorrelationID,
			ObservedAt:    event.ObservedAt.UTC(),
			EventKind:     event.EventKind,
			Status:        observation.ReviewStatus,
			Verdict:       observation.DecisionVerdict,
		})
	case "poll_pr_ready":
		model.PR = append(model.PR, PRObservation{
			Authority:     ObservationAuthorityAuthoritative,
			Source:        event.EventSource,
			CorrelationID: event.CorrelationID,
			ObservedAt:    event.ObservedAt.UTC(),
			EventKind:     event.EventKind,
			PRNumber:      observation.PRNumber,
			Draft:         observation.Draft,
			WorkflowState: observation.NewState,
			Verdict:       observation.DecisionVerdict,
		})
	}
}

func consumeOperatorObservationEvent(model *ObservationModel, event store.ExternalEvent) {
	var detail manualOverrideDetail
	if err := json.Unmarshal([]byte(event.Payload), &detail); err != nil {
		return
	}
	model.Operator = append(model.Operator, OperatorObservation{
		Authority:     ObservationAuthorityAuthoritative,
		Source:        event.EventSource,
		CorrelationID: event.CorrelationID,
		ObservedAt:    event.ObservedAt.UTC(),
		Action:        detail.Action,
		RequestedBy:   detail.RequestedBy,
		Reason:        detail.Reason,
		PreviousState: detail.PreviousState,
		NewState:      detail.NewState,
		ResumeState:   detail.ResumeState,
	})
}

func consumeObservationSummary(model *ObservationModel, decision store.PolicyDecision) {
	switch decision.DecisionKind {
	case pollDecisionKind, string(PolicyDecisionCIReadiness), string(PolicyDecisionReviewReadiness):
		observation, ok := decodePollObservation(decision.Detail)
		if !ok || !isObservationPRRelevant(model.Checkpoint.PRNumber, observation.PRNumber) {
			return
		}
		if modelSummary := pollSummaryFromObservation(decision, observation); modelSummary != nil {
			switch summary := modelSummary.(type) {
			case *CISummary:
				if model.Summaries.CI == nil {
					model.Summaries.CI = summary
				}
			case *ReviewSummary:
				if model.Summaries.Review == nil {
					model.Summaries.Review = summary
				}
			case *PRSummary:
				if model.Summaries.PR == nil {
					model.Summaries.PR = summary
				}
			}
		}
	case manualOverrideDecisionKind:
		if model.Summaries.Operator != nil {
			return
		}
		var detail manualOverrideDetail
		if err := json.Unmarshal([]byte(decision.Detail), &detail); err != nil {
			return
		}
		model.Summaries.Operator = &OperatorSummary{
			Authority:     ObservationAuthorityDerived,
			CorrelationID: decision.CorrelationID,
			RecordedAt:    decision.CreatedAt.UTC(),
			Action:        detail.Action,
			RequestedBy:   detail.RequestedBy,
			Reason:        detail.Reason,
		}
	}
}

func decodePollObservation(detail string) (pollObservation, bool) {
	var observation pollObservation
	if err := json.Unmarshal([]byte(detail), &observation); err != nil {
		return pollObservation{}, false
	}
	return observation, true
}

func pollSummaryFromObservation(decision store.PolicyDecision, observation pollObservation) any {

	switch observation.WakeKind {
	case "poll_ci":
		conclusion := ""
		if observation.CI.TotalChecks > 0 && observation.CI.GreenChecks == observation.CI.TotalChecks {
			conclusion = "success"
		} else if len(observation.CI.FailedChecks) > 0 {
			conclusion = "failure"
		} else if len(observation.CI.PendingChecks) > 0 {
			conclusion = "pending"
		}
		return &CISummary{
			Authority:     ObservationAuthorityDerived,
			CorrelationID: decision.CorrelationID,
			RecordedAt:    decision.CreatedAt.UTC(),
			Conclusion:    conclusion,
			Verdict:       decision.Verdict,
			TotalChecks:   observation.CI.TotalChecks,
			GreenChecks:   observation.CI.GreenChecks,
		}
	case "poll_review":
		return &ReviewSummary{
			Authority:     ObservationAuthorityDerived,
			CorrelationID: decision.CorrelationID,
			RecordedAt:    decision.CreatedAt.UTC(),
			Status:        observation.ReviewStatus,
			Verdict:       decision.Verdict,
		}
	case "poll_pr_ready":
		return &PRSummary{
			Authority:     ObservationAuthorityDerived,
			CorrelationID: decision.CorrelationID,
			RecordedAt:    decision.CreatedAt.UTC(),
			PRNumber:      observation.PRNumber,
			Draft:         observation.Draft,
			WorkflowState: observation.NewState,
			Verdict:       decision.Verdict,
		}
	default:
		return nil
	}
}

func isRelevantGitHubObservation(model *ObservationModel, observation githubEventObservation) bool {
	if sessionID := strings.TrimSpace(observation.SessionID); sessionID != "" && sessionID != model.SessionID {
		return false
	}
	if !isSupportedGitHubObservationType(observation.EventType) {
		return false
	}
	return isObservationPRRelevant(model.Checkpoint.PRNumber, observation.PRNumber)
}

func isSupportedGitHubObservationType(eventType string) bool {
	switch strings.ToLower(strings.TrimSpace(eventType)) {
	case "check_run", "check_suite", "status", "pull_request_review", "pull_request_review_thread", "pull_request":
		return true
	default:
		return false
	}
}

func isObservationPRRelevant(activePR, observedPR int) bool {
	if activePR > 0 {
		return observedPR == activePR
	}
	return observedPR <= 0
}

func pollBranchObservation(event store.ExternalEvent, observation pollObservation) (BranchObservation, bool) {
	ref := firstNonEmpty(observation.Branch.HeadRef, observation.Branch.BaseRef)
	if ref == "" && observation.Branch.BaseRef == "" && observation.Branch.HeadSHA == "" {
		return BranchObservation{}, false
	}
	return BranchObservation{
		Authority:     ObservationAuthorityAuthoritative,
		Source:        event.EventSource,
		CorrelationID: event.CorrelationID,
		ObservedAt:    event.ObservedAt.UTC(),
		Ref:           ref,
		BaseRef:       strings.TrimSpace(observation.Branch.BaseRef),
		HeadRef:       strings.TrimSpace(observation.Branch.HeadRef),
		HeadSHA:       strings.TrimSpace(observation.Branch.HeadSHA),
	}, true
}

func githubBranchObservation(event store.ExternalEvent, observation githubEventObservation, payload githubPayloadSnapshot, repository string) (BranchObservation, bool) {
	headRef := firstNonEmpty(payload.PullRequest.Head.Ref, payload.CheckSuite.HeadBranch, payload.CheckRun.CheckSuite.HeadBranch)
	headSHA := firstNonEmpty(payload.PullRequest.Head.SHA, payload.CheckSuite.HeadSHA, payload.CheckRun.CheckSuite.HeadSHA)
	baseRef := strings.TrimSpace(payload.PullRequest.Base.Ref)
	ref := firstNonEmpty(payload.Ref, headRef)
	if ref == "" && baseRef == "" && headSHA == "" {
		return BranchObservation{}, false
	}
	return BranchObservation{
		Authority:     ObservationAuthorityAuthoritative,
		Source:        event.EventSource,
		CorrelationID: event.CorrelationID,
		ObservedAt:    event.ObservedAt.UTC(),
		Repository:    repository,
		Ref:           ref,
		BaseRef:       baseRef,
		HeadRef:       headRef,
		HeadSHA:       headSHA,
	}, true
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}
