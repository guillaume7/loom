package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/guillaume7/loom/internal/config"
	"github.com/guillaume7/loom/internal/store"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Print the current Loom workflow state and phase",
		Long:  "Read the latest checkpoint from the store and print state, phase, and recent log lines.",
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

			if cp.State == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "No active session")
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "State: %s\nPhase: %d\n", cp.State, cp.Phase)
			return nil
		},
	}
}
