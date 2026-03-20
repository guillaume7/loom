package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/guillaume7/loom/internal/config"
	"github.com/guillaume7/loom/internal/fsm"
	loomgh "github.com/guillaume7/loom/internal/github"
	loomruntime "github.com/guillaume7/loom/internal/runtime"
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
			warnIfConfigPermissionsTooOpen(cmd.OutOrStdout(), runtime.GOOS, os.UserHomeDir, os.Stat)

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

			controller := loomruntime.NewController(st, loomruntime.DefaultConfig())
			if cp.State != string(fsm.StatePaused) {
				lifecycle, err := controller.Start(ctx)
				if err != nil {
					return err
				}
				printControllerLifecycle(cmd.OutOrStdout(), lifecycle)
			}

			machine := fsm.NewMachine(fsm.DefaultConfig())
			if err := machine.Hydrate(fsm.State(cp.State)); err != nil {
				return err
			}
			if cp.State != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Resuming from %s\n", cp.State)
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "Starting from IDLE")
			}

			// Block until a signal is received, then write a PAUSED checkpoint.
			<-ctx.Done()

			if err := st.WriteCheckpoint(context.Background(), store.Checkpoint{
				State:       string(fsm.StatePaused),
				ResumeState: resumableState(cp),
				Phase:       cp.Phase,
				PRNumber:    cp.PRNumber,
				IssueNumber: cp.IssueNumber,
				RetryCount:  cp.RetryCount,
			}); err != nil {
				slog.Error("failed to write PAUSED checkpoint on shutdown", "error", err)
			}
			if _, err := controller.Shutdown(context.Background()); err != nil {
				slog.Error("failed to expire controller lease on shutdown", "error", err)
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

func warnIfConfigPermissionsTooOpen(
	out io.Writer,
	goos string,
	userHomeDir func() (string, error),
	statFn func(string) (os.FileInfo, error),
) {
	if !supportsUnixPermissions(goos) {
		return
	}

	homeDir, err := userHomeDir()
	if err != nil || strings.TrimSpace(homeDir) == "" {
		return
	}

	configPath := filepath.Join(homeDir, ".loom", "config.toml")
	info, err := statFn(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return
		}
		return
	}

	mode := info.Mode().Perm()
	if mode&0o077 != 0 {
		_, _ = fmt.Fprintf(out, "Warning: config.toml has permissions %04o, recommended 0600\n", mode)
	}
}

func supportsUnixPermissions(goos string) bool {
	switch goos {
	case "windows", "plan9":
		return false
	default:
		return true
	}
}
