package main

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/guillaume7/loom/internal/config"
	"github.com/spf13/cobra"
)

func newLogCmd() *cobra.Command {
	var follow bool
	var tail int

	cmd := &cobra.Command{
		Use:   "log",
		Short: "Stream structured JSON log output",
		Long:  "Print log entries from the Loom log file. Use --follow to stream new entries in real time.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			logPath := cfg.LogPath

			f, err := os.Open(logPath)
			if err != nil {
				if os.IsNotExist(err) {
					if !follow {
						// Nothing to print; exit 0.
						return nil
					}
					// Wait for the file to appear when following.
					waitTicker := time.NewTicker(500 * time.Millisecond)
					defer waitTicker.Stop()
					for {
						select {
						case <-cmd.Context().Done():
							return nil
						case <-waitTicker.C:
						}
						f, err = os.Open(logPath)
						if err == nil {
							break
						}
					}
				} else {
					return err
				}
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

			// Apply -n tail limit.
			if tail > 0 && len(lines) > tail {
				lines = lines[len(lines)-tail:]
			}
			for _, l := range lines {
				fmt.Fprintln(cmd.OutOrStdout(), l)
			}

			if !follow {
				return nil
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
		},
	}

	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output in real time")
	cmd.Flags().IntVarP(&tail, "tail", "n", 0, "Number of last lines to show (0 = all)")
	return cmd
}
