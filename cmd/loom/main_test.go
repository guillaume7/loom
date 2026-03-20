package main

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

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
	for _, sub := range []string{"version", "mcp", "start", "status", "pause", "resume", "reset", "log"} {
		assert.True(t, strings.Contains(out, sub), "expected %q in help output", sub)
	}
}

func TestVersionCmd_PrintsBuildMetadata(t *testing.T) {
	oldVersion, oldCommit, oldDate := version, commit, date
	version, commit, date = "v1.0.0", "abc1234", "2026-03-10T00:00:00Z"
	t.Cleanup(func() {
		version, commit, date = oldVersion, oldCommit, oldDate
	})

	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"version"})
	require.NoError(t, cmd.Execute())

	out := buf.String()
	assert.Contains(t, out, "loom v1.0.0")
	assert.Contains(t, out, "commit: abc1234")
	assert.Contains(t, out, "built: 2026-03-10T00:00:00Z")
}

func TestRootCmd_VersionFlag_PrintsBuildMetadata(t *testing.T) {
	oldVersion, oldCommit, oldDate := version, commit, date
	version, commit, date = "v1.0.0", "abc1234", "2026-03-10T00:00:00Z"
	t.Cleanup(func() {
		version, commit, date = oldVersion, oldCommit, oldDate
	})

	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--version"})
	require.NoError(t, cmd.Execute())

	out := buf.String()
	assert.Contains(t, out, "loom v1.0.0")
	assert.Contains(t, out, "commit: abc1234")
	assert.Contains(t, out, "built: 2026-03-10T00:00:00Z")
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
	chdir(t, initGitRepo(t))
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
	chdir(t, initGitRepo(t))
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
	dbPath := t.TempDir() + "/state.db"
	t.Setenv("LOOM_DB_PATH", dbPath)

	st, err := store.New(dbPath)
	require.NoError(t, err)
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{
		State:       "AWAITING_READY",
		Phase:       3,
		PRNumber:    25,
		IssueNumber: 24,
	}))
	require.NoError(t, st.Close())

	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"pause"})
	err = cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Paused.")

	st, err = store.New(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, st.Close())
	})
	cp, err := st.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "PAUSED", cp.State)
	assert.Equal(t, "AWAITING_READY", cp.ResumeState)

	actions, err := st.ReadActions(context.Background(), 10)
	require.NoError(t, err)
	require.Len(t, actions, 1)
	assert.Equal(t, "manual_override_pause", actions[0].Event)

	events, err := st.ReadExternalEvents(context.Background(), "default", 10)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "manual_override.pause", events[0].EventKind)

	decisions, err := st.ReadPolicyDecisions(context.Background(), "default", 10)
	require.NoError(t, err)
	require.Len(t, decisions, 1)
	assert.Equal(t, "pause", decisions[0].Verdict)
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
	t.Setenv("LOOM_DB_PATH", t.TempDir()+"/state.db")
	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"log"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Equal(t, "No actions recorded\n", buf.String())
}

func TestLogCmd_WithActions_PrintsRecentEntries(t *testing.T) {
	dir := t.TempDir()
	dbPath := dir + "/state.db"
	t.Setenv("LOOM_DB_PATH", dbPath)

	st, err := store.New(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, st.Close())
	})

	now := time.Date(2026, 3, 14, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		err := st.WriteAction(context.Background(), store.Action{
			SessionID:    "session-1",
			OperationKey: fmt.Sprintf("op-%d", i),
			StateBefore:  fmt.Sprintf("S%d", i),
			StateAfter:   fmt.Sprintf("S%d", i+1),
			Event:        fmt.Sprintf("event-%d", i),
			CreatedAt:    now.Add(time.Duration(i) * time.Second),
		})
		require.NoError(t, err)
	}

	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"log"})
	require.NoError(t, cmd.Execute())

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	require.Len(t, lines, 5)
	// Reverse-chronological by created_at.
	assert.Contains(t, lines[0], "op-4")
	assert.Contains(t, lines[0], "S4 -> S5")
	assert.Contains(t, lines[0], "event-4")
	assert.Regexp(t, regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T`), lines[0])
	assert.Contains(t, lines[4], "op-0")
}

func TestLogCmd_Limit_OverridesDefault(t *testing.T) {
	dir := t.TempDir()
	dbPath := dir + "/state.db"
	t.Setenv("LOOM_DB_PATH", dbPath)

	st, err := store.New(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, st.Close())
	})

	now := time.Date(2026, 3, 14, 11, 0, 0, 0, time.UTC)
	for i := 0; i < 100; i++ {
		err := st.WriteAction(context.Background(), store.Action{
			SessionID:    "session-2",
			OperationKey: fmt.Sprintf("k-%03d", i),
			StateBefore:  "BEFORE",
			StateAfter:   "AFTER",
			Event:        "transition",
			CreatedAt:    now.Add(time.Duration(i) * time.Second),
		})
		require.NoError(t, err)
	}

	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"log", "--limit", "10"})
	require.NoError(t, cmd.Execute())

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	require.Len(t, lines, 10)
	assert.Contains(t, lines[0], "k-099")
	assert.Contains(t, lines[9], "k-090")
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
	assert.Contains(t, out, "Controller:")
}

func TestStatusCmd_PrintsPendingWakes(t *testing.T) {
	dir := t.TempDir()
	dbPath := dir + "/state.db"
	t.Setenv("LOOM_DB_PATH", dbPath)

	st, err := store.New(dbPath)
	require.NoError(t, err)
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{
		State: "AWAITING_CI",
		Phase: 2,
	}))
	require.NoError(t, st.UpsertWakeSchedule(context.Background(), store.WakeSchedule{
		SessionID: "default",
		WakeKind:  "poll_ci",
		DueAt:     time.Date(2026, 3, 20, 12, 1, 0, 0, time.UTC),
		DedupeKey: "run:default:poll_ci",
	}))
	require.NoError(t, st.UpsertWakeSchedule(context.Background(), store.WakeSchedule{
		SessionID: "default",
		WakeKind:  "poll_review",
		DueAt:     time.Date(2026, 3, 20, 12, 2, 0, 0, time.UTC),
		DedupeKey: "run:default:poll_review",
	}))
	require.NoError(t, st.Close())

	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"status"})
	require.NoError(t, cmd.Execute())

	out := buf.String()
	assert.Contains(t, out, "Pending Wakes:")
	assert.Contains(t, out, "poll_ci at 2026-03-20T12:01:00Z (run:default:poll_ci)")
	assert.Contains(t, out, "poll_review at 2026-03-20T12:02:00Z (run:default:poll_review)")
}

func TestResumeCmd_WithPausedCheckpoint_PrintsResuming(t *testing.T) {
	dir := t.TempDir()
	dbPath := dir + "/state.db"
	t.Setenv("LOOM_DB_PATH", dbPath)

	// Write a PAUSED checkpoint.
	st, err := store.New(dbPath)
	require.NoError(t, err)
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{
		State:       "PAUSED",
		ResumeState: "AWAITING_READY",
		Phase:       1,
	}))
	require.NoError(t, st.Close())

	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"resume"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "Resuming from AWAITING_READY")
	assert.Contains(t, buf.String(), "Controller:")

	st, err = store.New(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, st.Close())
	})
	cp, err := st.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "AWAITING_READY", cp.State)
	assert.Equal(t, "", cp.ResumeState)

	decisions, err := st.ReadPolicyDecisions(context.Background(), "default", 10)
	require.NoError(t, err)
	require.Len(t, decisions, 1)
	assert.Equal(t, "resume", decisions[0].Verdict)
}

func TestResumeCmd_InfersResumeStateFromLastAction(t *testing.T) {
	dir := t.TempDir()
	dbPath := dir + "/state.db"
	t.Setenv("LOOM_DB_PATH", dbPath)

	st, err := store.New(dbPath)
	require.NoError(t, err)
	require.NoError(t, st.WriteAction(context.Background(), store.Action{
		SessionID:    "s1",
		OperationKey: "checkpoint:AWAITING_READY->PAUSED",
		StateBefore:  "AWAITING_READY",
		StateAfter:   "PAUSED",
		Event:        "abort",
	}))
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
	assert.Contains(t, buf.String(), "Resuming from AWAITING_READY")
	assert.Contains(t, buf.String(), "Controller:")

	st, err = store.New(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, st.Close())
	})
	cp, err := st.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "AWAITING_READY", cp.State)
}

func TestResumeCmd_SkipsPausedSelfLoopAndUsesEarlierAction(t *testing.T) {
	dir := t.TempDir()
	dbPath := dir + "/state.db"
	t.Setenv("LOOM_DB_PATH", dbPath)

	st, err := store.New(dbPath)
	require.NoError(t, err)
	require.NoError(t, st.WriteAction(context.Background(), store.Action{
		SessionID:    "s1",
		OperationKey: "checkpoint:AWAITING_READY->PAUSED",
		StateBefore:  "AWAITING_READY",
		StateAfter:   "PAUSED",
		Event:        "abort",
	}))
	require.NoError(t, st.WriteAction(context.Background(), store.Action{
		SessionID:    "s1",
		OperationKey: "recheck-next-step-paused",
		StateBefore:  "PAUSED",
		StateAfter:   "PAUSED",
		Event:        "next_step",
	}))
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
	assert.Contains(t, buf.String(), "Resuming from AWAITING_READY")
	assert.Contains(t, buf.String(), "Controller:")

	st, err = store.New(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, st.Close())
	})
	cp, err := st.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "AWAITING_READY", cp.State)
	assert.Equal(t, "", cp.ResumeState)
}

func TestResumeCmd_IgnoresPausedResumeStateAndFallsBackToHistory(t *testing.T) {
	dir := t.TempDir()
	dbPath := dir + "/state.db"
	t.Setenv("LOOM_DB_PATH", dbPath)

	st, err := store.New(dbPath)
	require.NoError(t, err)
	require.NoError(t, st.WriteAction(context.Background(), store.Action{
		SessionID:    "s1",
		OperationKey: "manual-recovery",
		StateBefore:  "PAUSED",
		StateAfter:   "AWAITING_READY",
		Event:        "manual_recovery",
	}))
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{
		State:       "PAUSED",
		ResumeState: "PAUSED",
		Phase:       1,
	}))
	require.NoError(t, st.Close())

	var buf bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"resume"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "Resuming from AWAITING_READY")
	assert.Contains(t, buf.String(), "Controller:")

	st, err = store.New(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, st.Close())
	})
	cp, err := st.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "AWAITING_READY", cp.State)
	assert.Equal(t, "", cp.ResumeState)
}
