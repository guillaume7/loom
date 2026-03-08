package main

import (
	"context"
	"fmt"

	"github.com/guillaume7/loom/internal/config"
	"github.com/guillaume7/loom/internal/fsm"
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

			cp, err := st.ReadCheckpoint(context.Background())
			if err != nil {
				return err
			}

			if cp.State == "" || cp.State != string(fsm.StatePaused) {
				fmt.Fprintln(cmd.OutOrStdout(), "Nothing to resume")
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Resuming from %s\n", cp.State)
			return nil
		},
	}
}
