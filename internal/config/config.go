// Package config loads and validates Loom runtime configuration.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// Config holds the runtime configuration for Loom.
type Config struct {
	// Owner is the GitHub organisation or user that owns the target repository.
	Owner string `toml:"owner"`
	// Repo is the name of the target GitHub repository.
	Repo string `toml:"repo"`
	// Token is the GitHub personal access token used for API calls.
	// Never logged.
	Token string `toml:"token"`
	// DBPath is the path to the SQLite checkpoint database.
	// Defaults to ".loom/state.db" if unset.
	DBPath string `toml:"db_path"`
	// LogPath is the path to the structured JSON log file.
	// Defaults to ~/.loom/loom.log if unset.
	LogPath string `toml:"log_path"`
	// MaxParallel is the maximum number of stories the orchestrator may spawn
	// concurrently. Defaults to 3 when unset or invalid.
	MaxParallel int `toml:"max_parallel"`

	// Deprecated: Use Owner instead.
	RepoOwner string `toml:"repo_owner"`
	// Deprecated: Use Repo instead.
	RepoName string `toml:"repo_name"`
}

// Load reads configuration from ~/.loom/config.toml and then applies
// field-level overrides from environment variables:
//
//	LOOM_OWNER     → cfg.Owner
//	LOOM_REPO      → cfg.Repo
//	LOOM_TOKEN     → cfg.Token
//	LOOM_DB_PATH   → cfg.DBPath
//	LOOM_LOG_PATH  → cfg.LogPath
//	LOOM_MAX_PARALLEL → cfg.MaxParallel
//
// A missing config file is not an error; Load returns a zero-value Config.
// DBPath defaults to ".loom/state.db" and LogPath defaults to
// ~/.loom/loom.log when they are still empty after loading.
func Load() (Config, error) {
	var cfg Config

	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = ""
	}
	if homeDir != "" {
		path := filepath.Join(homeDir, ".loom", "config.toml")
		data, err := os.ReadFile(path)
		if err == nil {
			if err2 := toml.Unmarshal(data, &cfg); err2 != nil {
				return cfg, err2
			}
		}
	}

	// Environment variable overrides.
	if v := os.Getenv("LOOM_OWNER"); v != "" {
		cfg.Owner = v
	}
	if v := os.Getenv("LOOM_REPO"); v != "" {
		cfg.Repo = v
	}
	if v := os.Getenv("LOOM_TOKEN"); v != "" {
		cfg.Token = v
	}
	if v := os.Getenv("LOOM_DB_PATH"); v != "" {
		cfg.DBPath = v
	}
	if v := os.Getenv("LOOM_LOG_PATH"); v != "" {
		cfg.LogPath = v
	}
	if v := os.Getenv("LOOM_MAX_PARALLEL"); v != "" {
		var parsed int
		if _, err := fmt.Sscanf(v, "%d", &parsed); err == nil {
			cfg.MaxParallel = parsed
		}
	}

	// Default DBPath.
	if cfg.DBPath == "" {
		cfg.DBPath = ".loom/state.db"
	}
	// Default LogPath to ~/.loom/loom.log.
	if cfg.LogPath == "" && homeDir != "" {
		cfg.LogPath = filepath.Join(homeDir, ".loom", "loom.log")
	}
	if cfg.MaxParallel <= 0 {
		cfg.MaxParallel = 3
	}

	// Populate legacy aliases if not already set by file.
	if cfg.RepoOwner == "" {
		cfg.RepoOwner = cfg.Owner
	}
	if cfg.RepoName == "" {
		cfg.RepoName = cfg.Repo
	}

	return cfg, nil
}
