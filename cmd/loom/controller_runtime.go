package main

import (
	"context"
	"fmt"
	"time"

	loomruntime "github.com/guillaume7/loom/internal/runtime"
	"github.com/guillaume7/loom/internal/store"
)

func readControllerLifecycle(ctx context.Context, st store.Store) (loomruntime.Lifecycle, error) {
	controller := loomruntime.NewController(st, loomruntime.DefaultConfig())
	return controller.Snapshot(ctx)
}

func readPendingWakeSchedules(ctx context.Context, st store.Store) ([]store.WakeSchedule, error) {
	controller := loomruntime.NewController(st, loomruntime.DefaultConfig())
	return controller.PendingWakes(ctx)
}

func formatControllerTime(ts time.Time) string {
	if ts.IsZero() {
		return ""
	}
	return ts.UTC().Format(time.RFC3339)
}

func printControllerLifecycle(out writer, lifecycle loomruntime.Lifecycle) {
	if lifecycle.Controller == "" {
		return
	}
	fmt.Fprintf(out, "Controller: %s\n", lifecycle.Controller)
	if lifecycle.Reason != "" {
		fmt.Fprintf(out, "Controller Reason: %s\n", lifecycle.Reason)
	}
	if lifecycle.HolderID != "" {
		fmt.Fprintf(out, "Controller Holder: %s\n", lifecycle.HolderID)
	}
	if lifecycle.LeaseKey != "" {
		fmt.Fprintf(out, "Controller Lease: %s\n", lifecycle.LeaseKey)
	}
	if leaseExpiry := formatControllerTime(lifecycle.LeaseExpires); leaseExpiry != "" {
		fmt.Fprintf(out, "Lease Expires: %s\n", leaseExpiry)
	}
	if lifecycle.NextWakeKind != "" {
		fmt.Fprintf(out, "Next Wake: %s\n", lifecycle.NextWakeKind)
	}
	if nextWakeAt := formatControllerTime(lifecycle.NextWakeAt); nextWakeAt != "" {
		fmt.Fprintf(out, "Next Wake At: %s\n", nextWakeAt)
	}
}

func printPendingWakeSchedules(out writer, wakes []store.WakeSchedule) {
	if len(wakes) == 0 {
		return
	}
	fmt.Fprintln(out, "Pending Wakes:")
	for _, wake := range wakes {
		line := fmt.Sprintf("- %s at %s", wake.WakeKind, formatControllerTime(wake.DueAt))
		if wake.DedupeKey != "" {
			line += fmt.Sprintf(" (%s)", wake.DedupeKey)
		}
		fmt.Fprintln(out, line)
	}
}

type writer interface {
	Write([]byte) (int, error)
}
