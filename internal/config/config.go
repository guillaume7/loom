// Package config loads and validates Loom runtime configuration.
package config

// Config holds the runtime configuration for Loom.
type Config struct {
	// RepoOwner is the GitHub organisation or user that owns the target repository.
	RepoOwner string
	// RepoName is the name of the target GitHub repository.
	RepoName string
}
