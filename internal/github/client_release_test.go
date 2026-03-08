package github_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPClient_CreateTag_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.URL.Path, "/git/refs")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write(fixture(t, "tag_created.json"))
	})
	c := newTestClient(t, handler)

	tag, err := c.CreateTag(context.Background(), "v1.0.0", "deadbeef")
	require.NoError(t, err)
	require.NotNil(t, tag)
	assert.Equal(t, "refs/tags/v1.0.0", tag.Ref)
	assert.Equal(t, "deadbeef", tag.SHA)
}

func TestHTTPClient_CreateTag_Error(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Reference already exists"}`, http.StatusUnprocessableEntity)
	})
	c := newTestClient(t, handler)

	tag, err := c.CreateTag(context.Background(), "v1.0.0", "deadbeef")
	require.Error(t, err)
	assert.Nil(t, tag)
	assert.Contains(t, err.Error(), "creating tag")
}

func TestHTTPClient_CreateRelease_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.URL.Path, "/releases")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write(fixture(t, "release_created.json"))
	})
	c := newTestClient(t, handler)

	release, err := c.CreateRelease(context.Background(), "v1.0.0", "Release v1.0.0", "Initial release")
	require.NoError(t, err)
	require.NotNil(t, release)
	assert.Equal(t, int64(99), release.ID)
	assert.Equal(t, "v1.0.0", release.TagName)
}

func TestHTTPClient_CreateRelease_Error(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Validation Failed"}`, http.StatusUnprocessableEntity)
	})
	c := newTestClient(t, handler)

	release, err := c.CreateRelease(context.Background(), "", "", "")
	require.Error(t, err)
	assert.Nil(t, release)
	assert.Contains(t, err.Error(), "creating release")
}
