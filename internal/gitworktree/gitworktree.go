package gitworktree

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"
)

const managedPrefix = "worktree-"

type runner interface {
	Run(ctx context.Context, dir string, args ...string) (string, error)
}

type commandRunner struct{}

// Manager creates, removes, and resets Loom-managed Git worktrees.
type Manager struct {
	runner runner
}

// New returns a Manager backed by the local git CLI.
func New() *Manager {
	return &Manager{runner: commandRunner{}}
}

// ManagedName returns the deterministic Loom-managed worktree directory name
// for a story ID.
func ManagedName(storyID string) string {
	return managedPrefix + sanitizeStoryID(storyID)
}

// RepoRoot resolves the git repository root for baseDir.
func (m *Manager) RepoRoot(ctx context.Context, baseDir string) (string, error) {
	output, err := m.runner.Run(ctx, baseDir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return filepath.Clean(strings.TrimSpace(output)), nil
}

// Ensure creates the deterministic Loom-managed worktree for storyID from the
// main branch HEAD, or reuses the existing worktree when it already exists.
func (m *Manager) Ensure(ctx context.Context, repoRoot, storyID string) (string, error) {
	if strings.TrimSpace(storyID) == "" {
		return "", fmt.Errorf("story_id is required")
	}

	repoRoot = filepath.Clean(repoRoot)
	targetPath := managedPath(repoRoot, storyID)

	worktrees, err := m.list(ctx, repoRoot)
	if err != nil {
		return "", err
	}
	if containsWorktree(worktrees, targetPath) {
		return targetPath, nil
	}

	if _, err := m.runner.Run(ctx, repoRoot, "worktree", "add", "--detach", targetPath, "refs/heads/main"); err != nil {
		worktrees, listErr := m.list(ctx, repoRoot)
		if listErr == nil && containsWorktree(worktrees, targetPath) {
			return targetPath, nil
		}
		return "", err
	}

	if err := m.verifyMainHead(ctx, repoRoot, targetPath); err != nil {
		return "", err
	}
	return targetPath, nil
}

// Remove deletes worktreePath via git worktree remove. Missing worktrees are a
// no-op so cleanup is idempotent.
func (m *Manager) Remove(ctx context.Context, repoRoot, worktreePath string, force bool) error {
	repoRoot = filepath.Clean(repoRoot)
	targetPath := resolvePath(repoRoot, worktreePath)

	worktrees, err := m.list(ctx, repoRoot)
	if err != nil {
		return err
	}
	if !containsWorktree(worktrees, targetPath) {
		return nil
	}

	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, targetPath)
	_, err = m.runner.Run(ctx, repoRoot, args...)
	return err
}

// CleanupManaged removes all Loom-managed worktrees for repoRoot and then
// prunes stale worktree metadata.
func (m *Manager) CleanupManaged(ctx context.Context, repoRoot string) error {
	repoRoot = filepath.Clean(repoRoot)

	worktrees, err := m.list(ctx, repoRoot)
	if err != nil {
		return err
	}
	for _, path := range worktrees {
		if !isManagedPath(repoRoot, path) {
			continue
		}
		if err := m.Remove(ctx, repoRoot, path, true); err != nil {
			return err
		}
	}

	_, err = m.runner.Run(ctx, repoRoot, "worktree", "prune")
	return err
}

func (commandRunner) Run(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	output, err := cmd.CombinedOutput()
	trimmed := strings.TrimSpace(string(output))
	if err == nil {
		return trimmed, nil
	}
	if trimmed == "" {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, trimmed)
}

func (m *Manager) list(ctx context.Context, repoRoot string) ([]string, error) {
	output, err := m.runner.Run(ctx, repoRoot, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}

	blocks := strings.Split(strings.TrimSpace(output), "\n\n")
	worktrees := make([]string, 0, len(blocks))
	for _, block := range blocks {
		for _, line := range strings.Split(block, "\n") {
			if !strings.HasPrefix(line, "worktree ") {
				continue
			}
			worktrees = append(worktrees, filepath.Clean(strings.TrimSpace(strings.TrimPrefix(line, "worktree "))))
			break
		}
	}
	return worktrees, nil
}

func (m *Manager) verifyMainHead(ctx context.Context, repoRoot, worktreePath string) error {
	mainHead, err := m.runner.Run(ctx, repoRoot, "rev-parse", "refs/heads/main")
	if err != nil {
		return err
	}
	worktreeHead, err := m.runner.Run(ctx, worktreePath, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if strings.TrimSpace(mainHead) != strings.TrimSpace(worktreeHead) {
		return fmt.Errorf("worktree %q is not based on main HEAD", worktreePath)
	}
	return nil
}

func managedPath(repoRoot, storyID string) string {
	return filepath.Join(filepath.Dir(filepath.Clean(repoRoot)), ManagedName(storyID))
}

func containsWorktree(worktrees []string, target string) bool {
	target = filepath.Clean(target)
	for _, worktree := range worktrees {
		if filepath.Clean(worktree) == target {
			return true
		}
	}
	return false
}

func isManagedPath(repoRoot, worktreePath string) bool {
	repoRoot = filepath.Clean(repoRoot)
	worktreePath = filepath.Clean(worktreePath)
	if worktreePath == repoRoot {
		return false
	}
	return filepath.Dir(worktreePath) == filepath.Dir(repoRoot) &&
		strings.HasPrefix(filepath.Base(worktreePath), managedPrefix)
}

func resolvePath(repoRoot, worktreePath string) string {
	if filepath.IsAbs(worktreePath) {
		return filepath.Clean(worktreePath)
	}
	return filepath.Clean(filepath.Join(repoRoot, worktreePath))
}

func sanitizeStoryID(storyID string) string {
	storyID = strings.TrimSpace(strings.ToLower(storyID))
	if storyID == "" {
		return "story"
	}

	var builder strings.Builder
	builder.Grow(len(storyID))
	for _, r := range storyID {
		switch {
		case unicode.IsLetter(r), unicode.IsNumber(r), r == '.', r == '-', r == '_':
			builder.WriteRune(r)
		default:
			builder.WriteByte('-')
		}
	}

	sanitized := strings.Trim(builder.String(), "-")
	if sanitized == "" {
		return "story"
	}
	return sanitized
}
