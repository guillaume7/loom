package github_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPClient_CreateIssue_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.URL.Path, "/issues")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write(fixture(t, "issue_created.json"))
	})
	c := newTestClient(t, handler)

	issue, err := c.CreateIssue(context.Background(), "Test issue", "Test body", nil)
	require.NoError(t, err)
	require.NotNil(t, issue)
	assert.Equal(t, 42, issue.Number)
}

func TestHTTPClient_CreateIssue_Error(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Validation Failed"}`, http.StatusUnprocessableEntity)
	})
	c := newTestClient(t, handler)

	issue, err := c.CreateIssue(context.Background(), "", "", nil)
	require.Error(t, err)
	assert.Nil(t, issue)
	assert.Contains(t, err.Error(), "creating issue")
}

func TestHTTPClient_AddComment_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.URL.Path, "/comments")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write(fixture(t, "comment_created.json"))
	})
	c := newTestClient(t, handler)

	err := c.AddComment(context.Background(), 42, "Test comment")
	require.NoError(t, err)
}

func TestHTTPClient_AddComment_Error(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	c := newTestClient(t, handler)

	err := c.AddComment(context.Background(), 999, "comment")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "adding comment")
}

func TestHTTPClient_CloseIssue_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPatch, r.Method)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fixture(t, "issue_closed.json"))
	})
	c := newTestClient(t, handler)

	err := c.CloseIssue(context.Background(), 42)
	require.NoError(t, err)
}

func TestHTTPClient_CloseIssue_Error(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	c := newTestClient(t, handler)

	err := c.CloseIssue(context.Background(), 999)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "closing issue")
}
