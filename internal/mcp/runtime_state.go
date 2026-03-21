package mcp

import (
	"context"
	"time"

	loomruntime "github.com/guillaume7/loom/internal/runtime"
	"github.com/guillaume7/loom/internal/store"
)

func (s *Server) controllerLifecycle(ctx context.Context) (loomruntime.Lifecycle, error) {
	return loomruntime.NewController(s.runtimeStore(), loomruntime.DefaultConfig()).Snapshot(ctx)
}

func (s *Server) pendingWakes(ctx context.Context) ([]store.WakeSchedule, error) {
	return loomruntime.NewController(s.runtimeStore(), loomruntime.DefaultConfig()).PendingWakes(ctx)
}

func formatLifecycleTime(ts time.Time) string {
	if ts.IsZero() {
		return ""
	}
	return ts.UTC().Format(time.RFC3339)
}

type WakeDiagnostic struct {
	WakeKind  string `json:"wake_kind"`
	DueAt     string `json:"due_at"`
	DedupeKey string `json:"dedupe_key"`
	ClaimedAt string `json:"claimed_at,omitempty"`
}

func buildWakeDiagnostics(wakes []store.WakeSchedule) []WakeDiagnostic {
	if len(wakes) == 0 {
		return []WakeDiagnostic{}
	}
	diagnostics := make([]WakeDiagnostic, 0, len(wakes))
	for _, wake := range wakes {
		diagnostics = append(diagnostics, WakeDiagnostic{
			WakeKind:  wake.WakeKind,
			DueAt:     formatLifecycleTime(wake.DueAt),
			DedupeKey: wake.DedupeKey,
			ClaimedAt: formatLifecycleTime(wake.ClaimedAt),
		})
	}
	return diagnostics
}
