package store

import (
	"context"
	"database/sql"
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

	// Write and read back via the full Store logic to confirm new columns work.
	st := &sqliteStore{db: db}
	want := Checkpoint{
		State:       "AWAITING_CI",
		Phase:       2,
		PRNumber:    7,
		IssueNumber: 3,
		RetryCount:  1,
	}
	require.NoError(t, st.WriteCheckpoint(context.Background(), want))

	got, err := st.ReadCheckpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, want.State, got.State)
	assert.Equal(t, want.Phase, got.Phase)
	assert.Equal(t, want.PRNumber, got.PRNumber)
	assert.Equal(t, want.IssueNumber, got.IssueNumber)
	assert.Equal(t, want.RetryCount, got.RetryCount)
	assert.False(t, got.UpdatedAt.IsZero(), "UpdatedAt should be auto-set")
}

// TestMigrate_Idempotent confirms that running migrate() on an up-to-date
// schema is a no-op and returns no error.
func TestMigrate_Idempotent(t *testing.T) {
	db := openPinnedMemDB(t)
	require.NoError(t, migrate(db))
	require.NoError(t, migrate(db)) // second call must not fail
}
