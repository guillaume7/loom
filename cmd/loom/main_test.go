package main

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/guillaume7/loom/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootCmd_Help_ListsAllSubcommands(t *testing.T) {
	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--help"})
	_ = cmd.Execute()
	out := buf.String()
	for _, sub := range []string{"mcp", "start", "status", "pause", "resume", "reset", "log"} {
		assert.True(t, strings.Contains(out, sub), "expected %q in help output", sub)
	}
}

func TestStatusCmd_NoCheckpoint_PrintsNoActiveSession(t *testing.T) {
	t.Setenv("LOOM_DB_PATH", t.TempDir()+"/state.db")
	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"status"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "No active session")
}

func TestResetCmd_Force_ClearsState(t *testing.T) {
	t.Setenv("LOOM_DB_PATH", t.TempDir()+"/state.db")
	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"reset", "--force"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "State cleared.")
}

func TestResetCmd_Abort_OnNo(t *testing.T) {
	t.Setenv("LOOM_DB_PATH", t.TempDir()+"/state.db")
	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetIn(strings.NewReader("N\n"))
	cmd.SetArgs([]string{"reset"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Aborted.")
}

func TestMCPCmd_Help(t *testing.T) {
	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"mcp", "--help"})
	_ = cmd.Execute()
	assert.Contains(t, buf.String(), "MCP")
}

func TestPauseCmd_WritesCheckpoint(t *testing.T) {
	t.Setenv("LOOM_DB_PATH", t.TempDir()+"/state.db")
	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"pause"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Paused.")
}

func TestResumeCmd_NothingToResume(t *testing.T) {
	t.Setenv("LOOM_DB_PATH", t.TempDir()+"/state.db")
	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"resume"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Nothing to resume")
}

func TestLogCmd_EmptyFile_ExitsOK(t *testing.T) {
	t.Setenv("LOOM_LOG_PATH", t.TempDir()+"/nonexistent.log")
	t.Setenv("LOOM_DB_PATH", t.TempDir()+"/state.db")
	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"log"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Empty(t, buf.String())
}

func TestLogCmd_WithContent_PrintsLines(t *testing.T) {
	dir := t.TempDir()
	logPath := dir + "/loom.log"
	t.Setenv("LOOM_LOG_PATH", logPath)
	t.Setenv("LOOM_DB_PATH", dir+"/state.db")

	// Write two log lines.
	require.NoError(t, os.WriteFile(logPath, []byte(`{"level":"INFO","msg":"first"}
{"level":"INFO","msg":"second"}
`), 0o600))

	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"log"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "first")
	assert.Contains(t, buf.String(), "second")
}

func TestLogCmd_Tail_LimitsOutput(t *testing.T) {
	dir := t.TempDir()
	logPath := dir + "/loom.log"
	t.Setenv("LOOM_LOG_PATH", logPath)
	t.Setenv("LOOM_DB_PATH", dir+"/state.db")

	require.NoError(t, os.WriteFile(logPath, []byte("line1\nline2\nline3\n"), 0o600))

	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"log", "-n", "1"})
	require.NoError(t, cmd.Execute())
	out := buf.String()
	assert.Contains(t, out, "line3")
	assert.NotContains(t, out, "line1")
}

func TestStatusCmd_WithActiveCheckpoint_PrintsStateAndPhase(t *testing.T) {
	dir := t.TempDir()
	dbPath := dir + "/state.db"
	t.Setenv("LOOM_DB_PATH", dbPath)

	// Write a checkpoint directly to the store.
	st, err := store.New(dbPath)
	require.NoError(t, err)
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{
		State: "SCANNING",
		Phase: 2,
	}))
	require.NoError(t, st.Close())

	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"status"})
	require.NoError(t, cmd.Execute())

	out := buf.String()
	assert.Contains(t, out, "SCANNING")
	assert.Contains(t, out, "2")
}

func TestResumeCmd_WithPausedCheckpoint_PrintsResuming(t *testing.T) {
	dir := t.TempDir()
	dbPath := dir + "/state.db"
	t.Setenv("LOOM_DB_PATH", dbPath)

	// Write a PAUSED checkpoint.
	st, err := store.New(dbPath)
	require.NoError(t, err)
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{
		State: "PAUSED",
		Phase: 1,
	}))
	require.NoError(t, st.Close())

	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"resume"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "Resuming")
}

