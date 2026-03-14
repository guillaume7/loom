package gitworktree

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnsureCreatesDeterministicWorktreeFromMainHeadAndReusesExisting(t *testing.T) {
	repoRoot := initGitRepo(t)
	manager := New()

	worktreePath, err := manager.Ensure(context.Background(), repoRoot, "US-2.1")
	require.NoError(t, err)

	expectedPath := filepath.Join(filepath.Dir(repoRoot), "worktree-us-2.1")
	assert.Equal(t, expectedPath, worktreePath)
	assert.DirExists(t, worktreePath)
	assert.Equal(t, gitOutput(t, repoRoot, "rev-parse", "refs/heads/main"), gitOutput(t, worktreePath, "rev-parse", "HEAD"))

	reusedPath, err := manager.Ensure(context.Background(), repoRoot, "US-2.1")
	require.NoError(t, err)
	assert.Equal(t, expectedPath, reusedPath)

	worktrees := gitOutput(t, repoRoot, "worktree", "list", "--porcelain")
	assert.Equal(t, 1, strings.Count(worktrees, "worktree "+expectedPath))
}

func TestCleanupManagedRemovesManagedWorktreesAndPrunes(t *testing.T) {
	repoRoot := initGitRepo(t)
	fake := &fakeRunner{
		outputs: map[string]string{
			commandKey(repoRoot, "worktree", "list", "--porcelain"): strings.Join([]string{
				"worktree " + repoRoot,
				"HEAD abc123",
				"branch refs/heads/main",
				"",
				"worktree " + filepath.Join(filepath.Dir(repoRoot), "worktree-us-2.1"),
				"HEAD def456",
				"detached",
				"",
				"worktree " + filepath.Join(filepath.Dir(repoRoot), "manual-sandbox"),
				"HEAD ghi789",
				"detached",
			}, "\n"),
		},
	}
	manager := &Manager{runner: fake}

	err := manager.CleanupManaged(context.Background(), repoRoot)
	require.NoError(t, err)

	assert.Equal(t, []fakeCall{
		{dir: repoRoot, args: []string{"worktree", "list", "--porcelain"}},
		{dir: repoRoot, args: []string{"worktree", "list", "--porcelain"}},
		{dir: repoRoot, args: []string{"worktree", "remove", "--force", filepath.Join(filepath.Dir(repoRoot), "worktree-us-2.1")}},
		{dir: repoRoot, args: []string{"worktree", "prune"}},
	}, fake.calls)
}

type fakeRunner struct {
	outputs map[string]string
	calls   []fakeCall
}

type fakeCall struct {
	dir  string
	args []string
}

func (f *fakeRunner) Run(_ context.Context, dir string, args ...string) (string, error) {
	f.calls = append(f.calls, fakeCall{dir: dir, args: append([]string(nil), args...)})
	return f.outputs[commandKey(dir, args...)], nil
}

func commandKey(dir string, args ...string) string {
	return dir + "\x00" + strings.Join(args, "\x00")
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
