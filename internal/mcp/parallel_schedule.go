package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/guillaume7/loom/internal/agentspawn"
	"github.com/guillaume7/loom/internal/depgraph"
	loomgh "github.com/guillaume7/loom/internal/github"
	"github.com/guillaume7/loom/internal/gitworktree"
	"github.com/guillaume7/loom/internal/store"
	mcplib "github.com/mark3labs/mcp-go/mcp"
)

const (
	defaultMaxParallel          = 3
	defaultRateLimitRemaining   = 100
	minScheduleActionReadLimit  = 200
	scheduleActionReadPerStory  = 32
	scheduleStateBefore         = "parallel_scheduler_idle"
	scheduleStateAfterScheduled = "parallel_scheduler_scheduled"
	scheduleStateAfterDeferred  = "parallel_scheduler_deferred"
	scheduleStateAfterWaiting   = "parallel_scheduler_waiting"
	scheduleStateAfterComplete  = "parallel_scheduler_complete"
	scheduleStateAfterIdle      = "parallel_scheduler_idle"
)

// SchedulerConfig configures DAG-aware parallel story scheduling.
type SchedulerConfig struct {
	MaxParallel           int
	MinRateLimitRemaining int
}

// DefaultSchedulerConfig returns the default parallel scheduling settings.
func DefaultSchedulerConfig() SchedulerConfig {
	return SchedulerConfig{
		MaxParallel:           defaultMaxParallel,
		MinRateLimitRemaining: defaultRateLimitRemaining,
	}
}

func normalizeSchedulerConfig(cfg SchedulerConfig) SchedulerConfig {
	defaults := DefaultSchedulerConfig()
	if cfg.MaxParallel <= 0 {
		cfg.MaxParallel = defaults.MaxParallel
	}
	if cfg.MinRateLimitRemaining <= 0 {
		cfg.MinRateLimitRemaining = defaults.MinRateLimitRemaining
	}
	return cfg
}

type githubRateLimitClient interface {
	RateLimit(ctx context.Context) (loomgh.RateLimit, error)
}

// ScheduleEpicResult describes one scheduler evaluation pass.
type ScheduleEpicResult struct {
	Status           string                       `json:"status"`
	Detail           string                       `json:"detail,omitempty"`
	MaxParallel      int                          `json:"max_parallel"`
	AvailableSlots   int                          `json:"available_slots"`
	UnblockedStories []string                     `json:"unblocked_stories"`
	RunningStories   []string                     `json:"running_stories"`
	CompletedStories []string                     `json:"completed_stories"`
	DeferredStories  []string                     `json:"deferred_stories"`
	Spawned          []BackgroundAgentSpawnResult `json:"spawned"`
	Failed           []ScheduleSpawnFailure       `json:"failed,omitempty"`
	RateLimit        ScheduleRateLimitResult      `json:"rate_limit"`
	EpicComplete     bool                         `json:"epic_complete"`
}

// ScheduleSpawnFailure captures a single story spawn failure.
type ScheduleSpawnFailure struct {
	StoryID string `json:"story_id"`
	Error   string `json:"error"`
}

// ScheduleRateLimitResult describes the budget check performed before spawning.
type ScheduleRateLimitResult struct {
	Checked   bool   `json:"checked"`
	Limit     int    `json:"limit,omitempty"`
	Remaining int    `json:"remaining,omitempty"`
	Threshold int    `json:"threshold,omitempty"`
	ResetAt   string `json:"reset_at,omitempty"`
}

type scheduleStoryState struct {
	completed   map[string]struct{}
	spawnCount  map[string]int
	exitCount   map[string]int
	failedCount map[string]int
}

func (s *Server) handleScheduleEpic(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	const toolName = "loom_schedule_epic"
	start := time.Now()

	if res, cancelled := checkCtx(ctx, toolName); cancelled {
		return res, nil
	}

	operationKey, hasOperationKey, err := optionalStringArgument(req, "operation_key")
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}

	s.schedulerMu.Lock()
	defer s.schedulerMu.Unlock()

	if hasOperationKey {
		cached, found, lookupErr := s.readActionByOperationKey(ctx, toolName, operationKey)
		if lookupErr != nil {
			return lookupErr, nil
		}
		if found {
			slog.InfoContext(ctx, "tool completed", "tool", toolName, "operation_key", operationKey, "cached", true, "duration_ms", time.Since(start).Milliseconds())
			return mcplib.NewToolResultText(cached.Detail), nil
		}
	}

	graph, err := depgraph.Load(".loom/dependencies.yaml")
	if err != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("failed to load dependency graph: %v", err)), nil
	}

	storyIDs := graphStoryIDs(graph)
	actions, err := s.st.ReadActions(ctx, scheduleActionReadLimit(len(storyIDs)))
	if err != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("failed to read action log: %v", err)), nil
	}

	state := collectScheduleStoryState(storyIDs, actions)
	completedStories := sortedStoryIDs(state.completed)
	unblockedStories := graph.Unblocked(completedStories)
	runningStories := collectRunningStories(state)
	availableSlots := maxInt(0, s.schedCfg.MaxParallel-len(runningStories))
	runningSet := setFromSlice(runningStories)
	candidates := difference(unblockedStories, runningSet)

	result := ScheduleEpicResult{
		MaxParallel:      s.schedCfg.MaxParallel,
		AvailableSlots:   availableSlots,
		UnblockedStories: unblockedStories,
		RunningStories:   runningStories,
		CompletedStories: completedStories,
		DeferredStories:  []string{},
		Spawned:          []BackgroundAgentSpawnResult{},
		Failed:           []ScheduleSpawnFailure{},
		EpicComplete:     len(storyIDs) == len(completedStories),
	}

	switch {
	case result.EpicComplete:
		result.Status = "complete"
		result.Detail = "all stories complete"
	case len(candidates) == 0:
		result.Status = "idle"
		result.Detail = "no newly unblocked stories to spawn"
	case availableSlots == 0:
		result.Status = "waiting_for_slot"
		result.Detail = "max_parallel limit reached; waiting for a slot to become available"
		result.DeferredStories = append(result.DeferredStories, candidates...)
	default:
		rateBudget, budgetErr := s.currentRateLimit(ctx)
		if budgetErr != nil {
			return mcplib.NewToolResultError(budgetErr.Error()), nil
		}
		result.RateLimit = scheduleRateLimitResult(rateBudget, s.schedCfg.MinRateLimitRemaining)
		if rateBudget.Remaining < s.schedCfg.MinRateLimitRemaining {
			result.Status = "deferred"
			result.Detail = fmt.Sprintf(
				"rate-limit budget low: remaining %d below threshold %d; deferring spawn until budget recovers",
				rateBudget.Remaining,
				s.schedCfg.MinRateLimitRemaining,
			)
			result.DeferredStories = append(result.DeferredStories, candidates...)
			slog.WarnContext(ctx, "deferring background agent spawn due to low GitHub rate-limit budget",
				"remaining", rateBudget.Remaining,
				"threshold", s.schedCfg.MinRateLimitRemaining,
				"reset", rateBudget.Reset,
			)
		} else {
			sessionID := sessionIDFromContext(ctx)
			spawnLimit := minInt(availableSlots, len(candidates))
			for _, storyID := range candidates[:spawnLimit] {
				prompt := fmt.Sprintf("Implement %s", storyID)
				attempt := nextAttemptNumber(storyID, actions)
				contract := newAgentJobContract(time.Now().UTC(), sessionID, storyID, prompt, attempt)
				handle, spawnErr := s.spawner.Spawn(agentspawn.Request{
					StoryID:  storyID,
					Prompt:   prompt,
					Worktree: gitworktree.ManagedName(storyID),
					Contract: contract,
				})
				if spawnErr != nil {
					result.Failed = append(result.Failed, ScheduleSpawnFailure{
						StoryID: storyID,
						Error:   spawnErr.Error(),
					})
					continue
				}

				started := handle.Started()
				spawned := BackgroundAgentSpawnResult{
					StoryID:  started.StoryID,
					Prompt:   started.Prompt,
					Worktree: started.Worktree,
					Contract: started.Contract,
					PID:      started.PID,
					Command:  started.Command(),
					Status:   "running",
				}
				result.Spawned = append(result.Spawned, spawned)
				s.logBackgroundAgentSpawn(ctx, sessionID, started)
				go s.awaitBackgroundAgentExit(sessionID, started, handle.Done())
			}
			result.DeferredStories = append(result.DeferredStories, candidates[spawnLimit:]...)
			switch {
			case len(result.Spawned) > 0 && len(result.Failed) == 0:
				result.Status = "scheduled"
				result.Detail = fmt.Sprintf("spawned %d unblocked stor%s", len(result.Spawned), pluralSuffix(len(result.Spawned), "y", "ies"))
			case len(result.Spawned) > 0:
				result.Status = "partial"
				result.Detail = fmt.Sprintf("spawned %d stor%s with %d failure%s", len(result.Spawned), pluralSuffix(len(result.Spawned), "y", "ies"), len(result.Failed), pluralSuffix(len(result.Failed), "", "s"))
			default:
				result.Status = "spawn_failed"
				result.Detail = "all eligible story spawns failed"
			}
		}
	}

	payload, err := marshalResultText(result)
	if err != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("failed to marshal result: %v", err)), nil
	}

	action := store.Action{
		SessionID:    sessionIDFromContext(ctx),
		OperationKey: scheduleOperationKey(operationKey, hasOperationKey),
		StateBefore:  scheduleStateBefore,
		StateAfter:   scheduleStateAfter(result.Status),
		Event:        scheduleEvent(result.Status),
		Detail:       payload,
	}
	if writeErr := s.st.WriteAction(ctx, action); writeErr != nil {
		if errorsText := cachedScheduleResult(ctx, s, toolName, action.OperationKey, writeErr); errorsText != nil {
			return errorsText, nil
		}
		return mcplib.NewToolResultError(fmt.Sprintf("failed to write schedule action: %v", writeErr)), nil
	}

	slog.InfoContext(ctx, "tool completed", "tool", toolName, "status", result.Status, "spawned", len(result.Spawned), "duration_ms", time.Since(start).Milliseconds())
	return mcplib.NewToolResultText(payload), nil
}

func (s *Server) currentRateLimit(ctx context.Context) (loomgh.RateLimit, error) {
	if s.gh == nil {
		return loomgh.RateLimit{}, fmt.Errorf("github client is required for loom_schedule_epic")
	}
	checker, ok := s.gh.(githubRateLimitClient)
	if !ok {
		return loomgh.RateLimit{}, fmt.Errorf("github client does not expose rate-limit budget")
	}
	budget, err := checker.RateLimit(ctx)
	if err != nil {
		return loomgh.RateLimit{}, fmt.Errorf("failed to read GitHub rate-limit budget: %w", err)
	}
	return budget, nil
}

func graphStoryIDs(graph depgraph.Graph) []string {
	ids := make([]string, 0)
	for _, epic := range graph.Epics {
		for _, story := range epic.Stories {
			ids = append(ids, story.ID)
		}
	}
	sort.Strings(ids)
	return ids
}

func collectScheduleStoryState(storyIDs []string, actions []store.Action) scheduleStoryState {
	state := scheduleStoryState{
		completed:   make(map[string]struct{}, len(storyIDs)),
		spawnCount:  make(map[string]int, len(storyIDs)),
		exitCount:   make(map[string]int, len(storyIDs)),
		failedCount: make(map[string]int, len(storyIDs)),
	}
	for _, action := range actions {
		switch action.Event {
		case "background_agent_spawned":
			if storyID := storyIDFromActionDetail(action.Detail); storyID != "" {
				state.spawnCount[storyID]++
			}
		case "background_agent_exited":
			if storyID := storyIDFromActionDetail(action.Detail); storyID != "" {
				state.exitCount[storyID]++
			}
		case "background_agent_failed":
			if storyID := storyIDFromActionDetail(action.Detail); storyID != "" {
				state.failedCount[storyID]++
			}
		case "merged", "merged_epic_boundary", "refactor_merged":
			if storyID := storyIDFromOperationKey(action.OperationKey, storyIDs); storyID != "" {
				state.completed[storyID] = struct{}{}
			}
		}
	}
	return state
}

func collectRunningStories(state scheduleStoryState) []string {
	running := make([]string, 0, len(state.spawnCount))
	for storyID, spawns := range state.spawnCount {
		if _, done := state.completed[storyID]; done {
			continue
		}
		terminalAttempts := maxInt(state.exitCount[storyID], state.failedCount[storyID])
		if spawns <= terminalAttempts {
			continue
		}
		running = append(running, storyID)
	}
	sort.Strings(running)
	return running
}

func sortedStoryIDs(stories map[string]struct{}) []string {
	ids := make([]string, 0, len(stories))
	for storyID := range stories {
		ids = append(ids, storyID)
	}
	sort.Strings(ids)
	return ids
}

func difference(stories []string, excluded map[string]struct{}) []string {
	result := make([]string, 0, len(stories))
	for _, storyID := range stories {
		if _, found := excluded[storyID]; found {
			continue
		}
		result = append(result, storyID)
	}
	return result
}

func storyIDFromActionDetail(detail string) string {
	if strings.TrimSpace(detail) == "" {
		return ""
	}
	var payload struct {
		StoryID string `json:"story_id"`
	}
	if err := json.Unmarshal([]byte(detail), &payload); err != nil {
		return ""
	}
	return payload.StoryID
}

func storyIDFromOperationKey(operationKey string, storyIDs []string) string {
	for _, storyID := range storyIDs {
		if strings.HasPrefix(operationKey, storyID+":") {
			return storyID
		}
	}
	return ""
}

func scheduleRateLimitResult(budget loomgh.RateLimit, threshold int) ScheduleRateLimitResult {
	result := ScheduleRateLimitResult{
		Checked:   true,
		Limit:     budget.Limit,
		Remaining: budget.Remaining,
		Threshold: threshold,
	}
	if !budget.Reset.IsZero() {
		result.ResetAt = budget.Reset.UTC().Format(time.RFC3339)
	}
	return result
}

func scheduleActionReadLimit(totalStories int) int {
	limit := totalStories * scheduleActionReadPerStory
	if limit < minScheduleActionReadLimit {
		limit = minScheduleActionReadLimit
	}
	return limit
}

func scheduleOperationKey(operationKey string, hasOperationKey bool) string {
	if hasOperationKey {
		return operationKey
	}
	return fmt.Sprintf("parallel_schedule:%d", time.Now().UTC().UnixNano())
}

func scheduleStateAfter(status string) string {
	switch status {
	case "complete":
		return scheduleStateAfterComplete
	case "deferred":
		return scheduleStateAfterDeferred
	case "waiting_for_slot":
		return scheduleStateAfterWaiting
	case "scheduled", "partial", "spawn_failed":
		return scheduleStateAfterScheduled
	default:
		return scheduleStateAfterIdle
	}
}

func scheduleEvent(status string) string {
	switch status {
	case "complete":
		return "parallel_epic_complete"
	case "deferred":
		return "parallel_schedule_deferred"
	default:
		return "parallel_schedule"
	}
}

func cachedScheduleResult(ctx context.Context, s *Server, toolName, operationKey string, err error) *mcplib.CallToolResult {
	if !errors.Is(err, store.ErrDuplicateOperationKey) {
		return nil
	}
	cached, found, lookupErr := s.readActionByOperationKey(ctx, toolName, operationKey)
	if lookupErr != nil {
		return lookupErr
	}
	if !found {
		return mcplib.NewToolResultError(fmt.Sprintf("duplicate operation key without cached result: %s", operationKey))
	}
	return mcplib.NewToolResultText(cached.Detail)
}

func pluralSuffix(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func setFromSlice(items []string) map[string]struct{} {
	set := make(map[string]struct{}, len(items))
	for _, item := range items {
		set[item] = struct{}{}
	}
	return set
}
