package mcp

import (
	"context"
	"time"

	loomruntime "github.com/guillaume7/loom/internal/runtime"
)

func (s *Server) controllerLifecycle(ctx context.Context) (loomruntime.Lifecycle, error) {
	return loomruntime.NewController(s.st, loomruntime.DefaultConfig()).Snapshot(ctx)
}

func formatLifecycleTime(ts time.Time) string {
	if ts.IsZero() {
		return ""
	}
	return ts.UTC().Format(time.RFC3339)
}