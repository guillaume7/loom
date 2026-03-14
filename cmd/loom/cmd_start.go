package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/guillaume7/loom/internal/config"
	"github.com/guillaume7/loom/internal/fsm"
	loomgh "github.com/guillaume7/loom/internal/github"
	"github.com/guillaume7/loom/internal/store"
	"github.com/spf13/cobra"
)

type tokenScopeClient interface {
	TokenScopes(ctx context.Context) ([]string, error)
}

var requiredTokenScopes = []string{"repo"}

func newStartCmd() *cobra.Command {
	var skipAuth bool

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start or resume the Loom workflow from the last checkpoint",
		Long:  "Begin the Loom workflow from IDLE, or resume from the last persisted checkpoint.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			if !skipAuth {
				if strings.TrimSpace(cfg.Token) == "" {
					return fmt.Errorf("GitHub token is required unless --skip-auth is provided")
				}
				gh := loomgh.NewHTTPClient("https://api.github.com", cfg.Token, cfg.Owner, cfg.Repo)
				validateCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				if err := validateTokenScopes(validateCtx, cmd.OutOrStdout(), gh, skipAuth); err != nil {
					return err
				}
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

			if err := st.WriteCheckpoint(context.Background(), store.Checkpoint{
				State: string(fsm.StatePaused),
				Phase: cp.Phase,
			}); err != nil {
				slog.Error("failed to write PAUSED checkpoint on shutdown", "error", err)
			}
			// TODO: drive the FSM action loop using machine.
			_ = machine
			return nil
		},
	}
	cmd.Flags().BoolVar(&skipAuth, "skip-auth", false, "Skip GitHub token validation (testing only)")
	return cmd
}

func validateTokenScopes(ctx context.Context, out io.Writer, gh tokenScopeClient, skipAuth bool) error {
	if skipAuth {
		return nil
	}

	scopes, err := gh.TokenScopes(ctx)
	if err != nil {
		return err
	}

	scopeSet := make(map[string]struct{}, len(scopes))
	for _, s := range scopes {
		scopeSet[s] = struct{}{}
	}

	missingRequired := make([]string, 0, len(requiredTokenScopes))
	for _, s := range requiredTokenScopes {
		if _, ok := scopeSet[s]; !ok {
			missingRequired = append(missingRequired, s)
		}
	}
	if len(missingRequired) > 0 {
		return fmt.Errorf("token missing required scope(s): %s. Required scopes: %s", strings.Join(missingRequired, ", "), strings.Join(requiredTokenScopes, ", "))
	}

	if _, ok := scopeSet["read:org"]; !ok {
		_, _ = fmt.Fprintln(out, "Warning: optional scope 'read:org' not present; organization features may be limited")
	}

	return nil
}
