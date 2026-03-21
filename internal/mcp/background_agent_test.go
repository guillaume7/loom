package mcp_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/guillaume7/loom/internal/mcp"
	"github.com/guillaume7/loom/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoomSpawnAgent_StartsBackgroundProcessAndLogsExitCode(t *testing.T) {
	repoRoot := initGitRepo(t)
	chdir(t, repoRoot)

	s, mcpSvr := newTestServer(t)
	argsFile := installFakeCodeCLI(t, 1, 0)

	result := callTool(t, mcpSvr, "loom_spawn_agent", map[string]interface{}{
		"story_id": "US-2.1",
		"prompt":   "Implement US-2.1",
		"worktree": "worktree-us-2.1",
	})
	require.False(t, result.IsError)

	var got mcp.BackgroundAgentSpawnResult
	require.NoError(t, json.Unmarshal([]byte(toolText(t, result)), &got))
	assert.Equal(t, "US-2.1", got.StoryID)
	assert.Equal(t, "Implement US-2.1", got.Prompt)
	assert.Equal(t, filepath.Join("..", "worktree-us-2.1"), got.Worktree)
	assert.Equal(t, "US-2.1", got.Contract.StoryID)
	assert.Equal(t, 1, got.Contract.Attempt)
	assert.Equal(t, "Implement US-2.1", got.Contract.Input)
	assert.Equal(t, "Implement the requested story and return completed code changes with tests", got.Contract.ExpectedOutput)
	assert.NotEmpty(t, got.Contract.JobID)
	assert.False(t, got.Contract.Deadline.IsZero())
	assert.Equal(t, "running", got.Status)
	assert.Greater(t, got.PID, 0)
	assert.Equal(t, []string{"code", "chat", "-m", "loom-orchestrator", "--worktree", filepath.Join("..", "worktree-us-2.1"), "Implement US-2.1"}, got.Command)
	assert.Equal(t, []string{"chat", "-m", "loom-orchestrator", "--worktree", filepath.Join("..", "worktree-us-2.1"), "Implement US-2.1"}, readArgsFile(t, argsFile))
	assert.Contains(t, got.Contract.JobID, ":US-2.1:1:")

	spawnAction := waitForActionEvent(t, s.Store(), "background_agent_spawned")
	var spawnDetail mcp.BackgroundAgentSpawnResult
	require.NoError(t, json.Unmarshal([]byte(spawnAction.Detail), &spawnDetail))
	assert.Equal(t, got.Contract, spawnDetail.Contract)

	exitAction := waitForActionEvent(t, s.Store(), "background_agent_exited")
	var exitDetail struct {
		StoryID         string `json:"story_id"`
		Prompt          string `json:"prompt"`
		Worktree        string `json:"worktree"`
		Contract        struct {
			JobID          string    `json:"job_id"`
			StoryID        string    `json:"story_id"`
			Attempt        int       `json:"attempt"`
			Input          string    `json:"input"`
			ExpectedOutput string    `json:"expected_output"`
			Deadline       time.Time `json:"deadline"`
		} `json:"contract"`
		PID             int    `json:"pid"`
		ExitCode        int    `json:"exit_code"`
		Success         bool   `json:"success"`
		WorktreeRemoved bool   `json:"worktree_removed"`
	}
	require.NoError(t, json.Unmarshal([]byte(exitAction.Detail), &exitDetail))
	assert.Equal(t, "US-2.1", exitDetail.StoryID)
	assert.Equal(t, "Implement US-2.1", exitDetail.Prompt)
	assert.Equal(t, filepath.Join("..", "worktree-us-2.1"), exitDetail.Worktree)
	assert.Equal(t, got.Contract.JobID, exitDetail.Contract.JobID)
	assert.Equal(t, got.Contract.StoryID, exitDetail.Contract.StoryID)
	assert.Equal(t, got.Contract.Attempt, exitDetail.Contract.Attempt)
	assert.Equal(t, got.Contract.Input, exitDetail.Contract.Input)
	assert.Equal(t, got.Contract.ExpectedOutput, exitDetail.Contract.ExpectedOutput)
	assert.Equal(t, got.Contract.Deadline, exitDetail.Contract.Deadline)
	assert.Equal(t, got.PID, exitDetail.PID)
	assert.Equal(t, 1, exitDetail.ExitCode)
	assert.False(t, exitDetail.Success)
	assert.True(t, exitDetail.WorktreeRemoved)
	assert.NoDirExists(t, filepath.Join(filepath.Dir(repoRoot), "worktree-us-2.1"))
}

func TestLoomSpawnAgent_CodeCLINotFoundReturnsClearError(t *testing.T) {
	_, mcpSvr := newTestServer(t)
	t.Setenv("PATH", t.TempDir())

	result := callTool(t, mcpSvr, "loom_spawn_agent", map[string]interface{}{
		"story_id": "US-2.1",
		"prompt":   "Implement US-2.1",
		"worktree": "worktree-us-2.1",
	})
	require.True(t, result.IsError)
	assert.Equal(t, "code CLI not found on PATH", toolText(t, result))
}

func TestLoomSpawnAgent_SecondSpawnHasAttempt2(t *testing.T) {
	repoRoot := initGitRepo(t)
	chdir(t, repoRoot)

	s, mcpSvr := newTestServer(t)
	installFakeCodeCLI(t, 0, 0)

	// First spawn → attempt 1.
	first := callTool(t, mcpSvr, "loom_spawn_agent", map[string]interface{}{
		"story_id": "US-2.1",
		"prompt":   "Implement US-2.1",
		"worktree": "worktree-us-2.1",
	})
	require.False(t, first.IsError, toolText(t, first))
	var firstResult mcp.BackgroundAgentSpawnResult
	require.NoError(t, json.Unmarshal([]byte(toolText(t, first)), &firstResult))
	assert.Equal(t, 1, firstResult.Contract.Attempt)
	assert.Contains(t, firstResult.Contract.JobID, ":US-2.1:1:")

	// background_agent_spawned is written synchronously before handleSpawnAgent returns.
	_ = s

	// Second spawn for same story → attempt 2 derived from prior spawn action.
	second := callTool(t, mcpSvr, "loom_spawn_agent", map[string]interface{}{
		"story_id": "US-2.1",
		"prompt":   "Implement US-2.1 retry",
		"worktree": "worktree-us-2.1",
	})
	require.False(t, second.IsError, toolText(t, second))
	var secondResult mcp.BackgroundAgentSpawnResult
	require.NoError(t, json.Unmarshal([]byte(toolText(t, second)), &secondResult))
	assert.Equal(t, 2, secondResult.Contract.Attempt)
	assert.Contains(t, secondResult.Contract.JobID, ":US-2.1:2:")
	assert.NotEqual(t, firstResult.Contract.JobID, secondResult.Contract.JobID)
}

func installFakeCodeCLI(t *testing.T, exitCode int, sleep time.Duration) string {
	t.Helper()

	tempDir := t.TempDir()
	argsFile := filepath.Join(tempDir, "args.txt")
	codePath := filepath.Join(tempDir, "code")
	originalPath := os.Getenv("PATH")
	script := strings.Join([]string{
		"#!/bin/sh",
		"printf '%s\n' \"$@\" > \"$LOOM_FAKE_CODE_ARGS_FILE\"",
		"if [ \"${LOOM_FAKE_CODE_SLEEP_MS:-0}\" -gt 0 ]; then",
		"  sleep $(awk \"BEGIN { printf \\\"%.3f\\\", ${LOOM_FAKE_CODE_SLEEP_MS}/1000 }\")",
		"fi",
		"exit \"${LOOM_FAKE_CODE_EXIT_CODE:-0}\"",
	}, "\n")
	require.NoError(t, os.WriteFile(codePath, []byte(script), 0o755))
	require.NoError(t, os.Chmod(codePath, 0o755))
	t.Setenv("PATH", tempDir+string(os.PathListSeparator)+originalPath)
	t.Setenv("LOOM_FAKE_CODE_ARGS_FILE", argsFile)
	t.Setenv("LOOM_FAKE_CODE_EXIT_CODE", strconv.Itoa(exitCode))
	t.Setenv("LOOM_FAKE_CODE_SLEEP_MS", strconv.FormatInt(sleep.Milliseconds(), 10))
	return argsFile
}

func readArgsFile(t *testing.T, path string) []string {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil {
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			if len(lines) == 1 && lines[0] == "" {
				return nil
			}
			return lines
		}
		if !os.IsNotExist(err) {
			require.NoError(t, err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for args file %q", path)
	return nil
}

func waitForActionEvent(t *testing.T, st store.Store, event string) store.Action {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		actions, err := st.ReadActions(context.Background(), 10)
		require.NoError(t, err)
		for _, action := range actions {
			if action.Event == event {
				return action
			}
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for action event %q", event)
	return store.Action{}
}

func initGitRepo(t *testing.T) string {
	t.Helper()

	repoRoot := t.TempDir()
	runGit(t, repoRoot, "init", "-b", "main")
	runGit(t, repoRoot, "config", "user.name", "Loom Test")
	runGit(t, repoRoot, "config", "user.email", "loom@example.com")
	require.NoError(t, os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("root\n"), 0o644))
	runGit(t, repoRoot, "add", "README.md")
	runGit(t, repoRoot, "commit", "-m", "initial")
	return repoRoot
}

func chdir(t *testing.T, dir string) {
	t.Helper()

	wd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() {
		require.NoError(t, os.Chdir(wd))
	})
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	output, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "git %s failed: %s", strings.Join(args, " "), string(output))
}
