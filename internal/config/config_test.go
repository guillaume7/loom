package config_test

import (
	"testing"

	"github.com/guillaume7/loom/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestConfig_Fields(t *testing.T) {
	cfg := config.Config{
		RepoOwner: "acme",
		RepoName:  "rocket",
	}
	assert.Equal(t, "acme", cfg.RepoOwner)
	assert.Equal(t, "rocket", cfg.RepoName)
}
