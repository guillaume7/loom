package github_test

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPClient_RateLimit_LowWarning(t *testing.T) {
	// Remaining=5, Limit=60 → below 10%, should log a warning and proceed.
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Limit", "60")
		w.Header().Set("X-RateLimit-Remaining", "5")
		w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(60*time.Second).Unix()))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fixture(t, "issue_created.json"))
	})
	c := newTestClient(t, handler)

	// Should succeed despite being below the rate-limit threshold.
	issue, err := c.CreateIssue(context.Background(), "test", "body", nil)
	require.NoError(t, err)
	assert.Equal(t, 42, issue.Number)
}

func TestHTTPClient_RateLimit_ZeroRemaining_ResetInPast(t *testing.T) {
	// Remaining=0, reset in the past → client should retry immediately and succeed on next call.
	var callCount atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)
		if n == 1 {
			// First call: rate limit exhausted, reset already passed.
			w.Header().Set("X-RateLimit-Limit", "60")
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(-1*time.Second).Unix()))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
			return
		}
		// Second call: success.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fixture(t, "issue_created.json"))
	})
	c := newTestClient(t, handler)

	issue, err := c.CreateIssue(context.Background(), "test", "body", nil)
	require.NoError(t, err)
	assert.Equal(t, 42, issue.Number)
	assert.GreaterOrEqual(t, int(callCount.Load()), 2)
}

func TestHTTPClient_RateLimit_HTTP429_ExponentialBackoff(t *testing.T) {
	// First 3 requests return 429; 4th succeeds.
	var callCount atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)
		if n <= 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"message":"rate limited"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fixture(t, "issue_created.json"))
	})
	c := newTestClient(t, handler)

	issue, err := c.CreateIssue(context.Background(), "test", "body", nil)
	require.NoError(t, err)
	assert.Equal(t, 42, issue.Number)
	assert.Equal(t, int32(4), callCount.Load())
}

func TestHTTPClient_RateLimit_HTTP429_ExhaustsRetries(t *testing.T) {
	// All calls return 429; should error after retries exhausted.
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"message":"rate limited"}`))
	})
	c := newTestClient(t, handler)

	_, err := c.CreateIssue(context.Background(), "test", "body", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating issue")
}
