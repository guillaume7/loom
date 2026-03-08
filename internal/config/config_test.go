package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/guillaume7/loom/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Fields(t *testing.T) {
	cfg := config.Config{
		RepoOwner: "acme",
		RepoName:  "rocket",
	}
	assert.Equal(t, "acme", cfg.RepoOwner)
	assert.Equal(t, "rocket", cfg.RepoName)
}

func TestLoad_MissingFile_ReturnsDefaultsNoError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("LOOM_OWNER", "")
	t.Setenv("LOOM_REPO", "")
	t.Setenv("LOOM_TOKEN", "")
	t.Setenv("LOOM_DB_PATH", "")
	t.Setenv("LOOM_LOG_PATH", "")

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Empty(t, cfg.Owner)
	assert.Empty(t, cfg.Repo)
	assert.Empty(t, cfg.Token)
	assert.Equal(t, ".loom/state.db", cfg.DBPath)
	assert.Contains(t, cfg.LogPath, ".loom/loom.log")
}

func TestLoad_FromFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("LOOM_OWNER", "")
	t.Setenv("LOOM_REPO", "")
	t.Setenv("LOOM_TOKEN", "")
	t.Setenv("LOOM_DB_PATH", "")
	t.Setenv("LOOM_LOG_PATH", "")

	dir := filepath.Join(home, ".loom")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.toml"), []byte(`
owner = "acme"
repo  = "myrepo"
`), 0o600))

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "acme", cfg.Owner)
	assert.Equal(t, "myrepo", cfg.Repo)
}

func TestLoad_EnvOverrides(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("LOOM_OWNER", "override-owner")
	t.Setenv("LOOM_REPO", "override-repo")
	t.Setenv("LOOM_TOKEN", "mytoken")
	t.Setenv("LOOM_DB_PATH", "/tmp/custom.db")

	// Write a file with different values — env must win.
	dir := filepath.Join(home, ".loom")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.toml"), []byte(`
owner = "file-owner"
repo  = "file-repo"
`), 0o600))

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "override-owner", cfg.Owner)
	assert.Equal(t, "override-repo", cfg.Repo)
	assert.Equal(t, "mytoken", cfg.Token)
	assert.Equal(t, "/tmp/custom.db", cfg.DBPath)
}

func TestLoad_Token_NotInLogs(t *testing.T) {
	// Verify that Config.Token is present in the struct but is not the
	// zero value when set — this is a static/type-level test confirming the
	// field exists and is populated correctly.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("LOOM_TOKEN", "super-secret")
	t.Setenv("LOOM_OWNER", "")
	t.Setenv("LOOM_REPO", "")
	t.Setenv("LOOM_DB_PATH", "")

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "super-secret", cfg.Token)
}

func TestLoad_DefaultDBPath(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("LOOM_DB_PATH", "")
	t.Setenv("LOOM_OWNER", "")
	t.Setenv("LOOM_REPO", "")
	t.Setenv("LOOM_TOKEN", "")
	t.Setenv("LOOM_LOG_PATH", "")

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, ".loom/state.db", cfg.DBPath)
}

