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

// TestFullLifecycle_ThreePhases drives the FSM from IDLE to COMPLETE through
// three simulated phases using an httptest server, real SQLite (in-memory),
// and the full MCP server stack (US-8.1).
func TestFullLifecycle_ThreePhases(t *testing.T) {
	var issueCount atomic.Int32
	var prListCount atomic.Int32
	var checkRunCount atomic.Int32
	var reviewRequestCount atomic.Int32
	var reviewCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		// Repository ping
		case r.Method == http.MethodGet && r.URL.Path == "/repos/owner/repo":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"full_name": "owner/repo"})

		// Create issue
		case r.Method == http.MethodPost && r.URL.Path == "/repos/owner/repo/issues":
			n := issueCount.Add(1)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(loomgh.Issue{Number: int(n), Title: "Phase issue", State: "open"})

		// List PRs
		case r.Method == http.MethodGet && r.URL.Path == "/repos/owner/repo/pulls":
			prListCount.Add(1)
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{"number": 1, "title": "PR 1", "draft": false, "state": "open", "head": map[string]interface{}{"sha": "abc123"}},
			})

		// Get single PR
		case r.Method == http.MethodGet && r.URL.Path == "/repos/owner/repo/pulls/1":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"number": 1, "title": "PR 1", "draft": false, "state": "open",
				"head": map[string]interface{}{"sha": "abc123"},
			})

		// Get check runs
		case r.Method == http.MethodGet && r.URL.Path == "/repos/owner/repo/commits/abc123/check-runs":
			checkRunCount.Add(1)
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"check_runs": []map[string]interface{}{
					{"name": "CI", "status": "completed", "conclusion": "success"},
				},
			})

		// Request review
		case r.Method == http.MethodPost && r.URL.Path == "/repos/owner/repo/pulls/1/requested_reviewers":
			reviewRequestCount.Add(1)
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{})

		// Get reviews
		case r.Method == http.MethodGet && r.URL.Path == "/repos/owner/repo/pulls/1/reviews":
			reviewCount.Add(1)
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{"state": "APPROVED", "body": "LGTM"},
			})

		// Merge PR
		case r.Method == http.MethodPut && r.URL.Path == "/repos/owner/repo/pulls/1/merge":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"merged": true})

		// Create tag
		case r.Method == http.MethodPost && r.URL.Path == "/repos/owner/repo/git/refs":
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"ref":    "refs/tags/v1.0.0",
				"object": map[string]interface{}{"sha": "abc123"},
			})

		// Create release
		case r.Method == http.MethodPost && r.URL.Path == "/repos/owner/repo/releases":
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(loomgh.Release{ID: 1, TagName: "v1.0.0", Name: "Release v1.0.0"})

		default:
			t.Logf("unhandled request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
		}
	}))
	defer srv.Close()

	// Wire up real components.
	machine := fsm.NewMachine(fsm.DefaultConfig())
	st, err := store.New(":memory:")
	require.NoError(t, err)
	defer st.Close()

	ghClient := newGHClient(srv.URL)
	server := mcp.NewServer(machine, st, ghClient)
	mcpSvr := server.MCPServer()

	ctx := context.Background()

	// Phase tracking: scanCount tracks which scan we're on (0-indexed).
	// mergeCount tracks how many merges have occurred.
	scanCount := 0
	mergeCount := 0

	const maxIter = 200
	for i := 0; i < maxIter; i++ {
		step := nextStep(t, mcpSvr)
		state := step.State

		switch state {
		case "IDLE":
			checkpoint(t, mcpSvr, "start")

		case "SCANNING":
			// Phase 1 and 2: create issue and advance; phase 3: all done.
			if scanCount < 2 {
				issue, err := ghClient.CreateIssue(ctx, "Phase issue", "", nil)
				require.NoError(t, err)
				r := callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": "phase_identified", "issue_number": issue.Number})
				require.False(t, r.IsError, "phase_identified failed: %s", toolText(t, r))
				scanCount++
			} else {
				checkpoint(t, mcpSvr, "all_phases_done")
			}

		case "ISSUE_CREATED":
			checkpoint(t, mcpSvr, "copilot_assigned")

		case "AWAITING_PR":
			prs, err := ghClient.ListPRs(ctx, "")
			require.NoError(t, err)
			if len(prs) > 0 {
				r := callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": "pr_opened", "pr_number": prs[0].Number})
				require.False(t, r.IsError, "pr_opened failed: %s", toolText(t, r))
			} else {
				checkpoint(t, mcpSvr, "timeout")
			}

		case "AWAITING_READY":
			pr, err := ghClient.GetPR(ctx, 1)
			require.NoError(t, err)
			if !pr.Draft {
				checkpoint(t, mcpSvr, "pr_ready")
			} else {
				checkpoint(t, mcpSvr, "timeout")
			}

		case "AWAITING_CI":
			runs, err := ghClient.GetCheckRuns(ctx, "abc123")
			require.NoError(t, err)
			if len(runs) > 0 && runs[0].Conclusion == "success" {
				checkpoint(t, mcpSvr, "ci_green")
			} else {
				checkpoint(t, mcpSvr, "ci_red")
			}

		case "REVIEWING":
			status, err := ghClient.GetReviewStatus(ctx, 1)
			require.NoError(t, err)
			switch status {
			case "APPROVED":
				checkpoint(t, mcpSvr, "review_approved")
			case "CHANGES_REQUESTED":
				checkpoint(t, mcpSvr, "review_changes_requested")
			default:
				checkpoint(t, mcpSvr, "review_approved") // default to approved in simulation
			}

		case "MERGING":
			require.NoError(t, ghClient.MergePR(ctx, 1, "merge"))
			// Phase 1 (mergeCount==0): simple merge back to SCANNING.
			// Phase 2 (mergeCount==1): epic-boundary merge → REFACTORING.
			if mergeCount == 0 {
				checkpoint(t, mcpSvr, "merged")
			} else {
				checkpoint(t, mcpSvr, "merged_epic_boundary")
			}
			mergeCount++

		case "REFACTORING":
			checkpoint(t, mcpSvr, "refactor_merged")

		case "DEBUGGING":
			checkpoint(t, mcpSvr, "fix_pushed")

		case "ADDRESSING_FEEDBACK":
			checkpoint(t, mcpSvr, "feedback_addressed")

		case "COMPLETE":
			// Done!
			goto done

		case "PAUSED":
			t.Fatalf("unexpected PAUSED state at iteration %d", i)
		}
	}
	t.Fatalf("FSM did not reach COMPLETE within %d iterations", maxIter)

done:
	// Assert final store state.
	cp, err := st.ReadCheckpoint(ctx)
	require.NoError(t, err)
	assert.Equal(t, "COMPLETE", cp.State, "store checkpoint must reflect COMPLETE state")

	// Assert that GitHub API was exercised.
	assert.Positive(t, issueCount.Load(), "expected at least one issue to be created")
	assert.Positive(t, prListCount.Load(), "expected at least one PR list call")
	assert.Positive(t, checkRunCount.Load(), "expected at least one check-run call")
	assert.Positive(t, reviewRequestCount.Load(), "expected at least one review request call")
	assert.Positive(t, reviewCount.Load(), "expected at least one review status call")
}
