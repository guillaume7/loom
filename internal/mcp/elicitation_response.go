package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/guillaume7/loom/internal/fsm"
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

	cp := s.readCheckpoint(ctx, toolName)
	previousState := s.machine.State()
	nextPhase := cp.Phase
	nextRetryCount := cp.RetryCount
	nextPRNumber := cp.PRNumber
	detail := ""

	snap := s.machine.TakeSnapshot()

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

		closingClient, hasClosingClient := s.gh.(prClosingClient)
		if !hasClosingClient {
			return mcplib.NewToolResultError("reassign requested but GitHub client does not support PR closing"), nil
		}
		if err := closingClient.CloseIssue(ctx, prNumber); err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("failed to close PR #%d: %v", prNumber, err)), nil
		}

		transitionEvent = fsm.EventReassign
		nextRetryCount = 0
		nextPRNumber = 0
		detail = fmt.Sprintf("reassigned story from PR #%d", prNumber)

	case "pause_epic":
		transitionEvent = fsm.EventAbort
		detail = "paused epic via elicitation response"
	}

	newState, transErr := s.machine.Transition(transitionEvent)
	if transErr != nil {
		return mcplib.NewToolResultError(transErr.Error()), nil
	}

	if writeErr := s.st.WriteCheckpoint(ctx, store.Checkpoint{
		State:       string(newState),
		Phase:       nextPhase,
		PRNumber:    nextPRNumber,
		IssueNumber: cp.IssueNumber,
		RetryCount:  nextRetryCount,
	}); writeErr != nil {
		s.machine.Rollback(snap)
		return mcplib.NewToolResultError(fmt.Sprintf("failed to persist checkpoint: %v", writeErr)), nil
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
