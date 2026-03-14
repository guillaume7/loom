package mcp

import (
	"context"
	"fmt"

	"github.com/guillaume7/loom/internal/fsm"
)

const elicitationNotificationMethod = "loom/elicitation"

var elicitationActions = []string{"skip", "reassign", "pause_epic"}

var elicitationActionDescriptions = []string{
	"Skip this user story and advance to the next",
	"Re-assign the PR to a fresh @copilot session",
	"Pause the epic and require human intervention",
}

// ElicitationEmitter emits MCP elicitations as JSON-RPC notifications.
type ElicitationEmitter struct {
	notifier TaskNotifier
}

// NewElicitationEmitter creates an ElicitationEmitter wrapping the given notifier.
func NewElicitationEmitter(notifier TaskNotifier) *ElicitationEmitter {
	return &ElicitationEmitter{notifier: notifier}
}

// BudgetExhaustion emits a structured elicitation when retry budget is exhausted.
func (e *ElicitationEmitter) BudgetExhaustion(ctx context.Context, prNumber int, state fsm.State, event fsm.Event) error {
	return e.notifier.SendNotificationToClient(ctx, elicitationNotificationMethod, budgetExhaustionElicitation(prNumber, state, event))
}

func budgetExhaustionElicitation(prNumber int, state fsm.State, event fsm.Event) map[string]any {
	title := "CI budget exhausted"
	if prNumber > 0 {
		title = fmt.Sprintf("PR #%d — CI budget exhausted", prNumber)
	}

	description := fmt.Sprintf("Retry budget exhausted in %s on action %s. Choose an action.", state, event)

	return map[string]any{
		"type":        "elicitation",
		"title":       title,
		"description": description,
		"schema": map[string]any{
			"action": map[string]any{
				"type":             "string",
				"enum":             elicitationActions,
				"enumDescriptions": elicitationActionDescriptions,
			},
		},
	}
}

// ElicitationEmitter returns the elicitation emitter bound to the latest MCP server instance.
// Call MCPServer() first to ensure it is initialized.
func (s *Server) ElicitationEmitter() *ElicitationEmitter {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.elicitationEmitter
}
