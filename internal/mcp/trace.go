// Package mcp — session traceability (E9).
//
// This file provides append-only session trace management:
//   - openTrace: initialises a SessionTrace record when the server starts
//   - appendTrace: appends a TraceEvent entry (FSM transitions, interventions, system events)
//   - closeTrace: seals the session trace with end time and terminal outcome
//
// Traces are stored in the same SQLite database as checkpoints and the action
// log, but they are strictly non-authoritative: the FSM checkpoint is the
// single source of truth for workflow state. Traces exist solely for human
// auditability during and after a /run-loom session.
package mcp

import (
	"context"
	"log/slog"
	"time"

	"github.com/guillaume7/loom/internal/store"
)

// TraceKind classifies a trace event.
const (
	TraceKindTransition   = "transition"
	TraceKindIntervention = "intervention"
	TraceKindGitHub       = "github"
	TraceKindSystem       = "system"
)

// TraceOutcome is the terminal disposition of a session.
const (
	TraceOutcomeInProgress = "in_progress"
	TraceOutcomeComplete   = "complete"
	TraceOutcomePaused     = "paused"
	TraceOutcomeAborted    = "aborted"
	TraceOutcomeStalled    = "stalled"
)

// openTrace initialises a new SessionTrace in the store. The call is
// idempotent: re-opening a trace with the same session ID is a no-op.
// Errors are logged and silently swallowed because trace failures must never
// interrupt the primary workflow path.
func (s *Server) openTrace(ctx context.Context) {
	if s.traceSessionID == "" {
		return
	}
	trace := store.SessionTrace{
		SessionID:  s.traceSessionID,
		LoomVer:    s.loomVersion,
		Repository: s.repository,
		StartedAt:  time.Now(),
		Outcome:    TraceOutcomeInProgress,
	}
	if err := s.st.OpenSessionTrace(ctx, trace); err != nil {
		slog.WarnContext(ctx, "trace: failed to open session trace",
			"session_id", s.traceSessionID, "error", err)
	}
}

// appendTrace appends a TraceEvent to the active session trace. Like
// openTrace, errors are logged and swallowed to keep trace failures
// isolated from the primary workflow path.
func (s *Server) appendTrace(ctx context.Context, ev store.TraceEvent) {
	if s.traceSessionID == "" {
		return
	}
	ev.SessionID = s.traceSessionID
	if ev.CreatedAt.IsZero() {
		ev.CreatedAt = time.Now()
	}
	if err := s.st.AppendTraceEvent(ctx, ev); err != nil {
		slog.WarnContext(ctx, "trace: failed to append trace event",
			"session_id", s.traceSessionID, "kind", ev.Kind, "error", err)
	}
}

// closeTrace seals the session trace with a terminal outcome. Errors are
// logged and swallowed.
func (s *Server) closeTrace(ctx context.Context, outcome string) {
	if s.traceSessionID == "" {
		return
	}
	if err := s.st.CloseSessionTrace(ctx, s.traceSessionID, outcome); err != nil {
		slog.WarnContext(ctx, "trace: failed to close session trace",
			"session_id", s.traceSessionID, "outcome", outcome, "error", err)
	}
}
