package main

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeTokenScopeClient struct {
	scopes []string
	err    error
	calls  int
}

func (f *fakeTokenScopeClient) TokenScopes(_ context.Context) ([]string, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.scopes, nil
}

func TestValidateTokenScopes_AllRequiredAndOptionalPresent(t *testing.T) {
	gh := &fakeTokenScopeClient{scopes: []string{"repo", "read:org"}}
	var out strings.Builder

	err := validateTokenScopes(context.Background(), &out, gh, false)
	require.NoError(t, err)
	assert.Equal(t, 1, gh.calls)
	assert.Empty(t, out.String())
}

func TestValidateTokenScopes_MissingRequiredRepoFails(t *testing.T) {
	gh := &fakeTokenScopeClient{scopes: []string{"read:org"}}
	var out strings.Builder

	err := validateTokenScopes(context.Background(), &out, gh, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "token missing required scope(s): repo")
	assert.Contains(t, err.Error(), "Required scopes: repo")
	assert.Equal(t, 1, gh.calls)
}

func TestValidateTokenScopes_MissingOptionalReadOrgWarns(t *testing.T) {
	gh := &fakeTokenScopeClient{scopes: []string{"repo"}}
	var out strings.Builder

	err := validateTokenScopes(context.Background(), &out, gh, false)
	require.NoError(t, err)
	assert.Equal(t, 1, gh.calls)
	assert.Contains(t, out.String(), "optional scope 'read:org' not present")
}

func TestValidateTokenScopes_SkipAuthBypassesValidation(t *testing.T) {
	gh := &fakeTokenScopeClient{scopes: []string{"repo"}}
	var out strings.Builder

	err := validateTokenScopes(context.Background(), &out, gh, true)
	require.NoError(t, err)
	assert.Equal(t, 0, gh.calls)
	assert.Empty(t, out.String())
}

func TestValidateTokenScopes_InvalidTokenFails(t *testing.T) {
	gh := &fakeTokenScopeClient{err: errors.New("GitHub token is invalid or expired")}
	var out strings.Builder

	err := validateTokenScopes(context.Background(), &out, gh, false)
	require.Error(t, err)
	assert.Equal(t, "GitHub token is invalid or expired", err.Error())
	assert.Equal(t, 1, gh.calls)
}

func TestWarnIfConfigPermissionsTooOpen_0600_NoWarning(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	configDir := filepath.Join(homeDir, ".loom")
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	configPath := filepath.Join(configDir, "config.toml")
	require.NoError(t, os.WriteFile(configPath, []byte("token = \"x\"\n"), 0o600))
	require.NoError(t, os.Chmod(configPath, 0o600))

	var out strings.Builder
	warnIfConfigPermissionsTooOpen(
		&out,
		"linux",
		func() (string, error) { return homeDir, nil },
		os.Stat,
	)

	assert.Empty(t, out.String())
}

func TestWarnIfConfigPermissionsTooOpen_0644_Warns(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	configDir := filepath.Join(homeDir, ".loom")
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	configPath := filepath.Join(configDir, "config.toml")
	require.NoError(t, os.WriteFile(configPath, []byte("token = \"x\"\n"), 0o644))
	require.NoError(t, os.Chmod(configPath, 0o644))

	var out strings.Builder
	warnIfConfigPermissionsTooOpen(
		&out,
		"linux",
		func() (string, error) { return homeDir, nil },
		os.Stat,
	)

	assert.Contains(t, out.String(), "Warning: config.toml has permissions 0644, recommended 0600")
}

func TestWarnIfConfigPermissionsTooOpen_MissingFile_NoWarning(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	var out strings.Builder
	warnIfConfigPermissionsTooOpen(
		&out,
		"linux",
		func() (string, error) { return homeDir, nil },
		os.Stat,
	)

	assert.Empty(t, out.String())
}

func TestWarnIfConfigPermissionsTooOpen_NonUnix_SkipsGracefully(t *testing.T) {
	t.Parallel()

	statCalled := false
	var out strings.Builder
	warnIfConfigPermissionsTooOpen(
		&out,
		"windows",
		func() (string, error) { return "", nil },
		func(_ string) (os.FileInfo, error) {
			statCalled = true
			return nil, fs.ErrNotExist
		},
	)

	assert.False(t, statCalled)
	assert.Empty(t, out.String())
}
