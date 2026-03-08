package main

import (
	"bytes"
	"strings"
	"testing"

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

