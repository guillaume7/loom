package mcp_test

import (
	"testing"

	"github.com/guillaume7/loom/internal/mcp"
	"github.com/stretchr/testify/assert"
)

func TestNewServer_ReturnsNonNil(t *testing.T) {
	s := mcp.NewServer()
	assert.NotNil(t, s)
}
