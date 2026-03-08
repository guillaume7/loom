package store

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // register sqlite driver
)

type sqliteStore struct {
	db *sql.DB
}

// New opens (or creates) the SQLite database at dbPath and returns a Store.
// The parent directory is created with permissions 0700 if it does not exist.
func New(dbPath string) (Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS checkpoint (
		id    INTEGER PRIMARY KEY,
		state TEXT    NOT NULL,
		phase INTEGER NOT NULL DEFAULT 0
	)`)
	if err != nil {
		db.Close()
		return nil, err
	}
	return &sqliteStore{db: db}, nil
}

func (s *sqliteStore) ReadCheckpoint(ctx context.Context) (Checkpoint, error) {
	var cp Checkpoint
	err := s.db.QueryRowContext(ctx,
		"SELECT state, phase FROM checkpoint WHERE id = 1").Scan(&cp.State, &cp.Phase)
	if err == sql.ErrNoRows {
		return Checkpoint{}, nil
	}
	return cp, err
}

func (s *sqliteStore) WriteCheckpoint(ctx context.Context, cp Checkpoint) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT OR REPLACE INTO checkpoint (id, state, phase) VALUES (1, ?, ?)",
		cp.State, cp.Phase)
	return err
}

func (s *sqliteStore) DeleteAll(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM checkpoint")
	return err
}
