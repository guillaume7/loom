package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/guillaume7/loom/internal/config"
	"github.com/guillaume7/loom/internal/fsm"
	"github.com/guillaume7/loom/internal/store"
	"github.com/spf13/cobra"
)

func newStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start or resume the Loom workflow from the last checkpoint",
		Long:  "Begin the Loom workflow from IDLE, or resume from the last persisted checkpoint.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			st, err := store.New(cfg.DBPath)
			if err != nil {
				return err
			}

			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			cp, err := st.ReadCheckpoint(ctx)
			if err != nil {
				return err
			}

			machine := fsm.NewMachine(fsm.DefaultConfig())
			if cp.State != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Resuming from %s\n", cp.State)
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "Starting from IDLE")
			}

			// Block until a signal is received, then write a PAUSED checkpoint.
			<-ctx.Done()

			_ = st.WriteCheckpoint(context.Background(), store.Checkpoint{
				State: string(fsm.StatePaused),
				Phase: cp.Phase,
			})
			// TODO: drive the FSM action loop using machine.
			_ = machine
			return nil
		},
	}
}
