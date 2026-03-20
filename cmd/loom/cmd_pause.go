package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/guillaume7/loom/internal/config"
	loomruntime "github.com/guillaume7/loom/internal/runtime"
	"github.com/guillaume7/loom/internal/store"
	"github.com/spf13/cobra"
)

func newPauseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pause",
		Short: "Gracefully pause the running Loom workflow",
		Long:  "Write a PAUSED checkpoint to the store, causing any running 'loom start' to stop.",
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

			ctx := context.Background()
			controller := loomruntime.NewController(st, loomruntime.DefaultConfig())
			lifecycle, err := controller.ApplyManualOverride(ctx, loomruntime.ManualOverrideRequest{
				Action:      loomruntime.ManualOverridePause,
				Source:      "cli",
				RequestedBy: "loom pause",
				Reason:      "operator requested pause",
			})
			if err != nil {
				if errors.Is(err, loomruntime.ErrNothingToPause) {
					fmt.Fprintln(cmd.OutOrStdout(), "Nothing to pause")
					return nil
				}
				return err
			}

			slog.Info("paused by operator")
			fmt.Fprintln(cmd.OutOrStdout(), "Paused.")
			printControllerLifecycle(cmd.OutOrStdout(), lifecycle)
			return nil
		},
	}
}
