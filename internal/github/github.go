// Package github provides a GitHub REST API client interface for Loom.
package github

import (
	"context"
	"time"
)

// RateLimit captures the current GitHub API core rate-limit budget.
type RateLimit struct {
	Limit     int
	Remaining int
	Reset     time.Time
}

// IssueNumber is the numeric identifier of a GitHub issue or pull request.
type IssueNumber int

// Client defines the GitHub operations required by Loom.
// Implementations must respect context cancellation and rate-limit headers.
type Client interface {
	// Ping verifies connectivity to the GitHub API and returns an error if
	// the server is unreachable or credentials are invalid.
	Ping(ctx context.Context) error
}
