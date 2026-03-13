package store

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // register sqlite driver
)

const memoryPath = ":memory:"

type sqliteStore struct {
	db *sql.DB
}

// New opens (or creates) the SQLite database at dbPath and returns a Store.
// The parent directory is created with permissions 0700 if it does not exist.
// Pass ":memory:" for an ephemeral in-memory database (useful in tests).
func New(dbPath string) (Store, error) {
	if dbPath != memoryPath {
		if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
			return nil, err
		}
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	if dbPath == memoryPath {
		// database/sql may open multiple connections; each ":memory:" connection
		// gets its own independent database. Pin to one connection so all
		// operations share the same in-memory database.
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return &sqliteStore{db: db}, nil
}

// migrate creates the checkpoint table when it does not exist and adds any
// columns that are present in the current schema but absent from an existing
// table (forward-compatible, zero-downtime migration).
func migrate(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS checkpoint (
		id           INTEGER PRIMARY KEY,
		state        TEXT    NOT NULL,
		phase        INTEGER NOT NULL DEFAULT 0,
		pr_number    INTEGER NOT NULL DEFAULT 0,
		issue_number INTEGER NOT NULL DEFAULT 0,
		retry_count  INTEGER NOT NULL DEFAULT 0,
		updated_at   TEXT    NOT NULL DEFAULT ''
	)`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS action_log (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id    TEXT    NOT NULL,
		operation_key TEXT    NOT NULL,
		state_before  TEXT    NOT NULL,
		state_after   TEXT    NOT NULL,
		event         TEXT    NOT NULL,
		detail        TEXT    NOT NULL DEFAULT '',
		created_at    TEXT    NOT NULL
	)`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_operation_key ON action_log(operation_key)`)
	if err != nil {
		return err
	}

	// Collect existing column names.
	rows, err := db.Query("PRAGMA table_info(checkpoint)")
	if err != nil {
		return err
	}
	existing := make(map[string]struct{})
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull int
		var dfltValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			rows.Close()
			return err
		}
		existing[name] = struct{}{}
	}
	if err := rows.Close(); err != nil {
		return err
	}

	// Add any columns that the current schema requires but the table lacks.
	additions := []struct {
		col string
		ddl string
	}{
		{"pr_number", "ALTER TABLE checkpoint ADD COLUMN pr_number    INTEGER NOT NULL DEFAULT 0"},
		{"issue_number", "ALTER TABLE checkpoint ADD COLUMN issue_number INTEGER NOT NULL DEFAULT 0"},
		{"retry_count", "ALTER TABLE checkpoint ADD COLUMN retry_count  INTEGER NOT NULL DEFAULT 0"},
		{"updated_at", "ALTER TABLE checkpoint ADD COLUMN updated_at   TEXT    NOT NULL DEFAULT ''"},
	}
	for _, a := range additions {
		if _, ok := existing[a.col]; ok {
			continue
		}
		if _, err := db.Exec(a.ddl); err != nil {
			return err
		}
	}
	return nil
}

func (s *sqliteStore) ReadCheckpoint(ctx context.Context) (Checkpoint, error) {
	var cp Checkpoint
	var updatedAt string
	err := s.db.QueryRowContext(ctx,
		"SELECT state, phase, pr_number, issue_number, retry_count, updated_at FROM checkpoint WHERE id = 1",
	).Scan(&cp.State, &cp.Phase, &cp.PRNumber, &cp.IssueNumber, &cp.RetryCount, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Checkpoint{}, nil
	}
	if err != nil {
		return Checkpoint{}, err
	}
	if updatedAt != "" {
		cp.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
		if err != nil {
			return Checkpoint{}, err
		}
	}
	return cp, nil
}

func (s *sqliteStore) WriteCheckpoint(ctx context.Context, cp Checkpoint) error {
	if cp.UpdatedAt.IsZero() {
		cp.UpdatedAt = time.Now()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO checkpoint
			(id, state, phase, pr_number, issue_number, retry_count, updated_at)
		VALUES (1, ?, ?, ?, ?, ?, ?)`,
		cp.State, cp.Phase, cp.PRNumber, cp.IssueNumber, cp.RetryCount,
		cp.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	return err
}

func (s *sqliteStore) DeleteAll(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM checkpoint")
	return err
}

func (s *sqliteStore) Close() error {
	return s.db.Close()
}
