package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

func ensureRuntimeTables(db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS wake_schedule (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT    NOT NULL,
			wake_kind  TEXT    NOT NULL,
			due_at     TEXT    NOT NULL,
			dedupe_key TEXT    NOT NULL,
			payload    TEXT    NOT NULL DEFAULT '',
			claimed_at TEXT    NOT NULL DEFAULT '',
			created_at TEXT    NOT NULL
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_wake_schedule_dedupe_key ON wake_schedule(dedupe_key)`,
		`CREATE TABLE IF NOT EXISTS external_event (
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id     TEXT    NOT NULL,
			event_source   TEXT    NOT NULL,
			event_kind     TEXT    NOT NULL,
			external_id    TEXT    NOT NULL DEFAULT '',
			correlation_id TEXT    NOT NULL DEFAULT '',
			payload        TEXT    NOT NULL,
			observed_at    TEXT    NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS runtime_lease (
			lease_key  TEXT PRIMARY KEY,
			holder_id  TEXT NOT NULL,
			scope      TEXT NOT NULL,
			expires_at TEXT NOT NULL,
			created_at TEXT NOT NULL,
			renewed_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS policy_decision (
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id     TEXT    NOT NULL,
			correlation_id TEXT    NOT NULL DEFAULT '',
			decision_kind  TEXT    NOT NULL,
			verdict        TEXT    NOT NULL,
			input_hash     TEXT    NOT NULL,
			detail         TEXT    NOT NULL DEFAULT '',
			created_at     TEXT    NOT NULL
		)`,
	}

	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			return err
		}
	}

	if err := ensureTableColumn(db, "external_event", "correlation_id", "ALTER TABLE external_event ADD COLUMN correlation_id TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureTableColumn(db, "policy_decision", "correlation_id", "ALTER TABLE policy_decision ADD COLUMN correlation_id TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}

	return nil
}

func ensureTableColumn(db *sql.DB, table string, column string, ddl string) error {
	rows, err := db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return nil
		}
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var colType string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if name == column {
			return rows.Err()
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	_, err = db.Exec(ddl)
	return err
}

func (s *sqliteStore) UpsertWakeSchedule(ctx context.Context, wake WakeSchedule) error {
	if wake.CreatedAt.IsZero() {
		wake.CreatedAt = time.Now()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO wake_schedule
			(session_id, wake_kind, due_at, dedupe_key, payload, claimed_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(dedupe_key) DO UPDATE SET
			session_id = excluded.session_id,
			wake_kind = excluded.wake_kind,
			due_at = excluded.due_at,
			payload = excluded.payload,
			claimed_at = excluded.claimed_at,
			created_at = excluded.created_at`,
		wake.SessionID,
		wake.WakeKind,
		formatDBTime(wake.DueAt),
		wake.DedupeKey,
		wake.Payload,
		formatDBTime(wake.ClaimedAt),
		formatDBTime(wake.CreatedAt),
	)
	return err
}

func (s *sqliteStore) ReadWakeSchedules(ctx context.Context, sessionID string, limit int) ([]WakeSchedule, error) {
	if limit <= 0 {
		return []WakeSchedule{}, nil
	}

	query := `SELECT id, session_id, wake_kind, due_at, dedupe_key, payload, claimed_at, created_at
		FROM wake_schedule`
	args := []any{}
	if sessionID != "" {
		query += ` WHERE session_id = ?`
		args = append(args, sessionID)
	}
	query += ` ORDER BY due_at ASC, id ASC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	wakes := make([]WakeSchedule, 0, limit)
	for rows.Next() {
		wake, err := scanWakeSchedule(rows)
		if err != nil {
			return nil, err
		}
		wakes = append(wakes, wake)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return wakes, nil
}

func (s *sqliteStore) WriteExternalEvent(ctx context.Context, event ExternalEvent) error {
	if event.ObservedAt.IsZero() {
		event.ObservedAt = time.Now()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO external_event
			(session_id, event_source, event_kind, external_id, correlation_id, payload, observed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		event.SessionID,
		event.EventSource,
		event.EventKind,
		event.ExternalID,
		event.CorrelationID,
		event.Payload,
		formatDBTime(event.ObservedAt),
	)
	return err
}

func (s *sqliteStore) ReadExternalEvents(ctx context.Context, sessionID string, limit int) ([]ExternalEvent, error) {
	if limit <= 0 {
		return []ExternalEvent{}, nil
	}

	query := `SELECT id, session_id, event_source, event_kind, external_id, correlation_id, payload, observed_at
		FROM external_event`
	args := []any{}
	if sessionID != "" {
		query += ` WHERE session_id = ?`
		args = append(args, sessionID)
	}
	query += ` ORDER BY observed_at DESC, id DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]ExternalEvent, 0, limit)
	for rows.Next() {
		event, err := scanExternalEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return events, nil
}

func (s *sqliteStore) UpsertRuntimeLease(ctx context.Context, lease RuntimeLease) error {
	now := time.Now()
	if lease.CreatedAt.IsZero() {
		lease.CreatedAt = now
	}
	if lease.RenewedAt.IsZero() {
		lease.RenewedAt = lease.CreatedAt
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO runtime_lease
			(lease_key, holder_id, scope, expires_at, created_at, renewed_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(lease_key) DO UPDATE SET
			holder_id = excluded.holder_id,
			scope = excluded.scope,
			expires_at = excluded.expires_at,
			created_at = excluded.created_at,
			renewed_at = excluded.renewed_at`,
		lease.LeaseKey,
		lease.HolderID,
		lease.Scope,
		formatDBTime(lease.ExpiresAt),
		formatDBTime(lease.CreatedAt),
		formatDBTime(lease.RenewedAt),
	)
	return err
}

func (s *sqliteStore) ReadRuntimeLease(ctx context.Context, leaseKey string) (RuntimeLease, error) {
	var lease RuntimeLease
	var expiresAt string
	var createdAt string
	var renewedAt string

	err := s.db.QueryRowContext(ctx,
		`SELECT lease_key, holder_id, scope, expires_at, created_at, renewed_at
		FROM runtime_lease
		WHERE lease_key = ?`,
		leaseKey,
	).Scan(&lease.LeaseKey, &lease.HolderID, &lease.Scope, &expiresAt, &createdAt, &renewedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return RuntimeLease{}, ErrRuntimeLeaseNotFound
	}
	if err != nil {
		return RuntimeLease{}, err
	}

	lease.ExpiresAt, err = parseDBTime(expiresAt)
	if err != nil {
		return RuntimeLease{}, err
	}
	lease.CreatedAt, err = parseDBTime(createdAt)
	if err != nil {
		return RuntimeLease{}, err
	}
	lease.RenewedAt, err = parseDBTime(renewedAt)
	if err != nil {
		return RuntimeLease{}, err
	}

	return lease, nil
}

func (s *sqliteStore) WritePolicyDecision(ctx context.Context, decision PolicyDecision) error {
	if decision.CreatedAt.IsZero() {
		decision.CreatedAt = time.Now()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO policy_decision
			(session_id, correlation_id, decision_kind, verdict, input_hash, detail, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		decision.SessionID,
		decision.CorrelationID,
		decision.DecisionKind,
		decision.Verdict,
		decision.InputHash,
		decision.Detail,
		formatDBTime(decision.CreatedAt),
	)
	return err
}

func (s *sqliteStore) ReadPolicyDecisions(ctx context.Context, sessionID string, limit int) ([]PolicyDecision, error) {
	if limit <= 0 {
		return []PolicyDecision{}, nil
	}

	query := `SELECT id, session_id, correlation_id, decision_kind, verdict, input_hash, detail, created_at
		FROM policy_decision`
	args := []any{}
	if sessionID != "" {
		query += ` WHERE session_id = ?`
		args = append(args, sessionID)
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	decisions := make([]PolicyDecision, 0, limit)
	for rows.Next() {
		decision, err := scanPolicyDecision(rows)
		if err != nil {
			return nil, err
		}
		decisions = append(decisions, decision)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return decisions, nil
}

func scanWakeSchedule(scanner interface{ Scan(dest ...any) error }) (WakeSchedule, error) {
	var wake WakeSchedule
	var dueAt string
	var claimedAt string
	var createdAt string
	if err := scanner.Scan(&wake.ID, &wake.SessionID, &wake.WakeKind, &dueAt, &wake.DedupeKey, &wake.Payload, &claimedAt, &createdAt); err != nil {
		return WakeSchedule{}, err
	}
	var err error
	wake.DueAt, err = parseDBTime(dueAt)
	if err != nil {
		return WakeSchedule{}, err
	}
	wake.ClaimedAt, err = parseDBTime(claimedAt)
	if err != nil {
		return WakeSchedule{}, err
	}
	wake.CreatedAt, err = parseDBTime(createdAt)
	if err != nil {
		return WakeSchedule{}, err
	}
	return wake, nil
}

func scanExternalEvent(scanner interface{ Scan(dest ...any) error }) (ExternalEvent, error) {
	var event ExternalEvent
	var observedAt string
	if err := scanner.Scan(&event.ID, &event.SessionID, &event.EventSource, &event.EventKind, &event.ExternalID, &event.CorrelationID, &event.Payload, &observedAt); err != nil {
		return ExternalEvent{}, err
	}
	var err error
	event.ObservedAt, err = parseDBTime(observedAt)
	if err != nil {
		return ExternalEvent{}, err
	}
	return event, nil
}

func scanPolicyDecision(scanner interface{ Scan(dest ...any) error }) (PolicyDecision, error) {
	var decision PolicyDecision
	var createdAt string
	if err := scanner.Scan(&decision.ID, &decision.SessionID, &decision.CorrelationID, &decision.DecisionKind, &decision.Verdict, &decision.InputHash, &decision.Detail, &createdAt); err != nil {
		return PolicyDecision{}, err
	}
	var err error
	decision.CreatedAt, err = parseDBTime(createdAt)
	if err != nil {
		return PolicyDecision{}, err
	}
	return decision, nil
}

func formatDBTime(ts time.Time) string {
	if ts.IsZero() {
		return ""
	}
	return ts.UTC().Format(time.RFC3339Nano)
}

func parseDBTime(raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339Nano, raw)
}