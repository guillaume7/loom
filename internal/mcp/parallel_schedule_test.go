package mcp_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/guillaume7/loom/internal/agentspawn"
	"github.com/guillaume7/loom/internal/fsm"
	loomgh "github.com/guillaume7/loom/internal/github"
	"github.com/guillaume7/loom/internal/mcp"
	"github.com/guillaume7/loom/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoomScheduleEpic_SpawnsAllUnblockedStoriesUpToDefaultLimit(t *testing.T) {
	withDependenciesFile(t, `version: 1
epics:
  - id: E2
    depends_on: []
    stories:
      - id: US-2.1
        depends_on: []
      - id: US-2.2
        depends_on: []
      - id: US-2.3
        depends_on: []
`, func() {
		gh := &scheduleGitHubClientMock{budget: loomgh.RateLimit{Limit: 5000, Remaining: 4000, Reset: time.Unix(1_700_000_000, 0)}}
		spawner := newScheduleSpawner()
		machine := fsm.NewMachine(fsm.DefaultConfig())
		st := newMemStore()
		s := mcp.NewServer(machine, st, gh, mcp.WithSpawner(spawner))

		result := callTool(t, s.MCPServer(), "loom_schedule_epic", nil)
		require.False(t, result.IsError, toolText(t, result))

		var got mcp.ScheduleEpicResult
		require.NoError(t, json.Unmarshal([]byte(toolText(t, result)), &got))

		assert.Equal(t, "scheduled", got.Status)
		assert.Equal(t, 3, got.MaxParallel)
		assert.Equal(t, []string{"US-2.1", "US-2.2", "US-2.3"}, got.UnblockedStories)
		assert.Len(t, got.Spawned, 3)
		assert.Empty(t, got.DeferredStories)
		assert.True(t, got.RateLimit.Checked)
		assert.Equal(t, 1, gh.calls)
		assert.Equal(t, []string{"US-2.1", "US-2.2", "US-2.3"}, spawner.spawnedStoryIDs())
		for _, spawned := range got.Spawned {
			assert.Equal(t, spawned.StoryID, spawned.Contract.StoryID)
			assert.Equal(t, 1, spawned.Contract.Attempt)
			assert.Equal(t, spawned.Prompt, spawned.Contract.Input)
			assert.Equal(t, "Implement the requested story and return completed code changes with tests", spawned.Contract.ExpectedOutput)
			assert.False(t, spawned.Contract.Deadline.IsZero())
		}
		for _, req := range spawner.requestsSnapshot() {
			assert.Equal(t, req.StoryID, req.Contract.StoryID)
			assert.Equal(t, req.Prompt, req.Contract.Input)
			assert.Equal(t, 1, req.Contract.Attempt)
			assert.False(t, req.Contract.Deadline.IsZero())
		}
	})
}

func TestLoomScheduleEpic_RespectsConcurrencyLimitAndFillsFreedSlots(t *testing.T) {
	withDependenciesFile(t, `version: 1
epics:
  - id: E2
    depends_on: []
    stories:
      - id: US-2.1
        depends_on: []
      - id: US-2.2
        depends_on: []
      - id: US-2.3
        depends_on: []
      - id: US-2.4
        depends_on: []
      - id: US-2.5
        depends_on: []
`, func() {
		gh := &scheduleGitHubClientMock{budget: loomgh.RateLimit{Limit: 5000, Remaining: 4000}}
		spawner := newScheduleSpawner()
		machine := fsm.NewMachine(fsm.DefaultConfig())
		st := newMemStore()
		s := mcp.NewServer(machine, st, gh,
			mcp.WithSpawner(spawner),
			mcp.WithSchedulerConfig(mcp.SchedulerConfig{MaxParallel: 3}),
		)
		mcpSvr := s.MCPServer()

		first := callTool(t, mcpSvr, "loom_schedule_epic", nil)
		require.False(t, first.IsError, toolText(t, first))
		var firstResult mcp.ScheduleEpicResult
		require.NoError(t, json.Unmarshal([]byte(toolText(t, first)), &firstResult))
		assert.Len(t, firstResult.Spawned, 3)
		assert.Equal(t, []string{"US-2.4", "US-2.5"}, firstResult.DeferredStories)

		require.NoError(t, st.WriteAction(context.Background(), store.Action{
			SessionID:    "story-US-2.1",
			OperationKey: "US-2.1:checkpoint:MERGING->SCANNING:merged",
			StateBefore:  "MERGING",
			StateAfter:   "SCANNING",
			Event:        "merged",
			Detail:       `{"story_id":"US-2.1"}`,
		}))

		second := callTool(t, mcpSvr, "loom_schedule_epic", nil)
		require.False(t, second.IsError, toolText(t, second))
		var secondResult mcp.ScheduleEpicResult
		require.NoError(t, json.Unmarshal([]byte(toolText(t, second)), &secondResult))
		assert.Len(t, secondResult.Spawned, 1)
		assert.Equal(t, "US-2.4", secondResult.Spawned[0].StoryID)
		assert.Equal(t, []string{"US-2.5"}, secondResult.DeferredStories)
		assert.Equal(t, []string{"US-2.2", "US-2.3"}, secondResult.RunningStories)
	})
}

func TestLoomScheduleEpic_ReevaluatesDagAndSpawnsNewlyUnblockedDependent(t *testing.T) {
	withDependenciesFile(t, `version: 1
epics:
  - id: E2
    depends_on: []
    stories:
      - id: US-2.1
        depends_on: []
      - id: US-2.4
        depends_on: [US-2.1]
`, func() {
		gh := &scheduleGitHubClientMock{budget: loomgh.RateLimit{Limit: 5000, Remaining: 4000}}
		spawner := newScheduleSpawner()
		machine := fsm.NewMachine(fsm.DefaultConfig())
		st := newMemStore()
		s := mcp.NewServer(machine, st, gh, mcp.WithSpawner(spawner))
		mcpSvr := s.MCPServer()

		first := callTool(t, mcpSvr, "loom_schedule_epic", nil)
		require.False(t, first.IsError, toolText(t, first))
		var firstResult mcp.ScheduleEpicResult
		require.NoError(t, json.Unmarshal([]byte(toolText(t, first)), &firstResult))
		assert.Equal(t, []string{"US-2.1"}, spawnedIDs(firstResult.Spawned))

		require.NoError(t, st.WriteAction(context.Background(), store.Action{
			SessionID:    "story-US-2.1",
			OperationKey: "US-2.1:checkpoint:MERGING->SCANNING:merged",
			StateBefore:  "MERGING",
			StateAfter:   "SCANNING",
			Event:        "merged",
			Detail:       `{"story_id":"US-2.1"}`,
		}))

		second := callTool(t, mcpSvr, "loom_schedule_epic", nil)
		require.False(t, second.IsError, toolText(t, second))
		var secondResult mcp.ScheduleEpicResult
		require.NoError(t, json.Unmarshal([]byte(toolText(t, second)), &secondResult))
		assert.Equal(t, []string{"US-2.4"}, spawnedIDs(secondResult.Spawned))
		assert.Equal(t, []string{"US-2.4"}, secondResult.UnblockedStories)
	})
}

func TestLoomScheduleEpic_DefersWhenRateLimitLowUntilRecovered(t *testing.T) {
	withDependenciesFile(t, `version: 1
epics:
  - id: E2
    depends_on: []
    stories:
      - id: US-2.1
        depends_on: []
`, func() {
		var logs strings.Builder
		restoreLogger := captureDefaultLogger(&logs)
		defer restoreLogger()

		gh := &scheduleGitHubClientMock{budget: loomgh.RateLimit{Limit: 5000, Remaining: 99, Reset: time.Unix(1_700_000_000, 0)}}
		spawner := newScheduleSpawner()
		machine := fsm.NewMachine(fsm.DefaultConfig())
		st := newMemStore()
		s := mcp.NewServer(machine, st, gh, mcp.WithSpawner(spawner))
		mcpSvr := s.MCPServer()

		first := callTool(t, mcpSvr, "loom_schedule_epic", nil)
		require.False(t, first.IsError, toolText(t, first))
		var firstResult mcp.ScheduleEpicResult
		require.NoError(t, json.Unmarshal([]byte(toolText(t, first)), &firstResult))
		assert.Equal(t, "deferred", firstResult.Status)
		assert.Empty(t, firstResult.Spawned)
		assert.Equal(t, []string{"US-2.1"}, firstResult.DeferredStories)
		assert.Contains(t, logs.String(), "deferring background agent spawn due to low GitHub rate-limit budget")

		gh.setBudget(loomgh.RateLimit{Limit: 5000, Remaining: 150})
		second := callTool(t, mcpSvr, "loom_schedule_epic", nil)
		require.False(t, second.IsError, toolText(t, second))
		var secondResult mcp.ScheduleEpicResult
		require.NoError(t, json.Unmarshal([]byte(toolText(t, second)), &secondResult))
		assert.Equal(t, "scheduled", secondResult.Status)
		assert.Equal(t, []string{"US-2.1"}, spawnedIDs(secondResult.Spawned))
	})
}

func TestLoomScheduleEpic_MarksEpicCompleteWhenAllStoriesDone(t *testing.T) {
	withDependenciesFile(t, `version: 1
epics:
  - id: E2
    depends_on: []
    stories:
      - id: US-2.1
        depends_on: []
      - id: US-2.2
        depends_on: []
`, func() {
		gh := &scheduleGitHubClientMock{budget: loomgh.RateLimit{Limit: 5000, Remaining: 4000}}
		spawner := newScheduleSpawner()
		machine := fsm.NewMachine(fsm.DefaultConfig())
		st := newMemStore()
		s := mcp.NewServer(machine, st, gh, mcp.WithSpawner(spawner))

		for _, storyID := range []string{"US-2.1", "US-2.2"} {
			require.NoError(t, st.WriteAction(context.Background(), store.Action{
				SessionID:    "story-" + storyID,
				OperationKey: storyID + ":checkpoint:MERGING->SCANNING:merged",
				StateBefore:  "MERGING",
				StateAfter:   "SCANNING",
				Event:        "merged",
				Detail:       `{"story_id":"` + storyID + `"}`,
			}))
		}

		result := callTool(t, s.MCPServer(), "loom_schedule_epic", nil)
		require.False(t, result.IsError, toolText(t, result))
		var got mcp.ScheduleEpicResult
		require.NoError(t, json.Unmarshal([]byte(toolText(t, result)), &got))
		assert.True(t, got.EpicComplete)
		assert.Equal(t, "complete", got.Status)
		assert.Empty(t, got.Spawned)

		actions, err := st.ReadActions(context.Background(), 10)
		require.NoError(t, err)
		require.NotEmpty(t, actions)
		assert.Equal(t, "parallel_epic_complete", actions[0].Event)
	})
}

type scheduleGitHubClientMock struct {
	mu     sync.Mutex
	budget loomgh.RateLimit
	err    error
	calls  int
}

func (m *scheduleGitHubClientMock) Ping(context.Context) error { return nil }

func (m *scheduleGitHubClientMock) RateLimit(context.Context) (loomgh.RateLimit, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	if m.err != nil {
		return loomgh.RateLimit{}, m.err
	}
	return m.budget, nil
}

func (m *scheduleGitHubClientMock) setBudget(budget loomgh.RateLimit) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.budget = budget
}

type scheduleSpawner struct {
	mu       sync.Mutex
	requests []agentspawn.Request
	nextPID  int
}

func newScheduleSpawner() *scheduleSpawner {
	return &scheduleSpawner{nextPID: 1000}
}

func (s *scheduleSpawner) Spawn(req agentspawn.Request) (agentspawn.SpawnHandle, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, req)
	started := agentspawn.Started{
		StoryID:  req.StoryID,
		Prompt:   req.Prompt,
		Worktree: req.Worktree,
		Contract: req.Contract,
		Path:     "/usr/bin/code",
		Args:     []string{"chat", "-m", "loom-orchestrator", "--worktree", req.Worktree, req.Prompt},
		PID:      s.nextPID,
	}
	s.nextPID++
	done := make(chan agentspawn.Exit)
	close(done)
	return scheduleSpawnHandle{started: started, done: done}, nil
}

func (s *scheduleSpawner) spawnedStoryIDs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := make([]string, 0, len(s.requests))
	for _, req := range s.requests {
		ids = append(ids, req.StoryID)
	}
	return ids
}

func (s *scheduleSpawner) requestsSnapshot() []agentspawn.Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	requests := make([]agentspawn.Request, len(s.requests))
	copy(requests, s.requests)
	return requests
}

type scheduleSpawnHandle struct {
	started agentspawn.Started
	done    <-chan agentspawn.Exit
}

func (h scheduleSpawnHandle) Started() agentspawn.Started { return h.started }

func (h scheduleSpawnHandle) Done() <-chan agentspawn.Exit { return h.done }

func withDependenciesFile(t *testing.T, content string, fn func()) {
	t.Helper()

	originalWD, err := os.Getwd()
	require.NoError(t, err)
	tempDir := t.TempDir()
	require.NoError(t, os.Chdir(tempDir))
	t.Cleanup(func() {
		require.NoError(t, os.Chdir(originalWD))
	})

	require.NoError(t, os.Mkdir(filepath.Join(tempDir, ".loom"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, ".loom", "dependencies.yaml"), []byte(content), 0o644))
	fn()
}

func TestLoomScheduleEpic_ReschedulesExitedStoryAsAttempt2(t *testing.T) {
	withDependenciesFile(t, `version: 1
epics:
  - id: E2
    depends_on: []
    stories:
      - id: US-2.1
        depends_on: []
`, func() {
		gh := &scheduleGitHubClientMock{budget: loomgh.RateLimit{Limit: 5000, Remaining: 4000}}
		spawner := newScheduleSpawner()
		machine := fsm.NewMachine(fsm.DefaultConfig())
		st := newMemStore()
		s := mcp.NewServer(machine, st, gh, mcp.WithSpawner(spawner))
		mcpSvr := s.MCPServer()

		// First pass: spawns US-2.1 with attempt 1.
		first := callTool(t, mcpSvr, "loom_schedule_epic", nil)
		require.False(t, first.IsError, toolText(t, first))
		var firstResult mcp.ScheduleEpicResult
		require.NoError(t, json.Unmarshal([]byte(toolText(t, first)), &firstResult))
		require.Len(t, firstResult.Spawned, 1)
		assert.Equal(t, "US-2.1", firstResult.Spawned[0].StoryID)
		assert.Equal(t, 1, firstResult.Spawned[0].Contract.Attempt)

		// Simulate exit of US-2.1 without merge/completion.
		require.NoError(t, st.WriteAction(context.Background(), store.Action{
			SessionID:    "test-session",
			OperationKey: "background_agent_exit:test-session:US-2.1:1234:5678",
			StateBefore:  "background_agent_running",
			StateAfter:   "background_agent_exited",
			Event:        "background_agent_exited",
			Detail:       `{"story_id":"US-2.1","exit_code":1,"success":false}`,
		}))

		// Second pass: US-2.1 is exited and not complete, so it becomes a candidate again.
		second := callTool(t, mcpSvr, "loom_schedule_epic", nil)
		require.False(t, second.IsError, toolText(t, second))
		var secondResult mcp.ScheduleEpicResult
		require.NoError(t, json.Unmarshal([]byte(toolText(t, second)), &secondResult))
		require.Len(t, secondResult.Spawned, 1)
		assert.Equal(t, "US-2.1", secondResult.Spawned[0].StoryID)
		assert.Equal(t, 2, secondResult.Spawned[0].Contract.Attempt)
		assert.Empty(t, secondResult.RunningStories)

		// Third pass: attempt 2 is now active (spawnCount=2 > exitCount=1).
		// The scheduler must not spawn attempt 3 and must report US-2.1 as running.
		third := callTool(t, mcpSvr, "loom_schedule_epic", nil)
		require.False(t, third.IsError, toolText(t, third))
		var thirdResult mcp.ScheduleEpicResult
		require.NoError(t, json.Unmarshal([]byte(toolText(t, third)), &thirdResult))
		assert.Empty(t, thirdResult.Spawned, "must not spawn attempt 3 while attempt 2 is active")
		assert.Equal(t, []string{"US-2.1"}, thirdResult.RunningStories, "attempt 2 must appear as running")
		assert.Equal(t, "idle", thirdResult.Status)
	})
}

func TestLoomScheduleEpic_ReschedulesFailedStoryWithoutExitAsAttempt2(t *testing.T) {
	withDependenciesFile(t, `version: 1
epics:
  - id: E2
    depends_on: []
    stories:
      - id: US-2.1
        depends_on: []
`, func() {
		gh := &scheduleGitHubClientMock{budget: loomgh.RateLimit{Limit: 5000, Remaining: 4000}}
		spawner := newScheduleSpawner()
		machine := fsm.NewMachine(fsm.DefaultConfig())
		st := newMemStore()
		s := mcp.NewServer(machine, st, gh, mcp.WithSpawner(spawner))
		mcpSvr := s.MCPServer()

		// First pass: spawns US-2.1 with attempt 1.
		first := callTool(t, mcpSvr, "loom_schedule_epic", nil)
		require.False(t, first.IsError, toolText(t, first))
		var firstResult mcp.ScheduleEpicResult
		require.NoError(t, json.Unmarshal([]byte(toolText(t, first)), &firstResult))
		require.Len(t, firstResult.Spawned, 1)
		assert.Equal(t, "US-2.1", firstResult.Spawned[0].StoryID)
		assert.Equal(t, 1, firstResult.Spawned[0].Contract.Attempt)

		// Simulate failure of US-2.1 without a corresponding exit action.
		require.NoError(t, st.WriteAction(context.Background(), store.Action{
			SessionID:    "test-session",
			OperationKey: "background_agent_failed:test-session:US-2.1:1234:5678",
			StateBefore:  "background_agent_running",
			StateAfter:   "background_agent_failed",
			Event:        "background_agent_failed",
			Detail:       `{"story_id":"US-2.1","failure_class":"timeout_watchdog","failure_reason":"agent execution timed out"}`,
		}))

		// Second pass: US-2.1 failed and is not complete, so it becomes a candidate again.
		second := callTool(t, mcpSvr, "loom_schedule_epic", nil)
		require.False(t, second.IsError, toolText(t, second))
		var secondResult mcp.ScheduleEpicResult
		require.NoError(t, json.Unmarshal([]byte(toolText(t, second)), &secondResult))
		require.Len(t, secondResult.Spawned, 1)
		assert.Equal(t, "US-2.1", secondResult.Spawned[0].StoryID)
		assert.Equal(t, 2, secondResult.Spawned[0].Contract.Attempt)
		assert.Empty(t, secondResult.RunningStories)

		// Third pass: attempt 2 is now active (spawnCount=2 > max(exitCount=0, failedCount=1)).
		// The scheduler must not spawn attempt 3 and must report US-2.1 as running.
		third := callTool(t, mcpSvr, "loom_schedule_epic", nil)
		require.False(t, third.IsError, toolText(t, third))
		var thirdResult mcp.ScheduleEpicResult
		require.NoError(t, json.Unmarshal([]byte(toolText(t, third)), &thirdResult))
		assert.Empty(t, thirdResult.Spawned, "must not spawn attempt 3 while attempt 2 is active")
		assert.Equal(t, []string{"US-2.1"}, thirdResult.RunningStories, "attempt 2 must appear as running")
		assert.Equal(t, "idle", thirdResult.Status)
	})
}

func TestLoomScheduleEpic_AttemptIncrementedFromPriorSpawnActions(t *testing.T) {
	withDependenciesFile(t, `version: 1
epics:
  - id: E2
    depends_on: []
    stories:
      - id: US-2.1
        depends_on: []
      - id: US-2.2
        depends_on: []
`, func() {
		gh := &scheduleGitHubClientMock{budget: loomgh.RateLimit{Limit: 5000, Remaining: 4000}}
		spawner := newScheduleSpawner()
		machine := fsm.NewMachine(fsm.DefaultConfig())
		st := newMemStore()
		s := mcp.NewServer(machine, st, gh, mcp.WithSpawner(spawner))
		mcpSvr := s.MCPServer()

		// Seed a prior spawn event for US-2.1 to simulate a previous session.
		require.NoError(t, st.WriteAction(context.Background(), store.Action{
			SessionID:    "prior-session",
			OperationKey: "background_agent_spawn:prior-session:US-2.1:999:111",
			StateBefore:  "background_agent_pending",
			StateAfter:   "background_agent_running",
			Event:        "background_agent_spawned",
			Detail:       `{"story_id":"US-2.1"}`,
		}))
		// Seed exit so US-2.1 is not counted as running.
		require.NoError(t, st.WriteAction(context.Background(), store.Action{
			SessionID:    "prior-session",
			OperationKey: "background_agent_exit:prior-session:US-2.1:999:222",
			StateBefore:  "background_agent_running",
			StateAfter:   "background_agent_exited",
			Event:        "background_agent_exited",
			Detail:       `{"story_id":"US-2.1","exit_code":1,"success":false}`,
		}))

		result := callTool(t, mcpSvr, "loom_schedule_epic", nil)
		require.False(t, result.IsError, toolText(t, result))
		var got mcp.ScheduleEpicResult
		require.NoError(t, json.Unmarshal([]byte(toolText(t, result)), &got))
		require.Len(t, got.Spawned, 2)

		attemptByStory := make(map[string]int, 2)
		for _, spawned := range got.Spawned {
			attemptByStory[spawned.StoryID] = spawned.Contract.Attempt
		}
		// US-2.1 had one prior spawn action → attempt 2.
		assert.Equal(t, 2, attemptByStory["US-2.1"])
		// US-2.2 had no prior spawn action → attempt 1.
		assert.Equal(t, 1, attemptByStory["US-2.2"])
	})
}

func spawnedIDs(spawned []mcp.BackgroundAgentSpawnResult) []string {
	ids := make([]string, 0, len(spawned))
	for _, item := range spawned {
		ids = append(ids, item.StoryID)
	}
	return ids
}

func captureDefaultLogger(dst *strings.Builder) func() {
	previous := slog.Default()
	logger := slog.New(slog.NewTextHandler(dst, nil))
	slog.SetDefault(logger)
	return func() {
		slog.SetDefault(previous)
	}
}
