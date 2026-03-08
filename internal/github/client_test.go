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
	"github.com/stretchr/testify/require"
)

// newTestClient starts an httptest.Server with the given handler and returns
// a configured HTTPClient pointing at it. The server is closed automatically
// when the test ends.
func newTestClient(t *testing.T, handler http.Handler) *loomgithub.HTTPClient {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := loomgithub.NewHTTPClient(srv.URL, "test-token", "owner", "repo")
	c.RetryBase = 1 * time.Millisecond // speed up back-off in tests
	return c
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
