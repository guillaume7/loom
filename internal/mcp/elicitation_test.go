package mcp_test

import (
	"context"
	"testing"

	"github.com/guillaume7/loom/internal/fsm"
	"github.com/guillaume7/loom/internal/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestElicitationEmitter_BudgetExhaustion_SendsStructuredSchema(t *testing.T) {
	notifier := &recordingNotifier{}
	emitter := mcp.NewElicitationEmitter(notifier)

	err := emitter.BudgetExhaustion(context.Background(), 42, fsm.StateAwaitingCI, fsm.EventTimeout)
	require.NoError(t, err)
	require.Len(t, notifier.notifications, 1)

	note := notifier.notifications[0]
	require.Equal(t, "loom/elicitation", note.method)

	assert.Equal(t, "elicitation", note.params["type"])
	assert.Equal(t, "PR #42 — CI budget exhausted", note.params["title"])
	assert.Contains(t, note.params["description"], "AWAITING_CI")

	schema, ok := note.params["schema"].(map[string]any)
	require.True(t, ok)

	action, ok := schema["action"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "string", action["type"])
	assert.Equal(t, []string{"skip", "reassign", "pause_epic"}, action["enum"])
	assert.Equal(t, []string{
		"Skip this user story and advance to the next",
		"Re-assign the PR to a fresh @copilot session",
		"Pause the epic and require human intervention",
	}, action["enumDescriptions"])
}
