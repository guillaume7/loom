package store

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// openPinnedMemDB opens an in-memory SQLite database with a single connection
// so that all operations share the same in-memory schema and data.
func openPinnedMemDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	return db
}

func tableExists(t *testing.T, db *sql.DB, table string) bool {
	t.Helper()

	var name string
	err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name)
	if err == sql.ErrNoRows {
		return false
	}
	require.NoError(t, err)
	return true
}

func tableColumns(t *testing.T, db *sql.DB, table string) []string {
	t.Helper()

	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	require.NoError(t, err)
	defer rows.Close()

	var columns []string
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull int
		var dfltValue sql.NullString
		var pk int
		require.NoError(t, rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk))
		columns = append(columns, name)
	}
	require.NoError(t, rows.Err())
	return columns
}

func hasUniqueOperationKeyIndex(t *testing.T, db *sql.DB) bool {
	t.Helper()

	rows, err := db.Query(`PRAGMA index_list(action_log)`)
	require.NoError(t, err)

	var indexNames []string

	for rows.Next() {
		var seq int
		var name string
		var unique int
		var origin, partial interface{}
		require.NoError(t, rows.Scan(&seq, &name, &unique, &origin, &partial))
		if unique != 1 {
			continue
		}
		indexNames = append(indexNames, name)
	}

	require.NoError(t, rows.Close())
	require.NoError(t, rows.Err())

	for _, name := range indexNames {
		infoRows, err := db.Query(fmt.Sprintf("PRAGMA index_info(%s)", name))
		require.NoError(t, err)

		for infoRows.Next() {
			var seqno, cid int
			var columnName string
			require.NoError(t, infoRows.Scan(&seqno, &cid, &columnName))
			if columnName == "operation_key" {
				require.NoError(t, infoRows.Close())
				return true
			}
		}
		require.NoError(t, infoRows.Close())
	}

	return false
}

func hasUniqueCheckpointStoryIDIndex(t *testing.T, db *sql.DB) bool {
	t.Helper()

	rows, err := db.Query(`PRAGMA index_list(checkpoint)`)
	require.NoError(t, err)

	var indexNames []string
	for rows.Next() {
		var seq int
		var name string
		var unique int
		var origin, partial interface{}
		require.NoError(t, rows.Scan(&seq, &name, &unique, &origin, &partial))
		if unique != 1 {
			continue
		}
		indexNames = append(indexNames, name)
	}

	require.NoError(t, rows.Close())
	require.NoError(t, rows.Err())

	for _, name := range indexNames {
		infoRows, err := db.Query(fmt.Sprintf("PRAGMA index_info(%s)", name))
		require.NoError(t, err)

		for infoRows.Next() {
			var seqno, cid int
			var columnName string
			require.NoError(t, infoRows.Scan(&seqno, &cid, &columnName))
			if columnName == "story_id" {
				require.NoError(t, infoRows.Close())
				return true
			}
		}
		require.NoError(t, infoRows.Close())
	}

	return false
}

func rowCount(t *testing.T, db *sql.DB, table string) int {
	t.Helper()

	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&count)
	require.NoError(t, err)
	return count
}

func TestMigrate_CreatesActionLogTableOnFreshDB(t *testing.T) {
	db := openPinnedMemDB(t)

	require.NoError(t, migrate(db))

	assert.True(t, tableExists(t, db, "checkpoint"))
	assert.True(t, tableExists(t, db, "action_log"))
	assert.True(t, tableExists(t, db, "wake_schedule"))
	assert.True(t, tableExists(t, db, "external_event"))
	assert.True(t, tableExists(t, db, "runtime_lease"))
	assert.True(t, tableExists(t, db, "policy_decision"))
	assert.Equal(t,
		[]string{"id", "session_id", "operation_key", "state_before", "state_after", "event", "detail", "created_at"},
		tableColumns(t, db, "action_log"),
	)
	assert.True(t, hasUniqueOperationKeyIndex(t, db), "action_log should have a unique index on operation_key")
	assert.Equal(t,
		[]string{"id", "session_id", "wake_kind", "due_at", "dedupe_key", "payload", "claimed_at", "created_at"},
		tableColumns(t, db, "wake_schedule"),
	)
	assert.Equal(t,
		[]string{"id", "session_id", "event_source", "event_kind", "external_id", "correlation_id", "payload", "observed_at"},
		tableColumns(t, db, "external_event"),
	)
	assert.Equal(t,
		[]string{"lease_key", "holder_id", "scope", "expires_at", "created_at", "renewed_at"},
		tableColumns(t, db, "runtime_lease"),
	)
	assert.Equal(t,
		[]string{"id", "session_id", "correlation_id", "decision_kind", "verdict", "input_hash", "detail", "created_at"},
		tableColumns(t, db, "policy_decision"),
	)
}

// TestMigrate_AddsColumnsToOldSchema verifies that migrate() adds the E7
// columns to a checkpoint table that was created with the original two-column
// (state, phase) schema, without disturbing existing rows.
func TestMigrate_AddsColumnsToOldSchema(t *testing.T) {
	db := openPinnedMemDB(t)

	// Create the old two-column schema and insert a row.
	_, err := db.Exec(`CREATE TABLE checkpoint (
		id    INTEGER PRIMARY KEY,
		state TEXT    NOT NULL,
		phase INTEGER NOT NULL DEFAULT 0
	)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO checkpoint (id, state, phase) VALUES (1, 'SCANNING', 1)`)
	require.NoError(t, err)

	// Run the migration — should add the missing E7 columns.
	require.NoError(t, migrate(db))
	assert.Contains(t, tableColumns(t, db, "checkpoint"), "story_id")
	assert.Contains(t, tableColumns(t, db, "checkpoint"), "resume_state")
	assert.True(t, hasUniqueCheckpointStoryIDIndex(t, db), "checkpoint should have a unique index on story_id")

	var storyID string
	err = db.QueryRow(`SELECT story_id FROM checkpoint WHERE id = 1`).Scan(&storyID)
	require.NoError(t, err)
	assert.Equal(t, "", storyID)

	// Write and read back via the full Store logic to confirm new columns work.
	st := &sqliteStore{db: db}
	want := Checkpoint{
		State:       "AWAITING_CI",
		ResumeState: "",
		Phase:       2,
		PRNumber:    7,
		IssueNumber: 3,
		RetryCount:  1,
	}
	require.NoError(t, st.WriteCheckpoint(context.Background(), want))

	got, err := st.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, want.State, got.State)
	assert.Equal(t, want.ResumeState, got.ResumeState)
	assert.Equal(t, want.Phase, got.Phase)
	assert.Equal(t, want.PRNumber, got.PRNumber)
	assert.Equal(t, want.IssueNumber, got.IssueNumber)
	assert.Equal(t, want.RetryCount, got.RetryCount)
	assert.False(t, got.UpdatedAt.IsZero(), "UpdatedAt should be auto-set")
}

func TestSQLiteCheckpointByStoryID_AllowsMultipleRowsAndLegacyRowIsUnchanged(t *testing.T) {
	db := openPinnedMemDB(t)
	require.NoError(t, migrate(db))

	st := &sqliteStore{db: db}
	ctx := context.Background()

	require.NoError(t, st.WriteCheckpoint(ctx, Checkpoint{State: "SCANNING", Phase: 1}))
	require.NoError(t, st.WriteCheckpointByStoryID(ctx, "US-2.1", Checkpoint{State: "ISSUE_CREATED", Phase: 2}))
	require.NoError(t, st.WriteCheckpointByStoryID(ctx, "US-2.2", Checkpoint{State: "AWAITING_CI", Phase: 3}))

	legacy, err := st.ReadCheckpoint(ctx)
	require.NoError(t, err)
	assert.Equal(t, "", legacy.StoryID)
	assert.Equal(t, "SCANNING", legacy.State)
	assert.Equal(t, 1, legacy.Phase)

	storyOne, err := st.ReadCheckpointByStoryID(ctx, "US-2.1")
	require.NoError(t, err)
	assert.Equal(t, "US-2.1", storyOne.StoryID)
	assert.Equal(t, "ISSUE_CREATED", storyOne.State)
	assert.Equal(t, 2, storyOne.Phase)

	storyTwo, err := st.ReadCheckpointByStoryID(ctx, "US-2.2")
	require.NoError(t, err)
	assert.Equal(t, "US-2.2", storyTwo.StoryID)
	assert.Equal(t, "AWAITING_CI", storyTwo.State)
	assert.Equal(t, 3, storyTwo.Phase)

	var rowCount int
	err = db.QueryRow(`SELECT COUNT(*) FROM checkpoint`).Scan(&rowCount)
	require.NoError(t, err)
	assert.Equal(t, 3, rowCount)
}

func TestMigrate_AddsActionLogToCheckpointOnlySchema(t *testing.T) {
	db := openPinnedMemDB(t)

	_, err := db.Exec(`CREATE TABLE checkpoint (
		id         INTEGER PRIMARY KEY,
		state      TEXT    NOT NULL,
		phase      INTEGER NOT NULL DEFAULT 0,
		updated_at TEXT    NOT NULL DEFAULT ''
	)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO checkpoint (id, state, phase, updated_at) VALUES (1, 'SCANNING', 1, '2026-03-13T12:00:00Z')`)
	require.NoError(t, err)

	require.NoError(t, migrate(db))

	assert.True(t, tableExists(t, db, "checkpoint"))
	assert.True(t, tableExists(t, db, "action_log"))

	var state string
	var phase int
	var updatedAt string
	err = db.QueryRow(`SELECT state, phase, updated_at FROM checkpoint WHERE id = 1`).Scan(&state, &phase, &updatedAt)
	require.NoError(t, err)
	assert.Equal(t, "SCANNING", state)
	assert.Equal(t, 1, phase)
	assert.Equal(t, "2026-03-13T12:00:00Z", updatedAt)
}

// TestMigrate_Idempotent confirms that running migrate() on an up-to-date
// schema is a no-op and returns no error.
func TestMigrate_Idempotent(t *testing.T) {
	db := openPinnedMemDB(t)
	require.NoError(t, migrate(db))
	beforeColumns := tableColumns(t, db, "action_log")
	beforeWakeColumns := tableColumns(t, db, "wake_schedule")
	beforeExternalEventColumns := tableColumns(t, db, "external_event")
	beforeRuntimeLeaseColumns := tableColumns(t, db, "runtime_lease")
	beforePolicyDecisionColumns := tableColumns(t, db, "policy_decision")
	beforeHasIndex := hasUniqueOperationKeyIndex(t, db)
	beforeHasCheckpointStoryIDIndex := hasUniqueCheckpointStoryIDIndex(t, db)
	require.NoError(t, migrate(db)) // second call must not fail
	assert.Equal(t, beforeColumns, tableColumns(t, db, "action_log"))
	assert.Equal(t, beforeWakeColumns, tableColumns(t, db, "wake_schedule"))
	assert.Equal(t, beforeExternalEventColumns, tableColumns(t, db, "external_event"))
	assert.Equal(t, beforeRuntimeLeaseColumns, tableColumns(t, db, "runtime_lease"))
	assert.Equal(t, beforePolicyDecisionColumns, tableColumns(t, db, "policy_decision"))
	assert.Equal(t, beforeHasIndex, hasUniqueOperationKeyIndex(t, db))
	assert.Equal(t, beforeHasCheckpointStoryIDIndex, hasUniqueCheckpointStoryIDIndex(t, db))
}

func TestMigrate_PreservesExistingCheckpointAndTraceRows(t *testing.T) {
	db := openPinnedMemDB(t)

	_, err := db.Exec(`CREATE TABLE checkpoint (
		id         INTEGER PRIMARY KEY,
		state      TEXT    NOT NULL,
		phase      INTEGER NOT NULL DEFAULT 0,
		updated_at TEXT    NOT NULL DEFAULT ''
	)`)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE action_log (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id    TEXT    NOT NULL,
		operation_key TEXT    NOT NULL,
		state_before  TEXT    NOT NULL,
		state_after   TEXT    NOT NULL,
		event         TEXT    NOT NULL,
		detail        TEXT    NOT NULL DEFAULT '',
		created_at    TEXT    NOT NULL
	)`)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE session_run (
		session_id   TEXT PRIMARY KEY,
		loom_version TEXT NOT NULL,
		repo_owner   TEXT NOT NULL,
		repo_name    TEXT NOT NULL,
		started_at   TEXT NOT NULL
	)`)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE session_trace_event (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id      TEXT NOT NULL,
		event_kind      TEXT NOT NULL,
		sequence_number INTEGER NOT NULL,
		payload         TEXT NOT NULL DEFAULT '',
		created_at      TEXT NOT NULL
	)`)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO checkpoint (id, state, phase, updated_at) VALUES (1, 'AWAITING_CI', 3, '2026-03-20T10:00:00Z')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO session_run (session_id, loom_version, repo_owner, repo_name, started_at) VALUES ('s1', 'v0.1.0', 'acme', 'loom', '2026-03-20T09:00:00Z')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO session_trace_event (session_id, event_kind, sequence_number, payload, created_at) VALUES ('s1', 'session_started', 1, '{}', '2026-03-20T09:00:01Z')`)
	require.NoError(t, err)

	require.NoError(t, migrate(db))

	assert.Equal(t, 1, rowCount(t, db, "checkpoint"))
	assert.Equal(t, 1, rowCount(t, db, "session_run"))
	assert.Equal(t, 1, rowCount(t, db, "session_trace_event"))

	var state string
	var phase int
	err = db.QueryRow(`SELECT state, phase FROM checkpoint WHERE id = 1`).Scan(&state, &phase)
	require.NoError(t, err)
	assert.Equal(t, "AWAITING_CI", state)
	assert.Equal(t, 3, phase)
	assert.True(t, tableExists(t, db, "wake_schedule"))
	assert.True(t, tableExists(t, db, "external_event"))
	assert.True(t, tableExists(t, db, "runtime_lease"))
	assert.True(t, tableExists(t, db, "policy_decision"))
}
