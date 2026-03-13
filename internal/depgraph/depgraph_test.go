package depgraph_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/guillaume7/loom/internal/depgraph"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_ParsesValidDependenciesYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dependencies.yaml")

	content := `version: 1
epics:
  - id: E1
    depends_on: []
    stories:
      - id: US-1.1
        depends_on: []
      - id: US-1.2
        depends_on: [US-1.1]
  - id: E2
    depends_on: [E1]
    stories:
      - id: US-2.1
        depends_on: []
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	graph, err := depgraph.Load(path)
	require.NoError(t, err)

	assert.Equal(t, 1, graph.Version)
	assert.Len(t, graph.Epics, 2)

	assert.Equal(t, "E1", graph.Epics[0].ID)
	assert.Empty(t, graph.Epics[0].DependsOn)
	assert.Len(t, graph.Epics[0].Stories, 2)
	assert.Equal(t, "US-1.2", graph.Epics[0].Stories[1].ID)
	assert.Equal(t, []string{"US-1.1"}, graph.Epics[0].Stories[1].DependsOn)

	assert.Equal(t, "E2", graph.Epics[1].ID)
	assert.Equal(t, []string{"E1"}, graph.Epics[1].DependsOn)
	assert.Len(t, graph.Epics[1].Stories, 1)
}

func TestLoad_RejectsUnsupportedVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dependencies.yaml")

	content := `version: 99
epics: []
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	_, err := depgraph.Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported version")
}

func TestLoad_MissingVersionRejected(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dependencies.yaml")

	content := `epics: []
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	_, err := depgraph.Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported version")
}

func TestLoad_MalformedYAMLIncludesParserDetails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dependencies.yaml")

	// Invalid flow sequence syntax should include parser line details.
	content := "version: 1\nepics: [\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	_, err := depgraph.Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse dependencies YAML")
	assert.Contains(t, err.Error(), "line")
}

func TestLoad_MissingFileReturnsWrappedNotExist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.yaml")

	_, err := depgraph.Load(path)
	require.Error(t, err)
	assert.True(t, errors.Is(err, os.ErrNotExist))
	assert.Contains(t, err.Error(), "read dependencies file")
}

func TestLoad_EmptyFileReturnsClearError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dependencies.yaml")
	require.NoError(t, os.WriteFile(path, []byte("  \n\t\n"), 0o600))

	_, err := depgraph.Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is empty")
}
