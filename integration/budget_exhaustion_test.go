package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/guillaume7/loom/internal/fsm"
	loomgh "github.com/guillaume7/loom/internal/github"
	"github.com/guillaume7/loom/internal/mcp"
	"github.com/guillaume7/loom/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRetryBudgetExhaustion verifies that AWAITING_PR budget exhaustion no
// longer transitions immediately to PAUSED, and that a resumed FSM can still
// advance past AWAITING_PR when a PR becomes available (US-8.3).
func TestRetryBudgetExhaustion(t *testing.T) {
	// returnPR controls whether the httptest server returns a PR.
	var returnPR atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/owner/repo":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"full_name": "owner/repo"})

		case r.Method == http.MethodPost && r.URL.Path == "/repos/owner/repo/issues":
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(loomgh.Issue{Number: 1, Title: "Budget test", State: "open"})

		case r.Method == http.MethodGet && r.URL.Path == "/repos/owner/repo/pulls":
			w.WriteHeader(http.StatusOK)
			if returnPR.Load() {
				_ = json.NewEncoder(w).Encode([]map[string]interface{}{
					{"number": 1, "title": "PR 1", "draft": false, "state": "open",
						"head": map[string]interface{}{"sha": "abc"}},
				})
			} else {
				_ = json.NewEncoder(w).Encode([]map[string]interface{}{}) // no PR yet
			}

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	// Budget of 2: after 3 timeout events the machine emits elicitation and
	// remains in AWAITING_PR until operator response.
	cfg := fsm.Config{
		MaxRetriesAwaitingPR:    2,
		MaxRetriesAwaitingReady: 60,
		MaxRetriesAwaitingCI:    20,
		MaxDebugCycles:          3,
		MaxFeedbackCycles:       5,
	}
	machine := fsm.NewMachine(cfg)
	st, err := store.New(":memory:")
	require.NoError(t, err)
	defer st.Close()

	ghClient := newGHClient(srv.URL)
	server := mcp.NewServer(machine, st, ghClient)
	mcpSvr := server.MCPServer()
	ctx := context.Background()

	// ── Phase 1: drive to AWAITING_PR and exhaust the budget ──────────────

	// IDLE → SCANNING
	checkpoint(t, mcpSvr, "start")
	// SCANNING → ISSUE_CREATED
	_, err = ghClient.CreateIssue(ctx, "Budget test", "", nil)
	require.NoError(t, err)
	checkpoint(t, mcpSvr, "phase_identified")
	// ISSUE_CREATED → AWAITING_PR
	checkpoint(t, mcpSvr, "copilot_assigned")

	// Fire timeout events until budget is exhausted (budget=2, need 3 timeouts).
	timeoutSession := newRegisteredSession(t, mcpSvr)
	initializeSessionWithCapabilities(t, mcpSvr, timeoutSession, map[string]interface{}{
		"experimental": map[string]interface{}{
			"elicitation": true,
		},
	})
	for i := 0; i <= cfg.MaxRetriesAwaitingPR; i++ {
		step := nextStep(t, mcpSvr)
		require.Equal(t, "AWAITING_PR", step.State,
			"expected AWAITING_PR before budget exhausted at iteration %d", i)
		// List PRs: server returns empty array.
		prs, lErr := ghClient.ListPRs(ctx, "")
		require.NoError(t, lErr)
		require.Empty(t, prs, "expected no PR from server")

		result := callToolOnSession(t, mcpSvr, timeoutSession, "loom_checkpoint", map[string]interface{}{"action": "timeout"})
		require.NotNil(t, result)
	}

	notes := drainNotifications(timeoutSession)
	require.Len(t, notes, 1, "expected one elicitation notification on AWAITING_PR budget exhaustion")
	assert.Equal(t, "loom/elicitation", notes[0].Method)
	assert.Equal(t, "elicitation", notes[0].Params.AdditionalFields["type"])

	// Assert state remains AWAITING_PR while awaiting elicitation response.
	step := nextStep(t, mcpSvr)
	assert.Equal(t, "AWAITING_PR", step.State, "expected AWAITING_PR after budget exhaustion elicitation")

	cp, err := st.ReadCheckpoint(ctx)
	require.NoError(t, err)
	assert.Equal(t, "AWAITING_PR", cp.State, "store checkpoint must remain at AWAITING_PR")

	// ── Phase 2: resume — new FSM from AWAITING_PR; PR now available ──────

	// Simulate loom resume: create a new FSM, replay transitions to AWAITING_PR.
	returnPR.Store(true)

	machine2 := fsm.NewMachine(fsm.DefaultConfig())
	st2, err := store.New(":memory:")
	require.NoError(t, err)
	defer st2.Close()

	server2 := mcp.NewServer(machine2, st2, ghClient)
	mcpSvr2 := server2.MCPServer()

	// Replay: IDLE → SCANNING → ISSUE_CREATED → AWAITING_PR
	checkpoint(t, mcpSvr2, "start")
	checkpoint(t, mcpSvr2, "phase_identified")
	checkpoint(t, mcpSvr2, "copilot_assigned")

	step2 := nextStep(t, mcpSvr2)
	require.Equal(t, "AWAITING_PR", step2.State, "resumed FSM must be in AWAITING_PR")

	// PR is now available; advance past AWAITING_PR.
	prs, err := ghClient.ListPRs(ctx, "")
	require.NoError(t, err)
	require.NotEmpty(t, prs, "expected PR to be available after resume")

	r := checkpoint(t, mcpSvr2, "pr_opened")
	assert.Equal(t, "AWAITING_PR", r.PreviousState)
	assert.Equal(t, "AWAITING_READY", r.NewState, "FSM must advance to AWAITING_READY after resume")

	cp2, err := st2.ReadCheckpoint(ctx)
	require.NoError(t, err)
	assert.Equal(t, "AWAITING_READY", cp2.State, "store must reflect AWAITING_READY after resume")
}
