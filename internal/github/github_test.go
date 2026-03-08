package github_test

import (
	"testing"

	loomgithub "github.com/guillaume7/loom/internal/github"
	"github.com/stretchr/testify/assert"
)

func TestIssueNumber_IsInt(t *testing.T) {
	var n loomgithub.IssueNumber = 42
	assert.Equal(t, 42, int(n))
}
