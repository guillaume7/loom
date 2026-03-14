package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	loomgh "github.com/guillaume7/loom/internal/github"
	"github.com/guillaume7/loom/internal/store"
	mcplib "github.com/mark3labs/mcp-go/mcp"
)

const defaultHeartbeatPollIntervalSeconds = 30

// heartbeatPollingClient is the subset of GitHub operations required by
// heartbeat polling mode.
type heartbeatPollingClient interface {
	GetPR(ctx context.Context, prNumber int) (*loomgh.PR, error)
	GetCheckRuns(ctx context.Context, sha string) ([]*loomgh.CheckRun, error)
}

type ciPollSummary struct {
	TotalChecks   int
	GreenChecks   int
	PendingChecks []string
	FailedChecks  []string
}

func (s ciPollSummary) allGreen() bool {
	return s.TotalChecks > 0 && s.GreenChecks == s.TotalChecks
}

func (s ciPollSummary) progressText() string {
	if len(s.FailedChecks) > 0 {
		return fmt.Sprintf("%d/%d checks green, failures: %s", s.GreenChecks, s.TotalChecks, strings.Join(s.FailedChecks, ", "))
	}
	if len(s.PendingChecks) > 0 {
		return fmt.Sprintf("%d/%d checks green, waiting on '%s'", s.GreenChecks, s.TotalChecks, strings.Join(s.PendingChecks, "', '"))
	}
	if s.TotalChecks == 0 {
		return "0/0 checks green, waiting for check runs"
	}
	return fmt.Sprintf("%d/%d checks green, all checks passing", s.GreenChecks, s.TotalChecks)
}

func heartbeatTaskID(prNumber int) string {
	return fmt.Sprintf("loom-ci-poll-pr-%d", prNumber)
}

func (s *Server) handleHeartbeat(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	const toolName = "loom_heartbeat"
	start := time.Now()

	if res, cancelled := checkCtx(ctx, toolName); cancelled {
		return res, nil
	}

	s.mu.RLock()
	currentState := s.machine.State()
	s.mu.RUnlock()

	slog.InfoContext(ctx, "tool called", "tool", toolName, "state", string(currentState))

	cp := s.readCheckpoint(ctx, toolName)

	gate := isGateState(currentState)
	retry := 0
	if gate {
		retry = retryInSeconds
	}
	baseResult := HeartbeatResult{
		State:          string(currentState),
		Phase:          cp.Phase,
		Wait:           gate,
		RetryInSeconds: retry,
	}

	pollMode, hasPollMode, err := optionalBoolArgument(req, "poll")
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	if hasPollMode && pollMode {
		res := s.runHeartbeatPollingTask(ctx, req, cp, baseResult)
		slog.InfoContext(ctx, "tool completed", "tool", toolName, "state", string(currentState), "duration_ms", time.Since(start).Milliseconds())
		return res, nil
	}

	slog.InfoContext(ctx, "tool completed", "tool", toolName, "state", string(currentState), "duration_ms", time.Since(start).Milliseconds())
	return toolResultJSON(baseResult), nil
}

func (s *Server) runHeartbeatPollingTask(ctx context.Context, req mcplib.CallToolRequest, cp store.Checkpoint, baseResult HeartbeatResult) *mcplib.CallToolResult {
	emitter := s.Emitter()
	if emitter == nil {
		return mcplib.NewToolResultError("task emitter is not initialized")
	}

	client, ok := s.gh.(heartbeatPollingClient)
	if !ok {
		return mcplib.NewToolResultError("GitHub client does not support heartbeat polling")
	}

	prNumber, hasPRNumber, err := optionalIntArgument(req, "pr_number")
	if err != nil {
		return mcplib.NewToolResultError(err.Error())
	}
	if !hasPRNumber {
		prNumber = cp.PRNumber
	}
	if prNumber <= 0 {
		return mcplib.NewToolResultError("missing or invalid 'pr_number' argument: must be a positive integer")
	}

	pollIntervalSeconds := defaultHeartbeatPollIntervalSeconds
	if interval, hasInterval, intervalErr := optionalIntArgument(req, "poll_interval_seconds"); intervalErr != nil {
		return mcplib.NewToolResultError(intervalErr.Error())
	} else if hasInterval {
		if interval < 0 {
			return mcplib.NewToolResultError("missing or invalid 'poll_interval_seconds' argument: must be a non-negative integer")
		}
		pollIntervalSeconds = interval
	}

	maxPolls := 0
	if max, hasMaxPolls, maxErr := optionalIntArgument(req, "max_polls"); maxErr != nil {
		return mcplib.NewToolResultError(maxErr.Error())
	} else if hasMaxPolls {
		if max < 1 {
			return mcplib.NewToolResultError("missing or invalid 'max_polls' argument: must be a positive integer")
		}
		maxPolls = max
	}

	taskID := heartbeatTaskID(prNumber)
	title := fmt.Sprintf("Watching CI for PR #%d", prNumber)
	if err := emitter.Start(ctx, taskID, title, true); err != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("failed to emit task start: %v", err))
	}

	pollInterval := time.Duration(pollIntervalSeconds) * time.Second
	for polls := 1; ; polls++ {
		summary, pollErr := pollCISummary(ctx, client, prNumber)
		if pollErr != nil {
			failedResult := map[string]any{"all_green": false, "failed_checks": []string{}, "error": pollErr.Error()}
			if doneErr := emitter.Done(ctx, taskID, failedResult); doneErr != nil {
				return mcplib.NewToolResultError(fmt.Sprintf("failed to emit task done: %v", doneErr))
			}
			return mcplib.NewToolResultError(fmt.Sprintf("failed to poll CI status: %v", pollErr))
		}

		if err := emitter.Progress(ctx, taskID, summary.progressText()); err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("failed to emit task progress: %v", err))
		}

		if summary.allGreen() || len(summary.FailedChecks) > 0 {
			doneResult := map[string]any{"all_green": summary.allGreen()}
			if len(summary.FailedChecks) > 0 {
				doneResult["failed_checks"] = summary.FailedChecks
			}
			if err := emitter.Done(ctx, taskID, doneResult); err != nil {
				return mcplib.NewToolResultError(fmt.Sprintf("failed to emit task done: %v", err))
			}
			return toolResultJSON(baseResult)
		}

		if maxPolls > 0 && polls >= maxPolls {
			doneResult := map[string]any{"all_green": false, "failed_checks": []string{}}
			if err := emitter.Done(ctx, taskID, doneResult); err != nil {
				return mcplib.NewToolResultError(fmt.Sprintf("failed to emit task done: %v", err))
			}
			return toolResultJSON(baseResult)
		}

		if pollInterval > 0 {
			timer := time.NewTimer(pollInterval)
			select {
			case <-ctx.Done():
				timer.Stop()
				return mcplib.NewToolResultError(fmt.Sprintf("request cancelled: %v", ctx.Err()))
			case <-timer.C:
			}
		}
	}
}

func pollCISummary(ctx context.Context, gh heartbeatPollingClient, prNumber int) (ciPollSummary, error) {
	pr, err := gh.GetPR(ctx, prNumber)
	if err != nil {
		return ciPollSummary{}, err
	}
	if pr == nil || strings.TrimSpace(pr.HeadSHA) == "" {
		return ciPollSummary{}, fmt.Errorf("PR #%d is missing head SHA", prNumber)
	}

	checks, err := gh.GetCheckRuns(ctx, pr.HeadSHA)
	if err != nil {
		return ciPollSummary{}, err
	}

	summary := ciPollSummary{TotalChecks: len(checks)}
	for _, check := range checks {
		if check == nil {
			continue
		}
		name := strings.TrimSpace(check.Name)
		if name == "" {
			name = "unnamed"
		}

		status := strings.ToLower(strings.TrimSpace(check.Status))
		conclusion := strings.ToLower(strings.TrimSpace(check.Conclusion))

		if status != "completed" {
			summary.PendingChecks = append(summary.PendingChecks, name)
			continue
		}

		if conclusion == "success" {
			summary.GreenChecks++
			continue
		}

		if conclusion == "" {
			summary.PendingChecks = append(summary.PendingChecks, name)
			continue
		}

		summary.FailedChecks = append(summary.FailedChecks, name)
	}

	sort.Strings(summary.PendingChecks)
	sort.Strings(summary.FailedChecks)
	return summary, nil
}

func optionalBoolArgument(req mcplib.CallToolRequest, name string) (bool, bool, error) {
	v, ok := req.Params.Arguments[name]
	if !ok {
		return false, false, nil
	}
	b, ok := v.(bool)
	if !ok {
		return false, false, fmt.Errorf("missing or invalid '%s' argument: must be a boolean", name)
	}
	return b, true, nil
}

func optionalIntArgument(req mcplib.CallToolRequest, name string) (int, bool, error) {
	v, ok := req.Params.Arguments[name]
	if !ok {
		return 0, false, nil
	}

	switch n := v.(type) {
	case int:
		return n, true, nil
	case int32:
		return int(n), true, nil
	case int64:
		return int(n), true, nil
	case float64:
		asInt := int(n)
		if float64(asInt) != n {
			return 0, false, fmt.Errorf("missing or invalid '%s' argument: must be an integer", name)
		}
		return asInt, true, nil
	default:
		return 0, false, fmt.Errorf("missing or invalid '%s' argument: must be an integer", name)
	}
}
