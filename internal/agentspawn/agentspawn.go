package agentspawn

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/guillaume7/loom/internal/gitworktree"
)

// ErrCodeCLINotFound indicates the VS Code CLI is unavailable on PATH.
var ErrCodeCLINotFound = errors.New("code CLI not found on PATH")

const storyIDEnvVar = "LOOM_STORY_ID"

// Request describes a background agent session to spawn.
type Request struct {
	StoryID  string
	Prompt   string
	Worktree string
}

// Started contains immutable metadata for a spawned agent process.
type Started struct {
	StoryID  string
	Prompt   string
	Worktree string
	Path     string
	Args     []string
	PID      int
}

// Command returns the executable name and argument vector.
func (s Started) Command() []string {
	cmd := make([]string, 0, len(s.Args)+1)
	cmd = append(cmd, "code")
	cmd = append(cmd, s.Args...)
	return cmd
}

// Exit contains the terminal state of a spawned background agent process.
type Exit struct {
	Started    Started
	ExitCode   int
	Err        error
	CleanupErr error
}

// SpawnHandle represents a running background agent process.
type SpawnHandle interface {
	Started() Started
	Done() <-chan Exit
}

// Handle represents a running background agent process.
type Handle struct {
	started Started
	done    <-chan Exit
}

// Started returns metadata about the spawned process.
func (h *Handle) Started() Started { return h.started }

// Done returns a channel that receives exactly one terminal process result.
func (h *Handle) Done() <-chan Exit { return h.done }

// Runner launches background agent sessions.
type Runner interface {
	Spawn(Request) (SpawnHandle, error)
}

// Spawner launches background agent sessions via the VS Code CLI.
type Spawner struct {
	lookPath  func(string) (string, error)
	command   func(string, ...string) *exec.Cmd
	getwd     func() (string, error)
	worktrees worktreeManager
}

type worktreeManager interface {
	RepoRoot(ctx context.Context, baseDir string) (string, error)
	Ensure(ctx context.Context, repoRoot, storyID string) (string, error)
	Remove(ctx context.Context, repoRoot, worktreePath string, force bool) error
}

// New returns a Spawner backed by os/exec.
func New() *Spawner {
	return &Spawner{
		lookPath:  exec.LookPath,
		command:   exec.Command,
		getwd:     os.Getwd,
		worktrees: gitworktree.New(),
	}
}

// Spawn starts a background agent process and returns immediately.
func (s *Spawner) Spawn(req Request) (SpawnHandle, error) {
	if req.StoryID == "" {
		return nil, fmt.Errorf("story_id is required")
	}
	if req.Prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}

	codePath, err := s.lookPath("code")
	if err != nil {
		return nil, ErrCodeCLINotFound
	}

	expectedWorktreeName := gitworktree.ManagedName(req.StoryID)
	if req.Worktree == "" {
		return nil, fmt.Errorf("worktree is required")
	}
	if filepath.Base(filepath.Clean(req.Worktree)) != expectedWorktreeName {
		return nil, fmt.Errorf("worktree must match deterministic name %q", expectedWorktreeName)
	}

	cwd, err := s.getwd()
	if err != nil {
		return nil, fmt.Errorf("resolve working directory: %w", err)
	}
	repoRoot, err := s.worktrees.RepoRoot(context.Background(), cwd)
	if err != nil {
		return nil, err
	}
	worktreePath, err := s.worktrees.Ensure(context.Background(), repoRoot, req.StoryID)
	if err != nil {
		return nil, err
	}
	worktreeArg, err := filepath.Rel(repoRoot, worktreePath)
	if err != nil {
		return nil, fmt.Errorf("resolve worktree path: %w", err)
	}

	args := []string{"chat", "-m", "loom-orchestrator", "--worktree", worktreeArg, req.Prompt}
	cmd := s.command(codePath, args...)
	cmd.Dir = repoRoot
	cmd.Env = append(filteredEnv(os.Environ()), storyIDEnvVar+"="+req.StoryID)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		cleanupErr := s.worktrees.Remove(context.Background(), repoRoot, worktreePath, true)
		if cleanupErr != nil {
			return nil, errors.Join(fmt.Errorf("start code chat: %w", err), cleanupErr)
		}
		return nil, fmt.Errorf("start code chat: %w", err)
	}

	started := Started{
		StoryID:  req.StoryID,
		Prompt:   req.Prompt,
		Worktree: worktreeArg,
		Path:     codePath,
		Args:     append([]string(nil), args...),
		PID:      cmd.Process.Pid,
	}
	results := make(chan Exit, 1)
	go func() {
		waitErr := cmd.Wait()
		exitCode := 0
		if waitErr != nil {
			var exitErr *exec.ExitError
			if errors.As(waitErr, &exitErr) {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = -1
			}
		}
		cleanupErr := s.worktrees.Remove(context.Background(), repoRoot, worktreePath, false)
		results <- Exit{
			Started:    started,
			ExitCode:   exitCode,
			Err:        errors.Join(waitErr, cleanupErr),
			CleanupErr: cleanupErr,
		}
		close(results)
	}()

	return &Handle{started: started, done: results}, nil
}

func filteredEnv(env []string) []string {
	filtered := make([]string, 0, len(env))
	for _, entry := range env {
		if entry == "" {
			continue
		}
		key, _, found := strings.Cut(entry, "=")
		if !found || !isAllowedEnvKey(key) {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

func isAllowedEnvKey(key string) bool {
	upperKey := strings.ToUpper(key)
	switch upperKey {
	case "HOME", "PATH", "PWD", "SHELL", "USER", "LOGNAME",
		"LANG", "TERM", "COLORTERM", "TMPDIR", "TMP", "TEMP",
		"DISPLAY", "WAYLAND_DISPLAY", "DBUS_SESSION_BUS_ADDRESS",
		"SSH_AUTH_SOCK", "SYSTEMROOT", "APPDATA", "LOCALAPPDATA",
		"USERPROFILE", "COMSPEC", "PATHEXT", "HOMEDRIVE", "HOMEPATH",
		"PROGRAMDATA", "PROGRAMFILES", "PROGRAMFILES(X86)",
		"COMMONPROGRAMFILES", "COMMONPROGRAMFILES(X86)":
		return true
	}
	for _, prefix := range []string{"LC_", "XDG_", "VSCODE_", "ELECTRON_", "LOOM_FAKE_CODE_"} {
		if strings.HasPrefix(upperKey, prefix) {
			return true
		}
	}
	return false
}
