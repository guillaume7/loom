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
	StoryID  string   `json:"story_id"`
	Prompt   string   `json:"prompt"`
	Worktree string   `json:"worktree"`
	Contract agentspawn.JobContract `json:"contract"`
	PID      int      `json:"pid"`
	Command  []string `json:"command"`
	Status   string   `json:"status"`
}

type backgroundAgentExitDetail struct {
	StoryID         string `json:"story_id"`
	Prompt          string `json:"prompt"`
	Worktree        string `json:"worktree"`
	Contract        agentspawn.JobContract `json:"contract"`
	PID             int    `json:"pid"`
	ExitCode        int    `json:"exit_code"`
	Success         bool   `json:"success"`
	WorktreeRemoved bool   `json:"worktree_removed"`
	CleanupError    string `json:"cleanup_error,omitempty"`
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
	go s.awaitBackgroundAgentExit(sessionID, handle.Done())

	slog.InfoContext(ctx, "tool completed", "tool", toolName, "story_id", storyID, "pid", started.PID, "duration_ms", time.Since(start).Milliseconds())
	return toolResultJSON(result), nil
}

func (s *Server) awaitBackgroundAgentExit(sessionID string, done <-chan agentspawn.Exit) {
	result, ok := <-done
	if !ok {
		return
	}

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
		OperationKey: fmt.Sprintf("background_agent_exit:%s:%s:%d:%d", sessionID, result.Started.StoryID, result.Started.PID, time.Now().UTC().UnixNano()),
		StateBefore:  "background_agent_running",
		StateAfter:   "background_agent_exited",
		Event:        "background_agent_exited",
		Detail:       string(payload),
	}); err != nil {
		slog.Error("background agent exit action log write failed", "story_id", result.Started.StoryID, "pid", result.Started.PID, "error", err)
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
