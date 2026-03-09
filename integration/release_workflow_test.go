package integration_test

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReleaseWorkflow_FileExists verifies that the release workflow YAML file
// exists and contains the expected configuration (US-8.6).
func TestReleaseWorkflow_FileExists(t *testing.T) {
	const path = "../.github/workflows/release.yml"

	data, err := os.ReadFile(path)
	require.NoError(t, err, "release.yml must exist at %s", path)
	content := string(data)

	// Trigger: tag push only
	assert.Contains(t, content, "tags:", "release.yml must trigger on tag pushes")
	assert.Contains(t, content, "'v*'", "release.yml must use the v* tag pattern")

	// Publisher action
	assert.Contains(t, content, "softprops/action-gh-release",
		"release.yml must use softprops/action-gh-release to publish the release")

	// All 5 platform targets
	platforms := []string{
		"linux-amd64",
		"linux-arm64",
		"darwin-amd64",
		"darwin-arm64",
		"windows-amd64",
	}
	for _, p := range platforms {
		assert.True(t, strings.Contains(content, p),
			"release.yml must reference platform binary %q", p)
	}

	// CGO disabled for static binaries
	assert.Contains(t, content, "CGO_ENABLED=0",
		"release.yml must set CGO_ENABLED=0 for static binaries")

	// Checksums
	assert.Contains(t, content, "checksums.txt",
		"release.yml must generate and upload checksums.txt")
}
