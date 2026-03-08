package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/guillaume7/loom/internal/config"
	"github.com/guillaume7/loom/internal/fsm"
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

			ctx := context.Background()
			cp, err := st.ReadCheckpoint(ctx)
			if err != nil {
				return err
			}

			if err := st.WriteCheckpoint(ctx, store.Checkpoint{
				State: string(fsm.StatePaused),
				Phase: cp.Phase,
			}); err != nil {
				return err
			}

			slog.Info("paused by operator")
			fmt.Fprintln(cmd.OutOrStdout(), "Paused.")
			return nil
		},
	}
}
