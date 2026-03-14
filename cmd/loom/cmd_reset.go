package main

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/guillaume7/loom/internal/config"
	"github.com/guillaume7/loom/internal/gitworktree"
	"github.com/guillaume7/loom/internal/store"
	"github.com/spf13/cobra"
)

func newResetCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Clear all Loom workflow state (with confirmation)",
		Long:  "Delete all checkpoint rows from the store after prompting for confirmation.",
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

			if !force {
				fmt.Fprint(cmd.OutOrStdout(), "Are you sure? [y/N] ")
				scanner := bufio.NewScanner(cmd.InOrStdin())
				scanner.Scan()
				answer := strings.TrimSpace(scanner.Text())
				if !strings.EqualFold(answer, "y") {
					fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
					return nil
				}
			}

			resetCtx := context.Background()
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			worktrees := gitworktree.New()
			repoRoot, err := worktrees.RepoRoot(resetCtx, wd)
			if err != nil {
				return err
			}
			if err := worktrees.CleanupManaged(resetCtx, repoRoot); err != nil {
				return err
			}
			if err := st.DeleteAll(resetCtx); err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), "State cleared.")
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompt")
	return cmd
}
