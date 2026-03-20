package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/guillaume7/loom/internal/config"
	loomruntime "github.com/guillaume7/loom/internal/runtime"
	"github.com/guillaume7/loom/internal/store"
	"github.com/spf13/cobra"
)

func newResumeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "resume",
		Short: "Resume the Loom workflow from a paused checkpoint",
		Long:  "Resume the workflow from the last PAUSED checkpoint, or print 'Nothing to resume' if none exists.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			st, err := store.New(cfg.DBPath)
			if err != nil {
				return err
			}
			defer func() {
				if cerr := st.Close(); cerr != nil {
					slog.Error("store close", "error", cerr)
				}
			}()

			cp, err := st.ReadCheckpoint(context.Background())
			if err != nil {
				return err
			}

			if cp.State != "PAUSED" {
				fmt.Fprintln(cmd.OutOrStdout(), "Nothing to resume")
				return nil
			}

			controller := loomruntime.NewController(st, loomruntime.DefaultConfig())
			lifecycle, err := controller.ApplyManualOverride(context.Background(), loomruntime.ManualOverrideRequest{
				Action:      loomruntime.ManualOverrideResume,
				Source:      "cli",
				RequestedBy: "loom resume",
				Reason:      "operator requested resume",
			})
			if err != nil {
				if err == loomruntime.ErrNothingToResume {
					fmt.Fprintln(cmd.OutOrStdout(), "Nothing to resume")
					return nil
				}
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Resuming from %s\n", lifecycle.WorkflowState)
			printControllerLifecycle(cmd.OutOrStdout(), lifecycle)
			return nil
		},
	}
}
