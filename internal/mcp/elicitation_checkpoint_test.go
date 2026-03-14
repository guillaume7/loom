package mcp_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/guillaume7/loom/internal/fsm"
	"github.com/guillaume7/loom/internal/mcp"
	"github.com/guillaume7/loom/internal/store"
	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoomCheckpoint_BudgetExhaustion_NonIdempotent_DoesNotDeadlock(t *testing.T) {
	s, mcpSvr := newTestServer(t)
	st := s.Store().(*memStore)

	for _, action := range []string{"start", "phase_identified", "copilot_assigned"} {
		result := callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": action})
		require.False(t, result.IsError, "failed to advance with action %q: %s", action, toolText(t, result))
	}

	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{
		State:    string(fsm.StateAwaitingPR),
		Phase:    2,
		PRNumber: 42,
	}))
	sess := newTestSession(nextSessionID())
	require.NoError(t, mcpSvr.RegisterSession(context.Background(), sess))
	initializeSessionWithCapabilities(t, mcpSvr, sess, map[string]interface{}{
		"experimental": map[string]interface{}{
			"elicitation": true,
		},
	})

	done := make(chan *mcplib.CallToolResult, 1)
	go func() {
		done <- callToolOnSession(t, mcpSvr, sess, "loom_checkpoint", map[string]interface{}{"action": "timeout"})
	}()

	select {
	case result := <-done:
		require.False(t, result.IsError)
		var got mcp.CheckpointResult
		require.NoError(t, json.Unmarshal([]byte(toolText(t, result)), &got))
		assert.Equal(t, "AWAITING_PR", got.NewState)
	case <-time.After(2 * time.Second):
		t.Fatal("loom_checkpoint timed out in budget-exhaustion elicitation path")
	}
}
