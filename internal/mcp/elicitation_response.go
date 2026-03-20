package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/guillaume7/loom/internal/fsm"
	loomruntime "github.com/guillaume7/loom/internal/runtime"
	"github.com/guillaume7/loom/internal/store"
	mcplib "github.com/mark3labs/mcp-go/mcp"
)

type prClosingClient interface {
	CloseIssue(ctx context.Context, issueNumber int) error
}

type ElicitationResponseResult struct {
	Action        string `json:"action"`
	PreviousState string `json:"previous_state"`
	NewState      string `json:"new_state"`
	Phase         int    `json:"phase"`
	Detail        string `json:"detail,omitempty"`
}

func isValidElicitationAction(action string) bool {
	for _, candidate := range elicitationActions {
		if action == candidate {
			return true
		}
	}
	return false
}

func validElicitationActionsText() string {
	return strings.Join(elicitationActions, ", ")
}

func (s *Server) handleElicitationResponse(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	const toolName = "loom_elicitation_response"
	start := time.Now()

	if res, cancelled := checkCtx(ctx, toolName); cancelled {
		return res, nil
	}

	action, ok := req.GetArguments()["action"].(string)
	if !ok || strings.TrimSpace(action) == "" {
		return mcplib.NewToolResultError("missing or invalid 'action' argument: must be a non-empty string"), nil
	}
	action = strings.TrimSpace(action)
	if !isValidElicitationAction(action) {
		return mcplib.NewToolResultError(fmt.Sprintf("invalid action %q: valid choices are %s", action, validElicitationActionsText())), nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	active := s.activeElicitation
	if !active.Active {
		return mcplib.NewToolResultError("no active elicitation to respond to"), nil
	}

	cp, readErr := s.readCheckpointWithErr(ctx)
	if readErr != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("failed to read checkpoint: %v", readErr)), nil
	}
	previousState := s.machine.State()
	nextPhase := cp.Phase
	nextRetryCount := cp.RetryCount
	nextPRNumber := cp.PRNumber
	detail := ""
	originalCheckpoint := cp

	snap := s.machine.TakeSnapshot()
	var closePRNumber int

	var transitionEvent fsm.Event
	switch action {
	case "skip":
		transitionEvent = fsm.EventSkipStory
		nextPhase = cp.Phase + 1
		nextRetryCount = 0
		detail = fmt.Sprintf("skipped story at phase %d", cp.Phase)

	case "reassign":
		prNumber := active.PRNumber
		if prNumber <= 0 {
			prNumber = cp.PRNumber
		}
		if prNumber <= 0 {
			return mcplib.NewToolResultError("reassign requires a positive PR number in the active elicitation context"), nil
		}

		transitionEvent = fsm.EventReassign
		nextRetryCount = 0
		nextPRNumber = 0
		closePRNumber = prNumber
		detail = fmt.Sprintf("reassigned story from PR #%d", prNumber)

	case "pause_epic":
		transitionEvent = fsm.EventAbort
		detail = "paused epic via elicitation response"
	}

	newState, transErr := s.machine.Transition(transitionEvent)
	if transErr != nil {
		return mcplib.NewToolResultError(transErr.Error()), nil
	}

	if action == "pause_epic" {
		controller := loomruntime.NewController(s.runtimeStore(), loomruntime.DefaultConfig())
		if _, writeErr := controller.ApplyManualOverride(ctx, loomruntime.ManualOverrideRequest{
			Action:      loomruntime.ManualOverridePause,
			Source:      "mcp",
			RequestedBy: sessionIDFromContext(ctx),
			Reason:      detail,
		}); writeErr != nil {
			s.machine.Rollback(snap)
			return mcplib.NewToolResultError(fmt.Sprintf("failed to persist checkpoint: %v", writeErr)), nil
		}
	} else if writeErr := s.writeCheckpoint(ctx, store.Checkpoint{
		State:       string(newState),
		ResumeState: string(previousStateIfPaused(newState, previousState)),
		Phase:       nextPhase,
		PRNumber:    nextPRNumber,
		IssueNumber: cp.IssueNumber,
		RetryCount:  nextRetryCount,
	}); writeErr != nil {
		s.machine.Rollback(snap)
		return mcplib.NewToolResultError(fmt.Sprintf("failed to persist checkpoint: %v", writeErr)), nil
	}

	if closePRNumber > 0 {
		closingClient, hasClosingClient := s.gh.(prClosingClient)
		if !hasClosingClient {
			s.machine.Rollback(snap)
			if restoreErr := s.writeCheckpoint(ctx, originalCheckpoint); restoreErr != nil {
				return mcplib.NewToolResultError(fmt.Sprintf("reassign requested but GitHub client does not support PR closing; additionally failed to restore checkpoint: %v", restoreErr)), nil
			}
			return mcplib.NewToolResultError("reassign requested but GitHub client does not support PR closing"), nil
		}
		if err := closingClient.CloseIssue(ctx, closePRNumber); err != nil {
			s.machine.Rollback(snap)
			if restoreErr := s.writeCheckpoint(ctx, originalCheckpoint); restoreErr != nil {
				return mcplib.NewToolResultError(fmt.Sprintf("failed to close PR #%d: %v; additionally failed to restore checkpoint: %v", closePRNumber, err, restoreErr)), nil
			}
			return mcplib.NewToolResultError(fmt.Sprintf("failed to close PR #%d: %v", closePRNumber, err)), nil
		}
	}

	s.activeElicitation = elicitationContext{}

	result := ElicitationResponseResult{
		Action:        action,
		PreviousState: string(previousState),
		NewState:      string(newState),
		Phase:         nextPhase,
		Detail:        detail,
	}

	slog.InfoContext(ctx, "tool completed", "tool", toolName, "action", action, "state", string(newState), "duration_ms", time.Since(start).Milliseconds())
	return toolResultJSON(result), nil
}

func previousStateIfPaused(newState fsm.State, previousState fsm.State) fsm.State {
	if newState == fsm.StatePaused {
		return previousState
	}
	return ""
}
