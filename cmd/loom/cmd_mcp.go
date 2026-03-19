package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/google/uuid"
	"github.com/guillaume7/loom/internal/config"
	"github.com/guillaume7/loom/internal/fsm"
	loomgh "github.com/guillaume7/loom/internal/github"
	"github.com/guillaume7/loom/internal/mcp"
	"github.com/guillaume7/loom/internal/store"
	"github.com/spf13/cobra"
)

func newMCPCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Start the Loom MCP stdio server",
		Long:  "Start the MCP stdio server and read JSON-RPC messages from stdin until EOF or interrupt.",
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

			machine := fsm.NewMachine(fsm.DefaultConfig())

			var gh loomgh.Client
			if cfg.Token != "" {
				gh = loomgh.NewHTTPClient("https://api.github.com", cfg.Token, cfg.Owner, cfg.Repo)
			}

			repo := ""
			if cfg.Owner != "" && cfg.Repo != "" {
				repo = fmt.Sprintf("%s/%s", cfg.Owner, cfg.Repo)
			}

			srv := mcp.NewServer(machine, st, gh,
				mcp.WithSchedulerConfig(mcp.SchedulerConfig{
					MaxParallel: cfg.MaxParallel,
				}),
				mcp.WithTraceSessionID(uuid.New().String()),
				mcp.WithLoomVersion(version),
				mcp.WithRepository(repo),
			)

			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			slog.Info("mcp server started")
			return srv.Serve(ctx, os.Stdin, os.Stdout)
		},
	}
}
