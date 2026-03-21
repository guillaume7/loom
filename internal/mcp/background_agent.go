package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/guillaume7/loom/internal/agentspawn"
	"github.com/guillaume7/loom/internal/store"
	mcplib "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// BackgroundAgentSpawnResult is returned by loom_spawn_agent.
type BackgroundAgentSpawnResult struct {
	StoryID  string                 `json:"story_id"`
	Prompt   string                 `json:"prompt"`
	Worktree string                 `json:"worktree"`
	Contract agentspawn.JobContract `json:"contract"`
	PID      int                    `json:"pid"`
	Command  []string               `json:"command"`
	Status   string                 `json:"status"`
}

type backgroundAgentExitDetail struct {
	StoryID         string                 `json:"story_id"`
	Prompt          string                 `json:"prompt"`
	Worktree        string                 `json:"worktree"`
	Contract        agentspawn.JobContract `json:"contract"`
	PID             int                    `json:"pid"`
	ExitCode        int                    `json:"exit_code"`
	Success         bool                   `json:"success"`
	WorktreeRemoved bool                   `json:"worktree_removed"`
	CleanupError    string                 `json:"cleanup_error,omitempty"`
}

// backgroundAgentFailedDetail is the payload written to the store when an
// agent job exits with an explicit failure. It carries enough context for
// operator review and eventual replay, satisfying AC1 and AC3.
type backgroundAgentFailedDetail struct {
	StoryID      string                 `json:"story_id"`
	Contract     agentspawn.JobContract `json:"contract"`
	PID          int                    `json:"pid"`
	ExitCode     int                    `json:"exit_code"`
	FailureKind  AgentJobFailureKind    `json:"failure_kind"`
	Outcome      AgentJobFailureOutcome `json:"outcome"`
	LockState    string                 `json:"lock_state"`
	Error        string                 `json:"error,omitempty"`
	CleanupError string                 `json:"cleanup_error,omitempty"`
	ObservedAt   time.Time              `json:"observed_at"`
}

func (s *Server) handleSpawnAgent(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	const toolName = "loom_spawn_agent"
	start := time.Now()

	if res, cancelled := checkCtx(ctx, toolName); cancelled {
		return res, nil
	}

	storyID, ok, err := optionalStringArgument(req, "story_id")
	if err != nil || !ok {
		return mcplib.NewToolResultError("missing or invalid 'story_id' argument: must be a non-empty string"), nil
	}
	prompt, ok, err := optionalStringArgument(req, "prompt")
	if err != nil || !ok {
		return mcplib.NewToolResultError("missing or invalid 'prompt' argument: must be a non-empty string"), nil
	}
	worktree, ok, err := optionalStringArgument(req, "worktree")
	if err != nil || !ok {
		return mcplib.NewToolResultError("missing or invalid 'worktree' argument: must be a non-empty string"), nil
	}
	slog.InfoContext(ctx, "tool called", "tool", toolName, "story_id", storyID, "worktree", worktree)
	sessionID := sessionIDFromContext(ctx)
	actions, readErr := s.st.ReadActions(ctx, minScheduleActionReadLimit)
	if readErr != nil {
		slog.WarnContext(ctx, "failed to read actions for attempt counting; defaulting to attempt 1", "story_id", storyID, "error", readErr)
		actions = nil
	}
	attempt := nextAttemptNumber(storyID, actions)
	contract := newAgentJobContract(time.Now().UTC(), sessionID, storyID, prompt, attempt)

	handle, spawnErr := s.spawner.Spawn(agentspawn.Request{
		StoryID:  storyID,
		Prompt:   prompt,
		Worktree: worktree,
		Contract: contract,
	})
	if spawnErr != nil {
		return mcplib.NewToolResultError(spawnErr.Error()), nil
	}

	started := handle.Started()
	result := BackgroundAgentSpawnResult{
		StoryID:  started.StoryID,
		Prompt:   started.Prompt,
		Worktree: started.Worktree,
		Contract: started.Contract,
		PID:      started.PID,
		Command:  started.Command(),
		Status:   "running",
	}

	s.logBackgroundAgentSpawn(ctx, sessionID, started)
	go s.awaitBackgroundAgentExit(sessionID, started, handle.Done())

	slog.InfoContext(ctx, "tool completed", "tool", toolName, "story_id", storyID, "pid", started.PID, "duration_ms", time.Since(start).Milliseconds())
	return toolResultJSON(result), nil
}

func (s *Server) awaitBackgroundAgentExit(sessionID string, started agentspawn.Started, done <-chan agentspawn.Exit) {
	result, ok, timeoutObservedAt, timedOut := s.waitForBackgroundAgentResult(started.Contract.Deadline, done)
	if timedOut {
		s.writeBackgroundAgentFailedTimeoutAction(sessionID, started, timeoutObservedAt)
		go s.logBackgroundAgentExitOnArrival(sessionID, done)
		return
	}
	if !ok {
		return
	}

	observedAt := time.Now().UTC()
	s.logBackgroundAgentExitedAction(sessionID, result, observedAt)

	if malformedOutput := result.ExitCode == 0 && result.CleanupErr == nil && result.Output != "" && !isValidAgentOutputPayload(result.Output); malformedOutput {
		s.writeBackgroundAgentFailedAction(sessionID, result, ClassifyMalformedOutput(), observedAt)
		return
	}

	if failure, failed := ClassifyAgentJobExit(result, result.Started.Contract, observedAt); failed {
		s.writeBackgroundAgentFailedAction(sessionID, result, failure, observedAt)
	}
}

func (s *Server) waitForBackgroundAgentResult(deadline time.Time, done <-chan agentspawn.Exit) (agentspawn.Exit, bool, time.Time, bool) {
	if deadline.IsZero() {
		result, ok := <-done
		return result, ok, time.Time{}, false
	}

	now := time.Now().UTC()
	if !now.Before(deadline) {
		return agentspawn.Exit{}, false, now, true
	}

	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()

	select {
	case result, ok := <-done:
		return result, ok, time.Time{}, false
	case timeoutObservedAt := <-timer.C:
		return agentspawn.Exit{}, false, timeoutObservedAt.UTC(), true
	}
}

func (s *Server) logBackgroundAgentExitOnArrival(sessionID string, done <-chan agentspawn.Exit) {
	result, ok := <-done
	if !ok {
		return
	}
	s.logBackgroundAgentExitedAction(sessionID, result, time.Now().UTC())
}

func isValidAgentOutputPayload(output string) bool {
	if output == "" {
		return true
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		return false
	}
	return payload != nil
}

func (s *Server) logBackgroundAgentExitedAction(sessionID string, result agentspawn.Exit, observedAt time.Time) {
	detail := backgroundAgentExitDetail{
		StoryID:         result.Started.StoryID,
		Prompt:          result.Started.Prompt,
		Worktree:        result.Started.Worktree,
		Contract:        result.Started.Contract,
		PID:             result.Started.PID,
		ExitCode:        result.ExitCode,
		Success:         result.ExitCode == 0 && result.CleanupErr == nil,
		WorktreeRemoved: result.CleanupErr == nil,
	}
	if result.CleanupErr != nil {
		detail.CleanupError = result.CleanupErr.Error()
	}
	payload, err := json.Marshal(detail)
	if err != nil {
		slog.Error("background agent exit marshal failed", "story_id", result.Started.StoryID, "pid", result.Started.PID, "error", err)
		return
	}
	if err := s.st.WriteAction(context.Background(), store.Action{
		SessionID:    sessionID,
		OperationKey: fmt.Sprintf("background_agent_exit:%s:%s:%d:%d", sessionID, result.Started.StoryID, result.Started.PID, observedAt.UnixNano()),
		StateBefore:  "background_agent_running",
		StateAfter:   "background_agent_exited",
		Event:        "background_agent_exited",
		Detail:       string(payload),
	}); err != nil {
		slog.Error("background agent exit action log write failed", "story_id", result.Started.StoryID, "pid", result.Started.PID, "error", err)
	}
}

func (s *Server) writeBackgroundAgentFailedTimeoutAction(sessionID string, started agentspawn.Started, observedAt time.Time) {
	timeoutFailure := AgentJobFailureResult{
		Kind:      AgentJobFailureKindTimeout,
		Outcome:   AgentJobFailureOutcomeRetry,
		LockState: "recoverable",
	}
	failedDetail := backgroundAgentFailedDetail{
		StoryID:     started.StoryID,
		Contract:    started.Contract,
		PID:         started.PID,
		ExitCode:    -1,
		FailureKind: timeoutFailure.Kind,
		Outcome:     timeoutFailure.Outcome,
		LockState:   timeoutFailure.LockState,
		ObservedAt:  observedAt,
	}
	payload, err := json.Marshal(failedDetail)
	if err != nil {
		slog.Error("background agent timeout marshal failed", "story_id", started.StoryID, "pid", started.PID, "error", err)
		return
	}
	if err := s.st.WriteAction(context.Background(), store.Action{
		SessionID:    sessionID,
		OperationKey: fmt.Sprintf("background_agent_failed:%s:%s:%d:%d", sessionID, started.StoryID, started.PID, observedAt.UnixNano()),
		StateBefore:  "background_agent_running",
		StateAfter:   "background_agent_failed",
		Event:        "background_agent_failed",
		Detail:       string(payload),
	}); err != nil {
		slog.Error("background agent timeout action log write failed", "story_id", started.StoryID, "pid", started.PID, "error", err)
	}
}

func (s *Server) writeBackgroundAgentFailedAction(sessionID string, result agentspawn.Exit, failure AgentJobFailureResult, observedAt time.Time) {
	failedDetail := backgroundAgentFailedDetail{
		StoryID:     result.Started.StoryID,
		Contract:    result.Started.Contract,
		PID:         result.Started.PID,
		ExitCode:    result.ExitCode,
		FailureKind: failure.Kind,
		Outcome:     failure.Outcome,
		LockState:   failure.LockState,
		ObservedAt:  observedAt,
	}
	if result.Err != nil {
		failedDetail.Error = result.Err.Error()
	}
	if result.CleanupErr != nil {
		failedDetail.CleanupError = result.CleanupErr.Error()
	}
	payload, err := json.Marshal(failedDetail)
	if err != nil {
		slog.Error("background agent failed marshal failed", "story_id", result.Started.StoryID, "pid", result.Started.PID, "error", err)
		return
	}
	if err := s.st.WriteAction(context.Background(), store.Action{
		SessionID:    sessionID,
		OperationKey: fmt.Sprintf("background_agent_failed:%s:%s:%d:%d", sessionID, result.Started.StoryID, result.Started.PID, observedAt.UnixNano()),
		StateBefore:  "background_agent_exited",
		StateAfter:   "background_agent_failed",
		Event:        "background_agent_failed",
		Detail:       string(payload),
	}); err != nil {
		slog.Error("background agent failed action log write failed", "story_id", result.Started.StoryID, "pid", result.Started.PID, "error", err)
	}
}

func (s *Server) logBackgroundAgentSpawn(ctx context.Context, sessionID string, started agentspawn.Started) {
	payload, err := json.Marshal(BackgroundAgentSpawnResult{
		StoryID:  started.StoryID,
		Prompt:   started.Prompt,
		Worktree: started.Worktree,
		Contract: started.Contract,
		PID:      started.PID,
		Command:  started.Command(),
		Status:   "running",
	})
	if err != nil {
		slog.ErrorContext(ctx, "background agent spawn marshal failed", "story_id", started.StoryID, "pid", started.PID, "error", err)
		return
	}
	if err := s.st.WriteAction(ctx, store.Action{
		SessionID:    sessionID,
		OperationKey: fmt.Sprintf("background_agent_spawn:%s:%s:%d:%d", sessionID, started.StoryID, started.PID, time.Now().UTC().UnixNano()),
		StateBefore:  "background_agent_pending",
		StateAfter:   "background_agent_running",
		Event:        "background_agent_spawned",
		Detail:       string(payload),
	}); err != nil {
		slog.ErrorContext(ctx, "background agent spawn action log write failed", "story_id", started.StoryID, "pid", started.PID, "error", err)
	}
}

func (s *Server) registerBackgroundAgentTool(srv *mcpserver.MCPServer) {
	srv.AddTool(
		mcplib.NewTool("loom_spawn_agent",
			mcplib.WithDescription("Spawns a background loom-orchestrator agent session via the VS Code code chat CLI"),
			mcplib.WithReadOnlyHintAnnotation(false),
			mcplib.WithString("story_id",
				mcplib.Required(),
				mcplib.Description("Story identifier for the spawned background agent"),
			),
			mcplib.WithString("prompt",
				mcplib.Required(),
				mcplib.Description("User story context to send as the initial prompt"),
			),
			mcplib.WithString("worktree",
				mcplib.Required(),
				mcplib.Description("Git worktree name or path to pass through to code chat --worktree"),
			),
		),
		s.handleSpawnAgent,
	)
}
