package github_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPClient_ListPRs_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Contains(t, r.URL.Path, "/pulls")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fixture(t, "prs_list.json"))
	})
	c := newTestClient(t, handler)

	prs, err := c.ListPRs(context.Background(), "feature/xyz")
	require.NoError(t, err)
	require.Len(t, prs, 1)
	assert.Equal(t, 7, prs[0].Number)
	assert.Equal(t, "abc123", prs[0].HeadSHA)
	assert.False(t, prs[0].Draft)
}

func TestHTTPClient_ListPRs_Error(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	c := newTestClient(t, handler)

	prs, err := c.ListPRs(context.Background(), "feature/xyz")
	require.Error(t, err)
	assert.Nil(t, prs)
	assert.Contains(t, err.Error(), "listing PRs")
}

func TestHTTPClient_GetPR_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Contains(t, r.URL.Path, "/pulls/7")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fixture(t, "pr_get.json"))
	})
	c := newTestClient(t, handler)

	pr, err := c.GetPR(context.Background(), 7)
	require.NoError(t, err)
	require.NotNil(t, pr)
	assert.Equal(t, 7, pr.Number)
	assert.False(t, pr.Draft)
	assert.Equal(t, "abc123", pr.HeadSHA)
}

func TestHTTPClient_GetPR_Error(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	c := newTestClient(t, handler)

	pr, err := c.GetPR(context.Background(), 999)
	require.Error(t, err)
	assert.Nil(t, pr)
	assert.Contains(t, err.Error(), "getting PR")
}

func TestHTTPClient_GetCheckRuns_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Contains(t, r.URL.Path, "/check-runs")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fixture(t, "check_runs.json"))
	})
	c := newTestClient(t, handler)

	runs, err := c.GetCheckRuns(context.Background(), "abc123")
	require.NoError(t, err)
	require.Len(t, runs, 2)
	for _, cr := range runs {
		assert.Equal(t, "success", cr.Conclusion)
	}
}

func TestHTTPClient_GetCheckRuns_Error(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	c := newTestClient(t, handler)

	runs, err := c.GetCheckRuns(context.Background(), "badsha")
	require.Error(t, err)
	assert.Nil(t, runs)
	assert.Contains(t, err.Error(), "getting check runs")
}

func TestHTTPClient_MarkReadyForReview_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.URL.Path, "/ready_for_review")
		w.WriteHeader(http.StatusOK)
	})
	c := newTestClient(t, handler)

	err := c.MarkReadyForReview(context.Background(), 7)
	require.NoError(t, err)
}

func TestHTTPClient_MarkReadyForReview_Error(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	c := newTestClient(t, handler)

	err := c.MarkReadyForReview(context.Background(), 999)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "marking PR ready for review")
}

func TestHTTPClient_MergePR_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Contains(t, r.URL.Path, "/merge")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fixture(t, "pr_merged.json"))
	})
	c := newTestClient(t, handler)

	err := c.MergePR(context.Background(), 7, "Merge feature PR")
	require.NoError(t, err)
}

func TestHTTPClient_MergePR_Error(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Method Not Allowed"}`, http.StatusMethodNotAllowed)
	})
	c := newTestClient(t, handler)

	err := c.MergePR(context.Background(), 7, "Merge feature PR")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "merging PR")
}
