package agentspawn

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSpawnStartsCodeChatWithPromptAndWorktree(t *testing.T) {
	repoRoot := initGitRepo(t)
	chdir(t, repoRoot)

	spawner := New()
	argsFile := installFakeCodeCLI(t, 0, 250*time.Millisecond)
	contract := testJobContract("US-2.1", "Implement US-2.1")

	handle, err := spawner.Spawn(Request{
		StoryID:  "US-2.1",
		Prompt:   "Implement US-2.1",
		Worktree: "worktree-us-2.1",
		Contract: contract,
	})
	require.NoError(t, err)

	started := handle.Started()
	assert.Equal(t, "US-2.1", started.StoryID)
	assert.Equal(t, "Implement US-2.1", started.Prompt)
	assert.Equal(t, contract, started.Contract)
	assert.Equal(t, filepath.Join("..", "worktree-us-2.1"), started.Worktree)
	assert.Greater(t, started.PID, 0)
	assert.Equal(t, []string{"chat", "-m", "loom-orchestrator", "--worktree", filepath.Join("..", "worktree-us-2.1"), "Implement US-2.1"}, started.Args)

	worktreePath := filepath.Join(filepath.Dir(repoRoot), "worktree-us-2.1")
	assert.DirExists(t, worktreePath)
	assert.Equal(t, gitOutput(t, repoRoot, "rev-parse", "refs/heads/main"), gitOutput(t, worktreePath, "rev-parse", "HEAD"))

	exit := waitForExit(t, handle.Done())
	assert.NoError(t, exit.Err)
	assert.NoError(t, exit.CleanupErr)
	assert.Equal(t, 0, exit.ExitCode)
	assert.Equal(t, started.PID, exit.Started.PID)
	assert.NoDirExists(t, worktreePath)

	args := readArgsFile(t, argsFile)
	assert.Equal(t, []string{"chat", "-m", "loom-orchestrator", "--worktree", filepath.Join("..", "worktree-us-2.1"), "Implement US-2.1"}, args)
}

func TestSpawnReturnsClearErrorWhenCodeCLIIsMissing(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	spawner := New()

	handle, err := spawner.Spawn(Request{
		StoryID:  "US-2.1",
		Prompt:   "Implement US-2.1",
		Worktree: "worktree-us-2.1",
		Contract: testJobContract("US-2.1", "Implement US-2.1"),
	})
	require.ErrorIs(t, err, ErrCodeCLINotFound)
	assert.Nil(t, handle)
	assert.Equal(t, "code CLI not found on PATH", err.Error())
}

func TestSpawnCapturesNonZeroExitCode(t *testing.T) {
	repoRoot := initGitRepo(t)
	chdir(t, repoRoot)

	installFakeCodeCLI(t, 1, 0)
	spawner := New()

	handle, err := spawner.Spawn(Request{
		StoryID:  "US-2.1",
		Prompt:   "Implement US-2.1",
		Worktree: "worktree-us-2.1",
		Contract: testJobContract("US-2.1", "Implement US-2.1"),
	})
	require.NoError(t, err)

	exit := waitForExit(t, handle.Done())
	require.Error(t, exit.Err)
	assert.Equal(t, 1, exit.ExitCode)
	assert.NoError(t, exit.CleanupErr)
	assert.NoDirExists(t, filepath.Join(filepath.Dir(repoRoot), "worktree-us-2.1"))
}

func TestSpawnFiltersSecretsAndPassesStoryIDEnv(t *testing.T) {
	t.Setenv("LOOM_TOKEN", "super-secret")
	t.Setenv("GH_TOKEN", "gh-secret")
	t.Setenv("GITHUB_TOKEN", "github-secret")
	t.Setenv("PATH", os.Getenv("PATH"))
	repoRoot := t.TempDir()
	worktreePath := filepath.Join(filepath.Dir(repoRoot), "worktree-us-2.1")

	var captured *exec.Cmd
	spawner := &Spawner{
		lookPath: func(name string) (string, error) {
			if name != "code" {
				return "", errors.New("unexpected executable")
			}
			return "/usr/bin/code", nil
		},
		command: func(name string, args ...string) *exec.Cmd {
			captured = exec.Command("sh", "-c", "exit 0")
			return captured
		},
		getwd: func() (string, error) {
			return repoRoot, nil
		},
		worktrees: fakeWorktreeManager{
			repoRoot:     repoRoot,
			worktreePath: worktreePath,
		},
	}

	handle, err := spawner.Spawn(Request{
		StoryID:  "US-2.1",
		Prompt:   "Implement US-2.1",
		Worktree: "worktree-us-2.1",
		Contract: testJobContract("US-2.1", "Implement US-2.1"),
	})
	require.NoError(t, err)
	require.NotNil(t, handle)
	require.NotNil(t, captured)

	env := envMap(captured.Env)
	assert.Equal(t, "US-2.1", env["LOOM_STORY_ID"])
	assert.Equal(t, repoRoot, captured.Dir)
	_, hasLoomToken := env["LOOM_TOKEN"]
	_, hasGHToken := env["GH_TOKEN"]
	_, hasGitHubToken := env["GITHUB_TOKEN"]
	assert.False(t, hasLoomToken)
	assert.False(t, hasGHToken)
	assert.False(t, hasGitHubToken)

	_ = waitForExit(t, handle.Done())
}

func TestSpawnRejectsInvalidJobContract(t *testing.T) {
	tests := []struct {
		name    string
		request Request
		wantErr string
	}{
		{
			name: "missing contract job id",
			request: Request{
				StoryID:  "US-2.1",
				Prompt:   "Implement US-2.1",
				Worktree: "worktree-us-2.1",
				Contract: JobContract{
					StoryID:        "US-2.1",
					Attempt:        1,
					Input:          "Implement US-2.1",
					ExpectedOutput: "Implement the requested story and return completed code changes with tests",
					Deadline:       time.Now().UTC().Add(30 * time.Minute),
				},
			},
			wantErr: "contract.job_id is required",
		},
		{
			name: "attempt must be positive",
			request: Request{
				StoryID:  "US-2.1",
				Prompt:   "Implement US-2.1",
				Worktree: "worktree-us-2.1",
				Contract: JobContract{
					JobID:          "job-US-2.1-1",
					StoryID:        "US-2.1",
					Attempt:        0,
					Input:          "Implement US-2.1",
					ExpectedOutput: "Implement the requested story and return completed code changes with tests",
					Deadline:       time.Now().UTC().Add(30 * time.Minute),
				},
			},
			wantErr: "contract.attempt must be at least 1",
		},
		{
			name: "missing deadline",
			request: Request{
				StoryID:  "US-2.1",
				Prompt:   "Implement US-2.1",
				Worktree: "worktree-us-2.1",
				Contract: JobContract{
					JobID:          "job-US-2.1-1",
					StoryID:        "US-2.1",
					Attempt:        1,
					Input:          "Implement US-2.1",
					ExpectedOutput: "Implement the requested story and return completed code changes with tests",
				},
			},
			wantErr: "contract.deadline is required",
		},
		{
			name: "story ids must match",
			request: Request{
				StoryID:  "US-2.2",
				Prompt:   "Implement US-2.1",
				Worktree: "worktree-us-2.1",
				Contract: testJobContract("US-2.1", "Implement US-2.1"),
			},
			wantErr: "story_id must match contract.story_id",
		},
		{
			name: "prompt must match contract input",
			request: Request{
				StoryID:  "US-2.1",
				Prompt:   "Different prompt",
				Worktree: "worktree-us-2.1",
				Contract: testJobContract("US-2.1", "Implement US-2.1"),
			},
			wantErr: "prompt must match contract.input",
		},
	}

	spawner := New()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handle, err := spawner.Spawn(tt.request)
			require.EqualError(t, err, tt.wantErr)
			assert.Nil(t, handle)
		})
	}
}

func TestFilteredEnv_AllowsWindowsRuntimeKeysAndDropsSecrets(t *testing.T) {
	filtered := filteredEnv([]string{
		"Path=C:\\Program Files\\Microsoft VS Code\\bin",
		"SystemRoot=C:\\Windows",
		"APPDATA=C:\\Users\\me\\AppData\\Roaming",
		"USERPROFILE=C:\\Users\\me",
		"GITHUB_TOKEN=secret",
		"GH_TOKEN=secret",
		"LOOM_TOKEN=secret",
	})

	env := envMap(filtered)
	assert.Equal(t, "C:\\Program Files\\Microsoft VS Code\\bin", env["Path"])
	assert.Equal(t, "C:\\Windows", env["SystemRoot"])
	assert.Equal(t, "C:\\Users\\me\\AppData\\Roaming", env["APPDATA"])
	assert.Equal(t, "C:\\Users\\me", env["USERPROFILE"])
	_, hasGitHubToken := env["GITHUB_TOKEN"]
	_, hasGHToken := env["GH_TOKEN"]
	_, hasLoomToken := env["LOOM_TOKEN"]
	assert.False(t, hasGitHubToken)
	assert.False(t, hasGHToken)
	assert.False(t, hasLoomToken)
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

func envMap(entries []string) map[string]string {
	env := make(map[string]string, len(entries))
	for _, entry := range entries {
		key, value, found := strings.Cut(entry, "=")
		if !found {
			continue
		}
		env[key] = value
	}
	return env
}

func waitForExit(t *testing.T, done <-chan Exit) Exit {
	t.Helper()
	select {
	case result := <-done:
		return result
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for background agent exit")
		return Exit{}
	}
}

type fakeWorktreeManager struct {
	repoRoot     string
	worktreePath string
}

func (m fakeWorktreeManager) RepoRoot(_ context.Context, _ string) (string, error) {
	return m.repoRoot, nil
}

func (m fakeWorktreeManager) Ensure(_ context.Context, _, _ string) (string, error) {
	return m.worktreePath, nil
}

func (m fakeWorktreeManager) Remove(_ context.Context, _, _ string, _ bool) error {
	return nil
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

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	output, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "git %s failed: %s", strings.Join(args, " "), string(output))
	return strings.TrimSpace(string(output))
}

func testJobContract(storyID, input string) JobContract {
	return JobContract{
		JobID:          "job-" + storyID + "-1",
		StoryID:        storyID,
		Attempt:        1,
		Input:          input,
		ExpectedOutput: "Implement the requested story and return completed code changes with tests",
		Deadline:       time.Date(2026, time.March, 21, 12, 0, 0, 0, time.UTC),
	}
}
