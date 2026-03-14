package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/guillaume7/loom/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResetCmd_Force_RemovesManagedWorktreesAndClearsState(t *testing.T) {
	repoRoot := initGitRepo(t)
	chdir(t, repoRoot)

	worktreePath := filepath.Join(filepath.Dir(repoRoot), "worktree-us-2.1")
	runGit(t, repoRoot, "worktree", "add", "--detach", worktreePath, "refs/heads/main")

	dbPath := filepath.Join(repoRoot, ".loom", "state.db")
	t.Setenv("LOOM_DB_PATH", dbPath)
	st, err := store.New(dbPath)
	require.NoError(t, err)
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: "SCANNING", Phase: 1}))
	require.NoError(t, st.Close())

	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"reset", "--force"})
	require.NoError(t, cmd.Execute())

	assert.Contains(t, buf.String(), "State cleared.")
	assert.NoDirExists(t, worktreePath)

	st, err = store.New(dbPath)
	require.NoError(t, err)
	defer func() { require.NoError(t, st.Close()) }()

	cp, err := st.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, store.Checkpoint{}, cp)
}

func TestResetCmd_ConfirmationNo_DoesNotRemoveManagedWorktrees(t *testing.T) {
	repoRoot := initGitRepo(t)
	chdir(t, repoRoot)

	worktreePath := filepath.Join(filepath.Dir(repoRoot), "worktree-us-2.1")
	runGit(t, repoRoot, "worktree", "add", "--detach", worktreePath, "refs/heads/main")

	t.Setenv("LOOM_DB_PATH", filepath.Join(repoRoot, ".loom", "state.db"))

	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetIn(strings.NewReader("N\n"))
	cmd.SetArgs([]string{"reset"})
	require.NoError(t, cmd.Execute())

	assert.Contains(t, buf.String(), "Aborted.")
	assert.DirExists(t, worktreePath)
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
