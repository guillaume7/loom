package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/guillaume7/loom/internal/config"
	"github.com/guillaume7/loom/internal/store"
	"github.com/spf13/cobra"
)

func newLogCmd() *cobra.Command {
	var follow bool
	var limit int

	cmd := &cobra.Command{
		Use:   "log",
		Short: "Print recent action history",
		Long:  "Print recent action history from the action_log table. Use --follow to stream legacy file logs.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			if follow {
				return streamFileLog(cmd, cfg.LogPath, limit)
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

			actions, err := st.ReadActions(context.Background(), limit)
			if err != nil {
				return err
			}
			if len(actions) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No actions recorded")
				return nil
			}

			for _, action := range actions {
				fmt.Fprintf(
					cmd.OutOrStdout(),
					"%s %s %s -> %s %s\n",
					action.CreatedAt.UTC().Format(time.RFC3339),
					action.OperationKey,
					action.StateBefore,
					action.StateAfter,
					action.Event,
				)
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow legacy file log output in real time")
	cmd.Flags().IntVarP(&limit, "limit", "n", 50, "Number of entries to show")
	return cmd
}

func streamFileLog(cmd *cobra.Command, logPath string, limit int) error {
	f, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Nothing to print; exit 0.
			return nil
		}
		return err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil {
			slog.Error("log file close", "error", cerr)
		}
	}()

	// Collect all existing lines.
	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	if limit > 0 && len(lines) > limit {
		lines = lines[len(lines)-limit:]
	}
	for _, l := range lines {
		fmt.Fprintln(cmd.OutOrStdout(), l)
	}

	// Follow mode: track the current file offset and poll for new lines.
	offset, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}

	followTicker := time.NewTicker(500 * time.Millisecond)
	defer followTicker.Stop()
	for {
		select {
		case <-cmd.Context().Done():
			return nil
		case <-followTicker.C:
		}

		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			return err
		}
		pollScanner := bufio.NewScanner(f)
		for pollScanner.Scan() {
			line := pollScanner.Text()
			fmt.Fprintln(cmd.OutOrStdout(), line)
		}
		if err := pollScanner.Err(); err != nil {
			return err
		}
		offset, err = f.Seek(0, io.SeekCurrent)
		if err != nil {
			return err
		}
	}
}
