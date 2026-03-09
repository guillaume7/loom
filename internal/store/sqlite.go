package store

import (
	"context"
	"database/sql"
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
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS checkpoint (
		id           INTEGER PRIMARY KEY,
		state        TEXT    NOT NULL,
		phase        INTEGER NOT NULL DEFAULT 0,
		pr_number    INTEGER NOT NULL DEFAULT 0,
		issue_number INTEGER NOT NULL DEFAULT 0,
		retry_count  INTEGER NOT NULL DEFAULT 0,
		updated_at   TEXT    NOT NULL DEFAULT ''
	)`)
	if err != nil {
		db.Close()
		return nil, err
	}
	return &sqliteStore{db: db}, nil
}

func (s *sqliteStore) ReadCheckpoint(ctx context.Context) (Checkpoint, error) {
	var cp Checkpoint
	var updatedAt string
	err := s.db.QueryRowContext(ctx,
		"SELECT state, phase, pr_number, issue_number, retry_count, updated_at FROM checkpoint WHERE id = 1",
	).Scan(&cp.State, &cp.Phase, &cp.PRNumber, &cp.IssueNumber, &cp.RetryCount, &updatedAt)
	if err == sql.ErrNoRows {
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
