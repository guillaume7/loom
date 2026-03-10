package integration_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReleaseWorkflow_FileExists verifies that the release workflow and
// GoReleaser configuration exist and contain the expected configuration
// (US-8.6).
func TestReleaseWorkflow_FileExists(t *testing.T) {
	const workflowPath = "../.github/workflows/release.yml"
	const goreleaserPath = "../.goreleaser.yml"

	workflowData, err := os.ReadFile(workflowPath)
	require.NoError(t, err, "release.yml must exist at %s", workflowPath)
	workflow := string(workflowData)

	goreleaserData, err := os.ReadFile(goreleaserPath)
	require.NoError(t, err, ".goreleaser.yml must exist at %s", goreleaserPath)
	goreleaser := string(goreleaserData)

	// Trigger: tag push only
	assert.Contains(t, workflow, "tags:", "release.yml must trigger on tag pushes")
	assert.Contains(t, workflow, "'v*'", "release.yml must use the v* tag pattern")

	// Publisher action delegates to GoReleaser.
	assert.Contains(t, workflow, "goreleaser/goreleaser-action@v6",
		"release.yml must use goreleaser/goreleaser-action to publish the release")
	assert.Contains(t, workflow, "fetch-depth: 0",
		"release.yml must fetch tags and history for GoReleaser")
	assert.Contains(t, workflow, "args: release --clean",
		"release.yml must run GoReleaser in release mode")
	assert.Contains(t, workflow, "GITHUB_TOKEN",
		"release.yml must pass GITHUB_TOKEN to GoReleaser")

	// All 5 platform targets are defined in GoReleaser.
	assert.Contains(t, goreleaser, "project_name: loom",
		".goreleaser.yml must define the project name")
	assert.Contains(t, goreleaser, "- linux",
		".goreleaser.yml must build for linux")
	assert.Contains(t, goreleaser, "- darwin",
		".goreleaser.yml must build for darwin")
	assert.Contains(t, goreleaser, "- windows",
		".goreleaser.yml must build for windows")
	assert.Contains(t, goreleaser, "- amd64",
		".goreleaser.yml must build for amd64")
	assert.Contains(t, goreleaser, "- arm64",
		".goreleaser.yml must build for arm64")
	assert.Contains(t, goreleaser, "goos: windows",
		".goreleaser.yml must explicitly filter unsupported windows/arm64 builds")
	assert.Contains(t, goreleaser, "goarch: arm64",
		".goreleaser.yml must explicitly filter unsupported windows/arm64 builds")

	// CGO disabled and raw binary uploads are configured in GoReleaser.
	assert.Contains(t, goreleaser, "CGO_ENABLED=0",
		".goreleaser.yml must set CGO_ENABLED=0 for static binaries")
	assert.Contains(t, goreleaser, "formats: [binary]",
		".goreleaser.yml must upload raw binaries rather than archives")
	assert.Contains(t, goreleaser, "loom-{{ .Os }}-{{ .Arch }}",
		".goreleaser.yml must keep the existing binary asset naming convention")

	// Version metadata and checksums are injected by GoReleaser.
	assert.Contains(t, goreleaser, "checksums.txt",
		".goreleaser.yml must generate and upload checksums.txt")
	assert.Contains(t, goreleaser, "changelog:",
		".goreleaser.yml must generate release notes from git history")
	assert.Contains(t, goreleaser, "use: git",
		".goreleaser.yml must use git changelog generation")
	assert.Contains(t, goreleaser, "sort: asc",
		".goreleaser.yml must sort generated changelog entries deterministically")
	assert.Contains(t, goreleaser, "^docs:",
		".goreleaser.yml must filter low-signal commits from release notes")
	assert.Contains(t, goreleaser, "## Loom {{ .Tag }}",
		".goreleaser.yml must include a release header for generated notes")
	assert.Contains(t, goreleaser, "-X main.version={{ .Version }}",
		".goreleaser.yml must inject the semantic version into the binary")
	assert.Contains(t, goreleaser, "-X main.commit={{ .Commit }}",
		".goreleaser.yml must inject the commit SHA into the binary")
	assert.Contains(t, goreleaser, "-X main.date={{ .Date }}",
		".goreleaser.yml must inject the build date into the binary")
}
