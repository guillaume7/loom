package store

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // register sqlite driver
	sqlite3 "modernc.org/sqlite"
	sqlite3lib "modernc.org/sqlite/lib"
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
		story_id     TEXT    NOT NULL DEFAULT '',
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
		{"story_id", "ALTER TABLE checkpoint ADD COLUMN story_id     TEXT    NOT NULL DEFAULT ''"},
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

	_, err = db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_checkpoint_story_id ON checkpoint(story_id)`)
	if err != nil {
		return err
	}
	return nil
}

func (s *sqliteStore) ReadCheckpoint(ctx context.Context) (Checkpoint, error) {
	return s.ReadCheckpointByStoryID(ctx, "")
}

// ReadCheckpointByStoryID returns the most recent persisted Checkpoint for a
// specific story ID. The empty story ID is reserved for v1 sequential mode.
func (s *sqliteStore) ReadCheckpointByStoryID(ctx context.Context, storyID string) (Checkpoint, error) {
	var cp Checkpoint
	var updatedAt string
	err := s.db.QueryRowContext(ctx,
		`SELECT story_id, state, phase, pr_number, issue_number, retry_count, updated_at
		FROM checkpoint
		WHERE story_id = ?
		ORDER BY id DESC
		LIMIT 1`,
		storyID,
	).Scan(&cp.StoryID, &cp.State, &cp.Phase, &cp.PRNumber, &cp.IssueNumber, &cp.RetryCount, &updatedAt)
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
	return s.WriteCheckpointByStoryID(ctx, "", cp)
}

// WriteCheckpointByStoryID persists cp for a specific story ID. The empty
// story ID preserves v1 sequential-mode behavior.
func (s *sqliteStore) WriteCheckpointByStoryID(ctx context.Context, storyID string, cp Checkpoint) error {
	cp.StoryID = storyID
	if cp.UpdatedAt.IsZero() {
		cp.UpdatedAt = time.Now()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO checkpoint
			(story_id, state, phase, pr_number, issue_number, retry_count, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(story_id) DO UPDATE SET
			state = excluded.state,
			phase = excluded.phase,
			pr_number = excluded.pr_number,
			issue_number = excluded.issue_number,
			retry_count = excluded.retry_count,
			updated_at = excluded.updated_at`,
		cp.StoryID,
		cp.State, cp.Phase, cp.PRNumber, cp.IssueNumber, cp.RetryCount,
		cp.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	return err
}

func (s *sqliteStore) WriteAction(ctx context.Context, action Action) error {
	if action.CreatedAt.IsZero() {
		action.CreatedAt = time.Now()
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO action_log
			(session_id, operation_key, state_before, state_after, event, detail, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		action.SessionID,
		action.OperationKey,
		action.StateBefore,
		action.StateAfter,
		action.Event,
		action.Detail,
		action.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	if isDuplicateOperationKeyError(err) {
		return ErrDuplicateOperationKey
	}
	return err
}

// WriteCheckpointAndAction atomically persists a checkpoint update and appends
// an action log entry in a single transaction. If the action's OperationKey
// already exists the transaction is rolled back and ErrDuplicateOperationKey
// is returned; neither the checkpoint nor the action is written.
func (s *sqliteStore) WriteCheckpointAndAction(ctx context.Context, cp Checkpoint, action Action) error {
	if cp.UpdatedAt.IsZero() {
		cp.UpdatedAt = time.Now()
	}
	if action.CreatedAt.IsZero() {
		action.CreatedAt = time.Now()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	var txErr error
	defer func() {
		if txErr != nil {
			_ = tx.Rollback()
		}
	}()

	cp.StoryID = ""
	_, txErr = tx.ExecContext(ctx,
		`INSERT INTO checkpoint
			(story_id, state, phase, pr_number, issue_number, retry_count, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(story_id) DO UPDATE SET
			state = excluded.state,
			phase = excluded.phase,
			pr_number = excluded.pr_number,
			issue_number = excluded.issue_number,
			retry_count = excluded.retry_count,
			updated_at = excluded.updated_at`,
		cp.StoryID,
		cp.State, cp.Phase, cp.PRNumber, cp.IssueNumber, cp.RetryCount,
		cp.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if txErr != nil {
		return txErr
	}

	_, txErr = tx.ExecContext(ctx,
		`INSERT INTO action_log
			(session_id, operation_key, state_before, state_after, event, detail, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		action.SessionID, action.OperationKey, action.StateBefore, action.StateAfter,
		action.Event, action.Detail, action.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	if isDuplicateOperationKeyError(txErr) {
		return ErrDuplicateOperationKey
	}
	if txErr != nil {
		return txErr
	}

	txErr = tx.Commit()
	return txErr
}

func (s *sqliteStore) ReadActionByOperationKey(ctx context.Context, operationKey string) (Action, error) {
	var action Action
	var createdAt string

	err := s.db.QueryRowContext(ctx,
		`SELECT id, session_id, operation_key, state_before, state_after, event, detail, created_at
		FROM action_log
		WHERE operation_key = ?`,
		operationKey,
	).Scan(
		&action.ID,
		&action.SessionID,
		&action.OperationKey,
		&action.StateBefore,
		&action.StateAfter,
		&action.Event,
		&action.Detail,
		&createdAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Action{}, ErrActionNotFound
	}
	if err != nil {
		return Action{}, err
	}

	action.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return Action{}, err
	}

	return action, nil
}

func (s *sqliteStore) ReadActions(ctx context.Context, limit int) ([]Action, error) {
	if limit <= 0 {
		return []Action{}, nil
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, session_id, operation_key, state_before, state_after, event, detail, created_at
		FROM action_log
		ORDER BY created_at DESC, id DESC
		LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	actions := make([]Action, 0, limit)
	for rows.Next() {
		var action Action
		var createdAt string
		if err := rows.Scan(
			&action.ID,
			&action.SessionID,
			&action.OperationKey,
			&action.StateBefore,
			&action.StateAfter,
			&action.Event,
			&action.Detail,
			&createdAt,
		); err != nil {
			return nil, err
		}
		action.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			return nil, err
		}
		actions = append(actions, action)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return actions, nil
}

func isDuplicateOperationKeyError(err error) bool {
	if err == nil {
		return false
	}

	var sqliteErr *sqlite3.Error
	return errors.As(err, &sqliteErr) && sqliteErr.Code() == sqlite3lib.SQLITE_CONSTRAINT_UNIQUE
}

func (s *sqliteStore) DeleteAll(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.ExecContext(ctx, "DELETE FROM action_log"); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM checkpoint"); err != nil {
		return err
	}

	err = tx.Commit()
	return err
}

func (s *sqliteStore) Close() error {
	return s.db.Close()
}
