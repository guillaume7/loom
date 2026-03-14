package github_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	loomgithub "github.com/guillaume7/loom/internal/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestClient starts an httptest.Server with the given handler and returns
// a configured HTTPClient pointing at it. The server is closed automatically
// when the test ends.
func newTestClient(t *testing.T, handler http.Handler) *loomgithub.HTTPClient {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return loomgithub.NewHTTPClient(srv.URL, "test-token", "owner", "repo",
		loomgithub.WithRetryBase(1*time.Millisecond)) // speed up back-off in tests
}

// fixture reads a JSON fixture file from testdata/.
func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return data
}

// compile-time interface satisfaction check.
var _ loomgithub.GitHubClient = (*loomgithub.HTTPClient)(nil)

func TestHTTPClient_Ping_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	c := newTestClient(t, handler)
	err := c.Ping(context.Background())
	require.NoError(t, err)
}

func TestHTTPClient_Ping_Error(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})
	c := newTestClient(t, handler)
	err := c.Ping(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "ping")
}

func TestHTTPClient_TokenScopes_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/user", r.URL.Path)
		require.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("X-OAuth-Scopes", "repo, read:org")
		w.WriteHeader(http.StatusOK)
	})

	c := newTestClient(t, handler)
	scopes, err := c.TokenScopes(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"repo", "read:org"}, scopes)
}

func TestHTTPClient_TokenScopes_InvalidToken(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "bad credentials", status)
			})

			c := newTestClient(t, handler)
			_, err := c.TokenScopes(context.Background())
			require.Error(t, err)
			assert.Equal(t, "GitHub token is invalid or expired", err.Error())
		})
	}
}
