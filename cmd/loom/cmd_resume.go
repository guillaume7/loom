package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/guillaume7/loom/internal/config"
	"github.com/guillaume7/loom/internal/fsm"
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

			if cp.State != string(fsm.StatePaused) {
				fmt.Fprintln(cmd.OutOrStdout(), "Nothing to resume")
				return nil
			}

			resumeState, err := inferResumeState(context.Background(), st, cp)
			if err != nil {
				return err
			}

			cp.State = resumeState
			cp.ResumeState = ""
			if err := st.WriteCheckpoint(context.Background(), cp); err != nil {
				return err
			}

			lifecycle, err := loomruntime.NewController(st, loomruntime.DefaultConfig()).Start(context.Background())
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Resuming from %s\n", resumeState)
			printControllerLifecycle(cmd.OutOrStdout(), lifecycle)
			return nil
		},
	}
}

func inferResumeState(ctx context.Context, st store.Store, cp store.Checkpoint) (string, error) {
	if cp.ResumeState != "" && cp.ResumeState != string(fsm.StatePaused) {
		return cp.ResumeState, nil
	}

	actions, err := st.ReadActions(ctx, 200)
	if err != nil {
		return "", err
	}
	if len(actions) == 0 {
		return "", errors.New("paused checkpoint has no resume state and no action history to infer from")
	}

	for _, action := range actions {
		if action.StateAfter == string(fsm.StatePaused) {
			if action.StateBefore != "" && action.StateBefore != string(fsm.StatePaused) {
				return action.StateBefore, nil
			}
			continue
		}
		if action.StateAfter != "" {
			return action.StateAfter, nil
		}
	}

	return "", errors.New("paused checkpoint has no resumable state")
}

func resumableState(cp store.Checkpoint) string {
	if cp.State == string(fsm.StatePaused) {
		return cp.ResumeState
	}
	return cp.State
}
