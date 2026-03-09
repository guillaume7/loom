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

// TestDebugLoop exercises the CI-failure → DEBUGGING → fix → REVIEWING path
// (US-8.2). The httptest server returns a CI failure on the first check-runs
// call and success on subsequent calls.
func TestDebugLoop(t *testing.T) {
	var checkRunCalls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/owner/repo":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"full_name": "owner/repo"})

		case r.Method == http.MethodPost && r.URL.Path == "/repos/owner/repo/issues":
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(loomgh.Issue{Number: 1, Title: "Debug issue", State: "open"})

		case r.Method == http.MethodGet && r.URL.Path == "/repos/owner/repo/pulls":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{"number": 1, "title": "PR 1", "draft": false, "state": "open",
					"head": map[string]interface{}{"sha": "deadbeef"}},
			})

		case r.Method == http.MethodGet && r.URL.Path == "/repos/owner/repo/pulls/1":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"number": 1, "draft": false, "state": "open",
				"head": map[string]interface{}{"sha": "deadbeef"},
			})

		case r.Method == http.MethodGet && r.URL.Path == "/repos/owner/repo/commits/deadbeef/check-runs":
			n := checkRunCalls.Add(1)
			w.WriteHeader(http.StatusOK)
			if n == 1 {
				// First call: CI failure
				json.NewEncoder(w).Encode(map[string]interface{}{
					"check_runs": []map[string]interface{}{
						{"name": "CI", "status": "completed", "conclusion": "failure"},
					},
				})
			} else {
				// Subsequent calls: CI success
				json.NewEncoder(w).Encode(map[string]interface{}{
					"check_runs": []map[string]interface{}{
						{"name": "CI", "status": "completed", "conclusion": "success"},
					},
				})
			}

		case r.Method == http.MethodPost && r.URL.Path == "/repos/owner/repo/pulls/1/requested_reviewers":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{})

		case r.Method == http.MethodGet && r.URL.Path == "/repos/owner/repo/pulls/1/reviews":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{"state": "APPROVED", "body": "LGTM"},
			})

		case r.Method == http.MethodPut && r.URL.Path == "/repos/owner/repo/pulls/1/merge":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"merged": true})

		default:
			t.Logf("unhandled request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	machine := fsm.NewMachine(fsm.DefaultConfig())
	st, err := store.New(":memory:")
	require.NoError(t, err)
	defer st.Close()

	ghClient := newGHClient(srv.URL)
	server := mcp.NewServer(machine, st, ghClient)
	mcpSvr := server.MCPServer()
	ctx := context.Background()

	debugged := false
	reached := false

	const maxIter = 100
	for i := 0; i < maxIter; i++ {
		step := nextStep(t, mcpSvr)
		state := step.State

		switch state {
		case "IDLE":
			checkpoint(t, mcpSvr, "start")

		case "SCANNING":
			_, err := ghClient.CreateIssue(ctx, "Debug issue", "", nil)
			require.NoError(t, err)
			checkpoint(t, mcpSvr, "phase_identified")

		case "ISSUE_CREATED":
			checkpoint(t, mcpSvr, "copilot_assigned")

		case "AWAITING_PR":
			prs, err := ghClient.ListPRs(ctx, "")
			require.NoError(t, err)
			if len(prs) > 0 {
				checkpoint(t, mcpSvr, "pr_opened")
			} else {
				checkpoint(t, mcpSvr, "timeout")
			}

		case "AWAITING_READY":
			checkpoint(t, mcpSvr, "pr_ready")

		case "AWAITING_CI":
			runs, err := ghClient.GetCheckRuns(ctx, "deadbeef")
			require.NoError(t, err)
			if len(runs) > 0 && runs[0].Conclusion == "success" {
				checkpoint(t, mcpSvr, "ci_green")
			} else {
				checkpoint(t, mcpSvr, "ci_red")
			}

		case "DEBUGGING":
			debugged = true
			// Simulate Copilot pushing a fix.
			checkpoint(t, mcpSvr, "fix_pushed")

		case "REVIEWING":
			status, err := ghClient.GetReviewStatus(ctx, 1)
			require.NoError(t, err)
			if status == "APPROVED" {
				checkpoint(t, mcpSvr, "review_approved")
			} else {
				checkpoint(t, mcpSvr, "review_changes_requested")
			}

		case "MERGING":
			require.NoError(t, ghClient.MergePR(ctx, 1, "merge"))
			checkpoint(t, mcpSvr, "merged")
			reached = true
			goto done

		case "PAUSED":
			t.Fatalf("unexpected PAUSED state at iteration %d", i)
		}
	}
	t.Fatalf("FSM did not reach MERGING within %d iterations", maxIter)

done:
	assert.True(t, debugged, "expected FSM to pass through DEBUGGING state")
	assert.True(t, reached, "expected FSM to reach MERGING")
	assert.Equal(t, int32(2), checkRunCalls.Load(),
		"expected exactly 2 check-run calls: one failure, one success")

	// Verify store reflects the post-merge SCANNING state.
	cp, err := st.ReadCheckpoint(ctx)
	require.NoError(t, err)
	assert.Equal(t, "SCANNING", cp.State)
}
