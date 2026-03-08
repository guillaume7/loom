package github_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPClient_RequestReview_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.URL.Path, "/requested_reviewers")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write(fixture(t, "review_requested.json"))
	})
	c := newTestClient(t, handler)

	err := c.RequestReview(context.Background(), 7, "copilot")
	require.NoError(t, err)
}

func TestHTTPClient_RequestReview_Error(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	c := newTestClient(t, handler)

	err := c.RequestReview(context.Background(), 999, "copilot")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requesting review")
}

func TestHTTPClient_GetReviewStatus_Approved(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Contains(t, r.URL.Path, "/reviews")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fixture(t, "reviews_approved.json"))
	})
	c := newTestClient(t, handler)

	status, err := c.GetReviewStatus(context.Background(), 7)
	require.NoError(t, err)
	assert.Equal(t, "APPROVED", status)
}

func TestHTTPClient_GetReviewStatus_ChangesRequested(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fixture(t, "reviews_changes_requested.json"))
	})
	c := newTestClient(t, handler)

	status, err := c.GetReviewStatus(context.Background(), 7)
	require.NoError(t, err)
	assert.Equal(t, "CHANGES_REQUESTED", status)
}

func TestHTTPClient_GetReviewStatus_Pending(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fixture(t, "reviews_empty.json"))
	})
	c := newTestClient(t, handler)

	status, err := c.GetReviewStatus(context.Background(), 7)
	require.NoError(t, err)
	assert.Equal(t, "PENDING", status)
}

func TestHTTPClient_GetReviewStatus_Error(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	c := newTestClient(t, handler)

	status, err := c.GetReviewStatus(context.Background(), 999)
	require.Error(t, err)
	assert.Empty(t, status)
	assert.Contains(t, err.Error(), "getting review status")
}
