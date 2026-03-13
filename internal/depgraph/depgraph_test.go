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

func TestLoad_RejectsDirectEpicCycle(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dependencies.yaml")

	content := `version: 1
epics:
  - id: E1
    depends_on: [E2]
    stories:
      - id: US-1.1
        depends_on: []
  - id: E2
    depends_on: [E1]
    stories:
      - id: US-2.1
        depends_on: []
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	_, err := depgraph.Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "circular dependency")
	assert.Contains(t, err.Error(), "E1")
	assert.Contains(t, err.Error(), "E2")
	assert.Contains(t, err.Error(), "E1 -> E2 -> E1")
}

func TestLoad_RejectsTransitiveStoryCycle(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dependencies.yaml")

	content := `version: 1
epics:
  - id: E1
    depends_on: []
    stories:
      - id: US-1.1
        depends_on: [US-1.2]
      - id: US-1.2
        depends_on: [US-1.3]
      - id: US-1.3
        depends_on: [US-1.1]
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	_, err := depgraph.Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "circular dependency")
	assert.Contains(t, err.Error(), "US-1.1 -> US-1.2 -> US-1.3 -> US-1.1")
}

func TestLoad_RejectsUnknownStoryDependency(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dependencies.yaml")

	content := `version: 1
epics:
  - id: E1
    depends_on: []
    stories:
      - id: US-2.1
        depends_on: [US-99.1]
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	_, err := depgraph.Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown dependency")
	assert.Contains(t, err.Error(), "US-99.1")
}

func TestLoad_RejectsUnknownEpicDependency(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dependencies.yaml")

	content := `version: 1
epics:
  - id: E1
    depends_on: [E999]
    stories:
      - id: US-1.1
        depends_on: []
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	_, err := depgraph.Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown dependency")
	assert.Contains(t, err.Error(), "E999")
}

func TestLoad_RejectsDuplicateEpicIDs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dependencies.yaml")

	content := `version: 1
epics:
  - id: E1
    depends_on: []
    stories:
      - id: US-1.1
        depends_on: []
  - id: E1
    depends_on: []
    stories:
      - id: US-1.2
        depends_on: []
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	_, err := depgraph.Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")
	assert.Contains(t, err.Error(), "E1")
}

func TestLoad_RejectsDuplicateStoryIDs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dependencies.yaml")

	content := `version: 1
epics:
  - id: E1
    depends_on: []
    stories:
      - id: US-1.1
        depends_on: []
      - id: US-1.1
        depends_on: []
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	_, err := depgraph.Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")
	assert.Contains(t, err.Error(), "US-1.1")
}

func TestUnblocked_AllRootStoriesAreUnblockedInitially(t *testing.T) {
	graph := depgraph.Graph{
		Version: 1,
		Epics: []depgraph.Epic{
			{
				ID:        "E1",
				DependsOn: nil,
				Stories: []depgraph.Story{
					{ID: "US-1.1", DependsOn: nil},
					{ID: "US-1.2", DependsOn: []string{"US-1.1"}},
				},
			},
		},
	}

	unblocked := graph.Unblocked(nil)
	assert.Equal(t, []string{"US-1.1"}, unblocked)
}

func TestUnblocked_CompletingDependencyUnblocksDependents(t *testing.T) {
	graph := depgraph.Graph{
		Version: 1,
		Epics: []depgraph.Epic{
			{
				ID:        "E1",
				DependsOn: nil,
				Stories: []depgraph.Story{
					{ID: "US-1.1", DependsOn: nil},
					{ID: "US-2.1", DependsOn: []string{"US-1.1"}},
				},
			},
		},
	}

	unblocked := graph.Unblocked([]string{"US-1.1"})
	assert.Equal(t, []string{"US-2.1"}, unblocked)
}

func TestIsBlocked_EpicDependencyBlocksAllStoriesInEpic(t *testing.T) {
	graph := depgraph.Graph{
		Version: 1,
		Epics: []depgraph.Epic{
			{
				ID:        "E1",
				DependsOn: nil,
				Stories: []depgraph.Story{
					{ID: "US-1.1", DependsOn: nil},
					{ID: "US-1.2", DependsOn: nil},
				},
			},
			{
				ID:        "E2",
				DependsOn: []string{"E1"},
				Stories: []depgraph.Story{
					{ID: "US-2.1", DependsOn: nil},
				},
			},
		},
	}

	blocked, err := graph.IsBlocked("US-2.1", nil)
	require.NoError(t, err)
	assert.True(t, blocked)
}

func TestIsBlocked_EpicDependencySatisfiedUnblocksEpicStories(t *testing.T) {
	graph := depgraph.Graph{
		Version: 1,
		Epics: []depgraph.Epic{
			{
				ID:        "E1",
				DependsOn: nil,
				Stories: []depgraph.Story{
					{ID: "US-1.1", DependsOn: nil},
					{ID: "US-1.2", DependsOn: nil},
				},
			},
			{
				ID:        "E2",
				DependsOn: []string{"E1"},
				Stories: []depgraph.Story{
					{ID: "US-2.1", DependsOn: nil},
				},
			},
		},
	}

	blocked, err := graph.IsBlocked("US-2.1", []string{"US-1.1", "US-1.2"})
	require.NoError(t, err)
	assert.False(t, blocked)
}

func TestIsBlocked_MultipleDependenciesMustAllBeSatisfied(t *testing.T) {
	graph := depgraph.Graph{
		Version: 1,
		Epics: []depgraph.Epic{
			{
				ID:        "E1",
				DependsOn: nil,
				Stories: []depgraph.Story{
					{ID: "US-1.1", DependsOn: nil},
				},
			},
			{
				ID:        "E2",
				DependsOn: nil,
				Stories: []depgraph.Story{
					{ID: "US-2.1", DependsOn: nil},
				},
			},
			{
				ID:        "E3",
				DependsOn: nil,
				Stories: []depgraph.Story{
					{ID: "US-3.1", DependsOn: []string{"US-1.1", "US-2.1"}},
				},
			},
		},
	}

	blocked, err := graph.IsBlocked("US-3.1", []string{"US-1.1"})
	require.NoError(t, err)
	assert.True(t, blocked)
}

func TestIsBlocked_UnknownIDReturnsError(t *testing.T) {
	graph := depgraph.Graph{
		Version: 1,
		Epics: []depgraph.Epic{
			{
				ID:        "E1",
				DependsOn: nil,
				Stories: []depgraph.Story{
					{ID: "US-1.1", DependsOn: nil},
				},
			},
		},
	}

	_, err := graph.IsBlocked("US-99.99", []string{"US-1.1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "US-99.99")
}

func TestUnblocked_RespectsEpicDependenciesAndReturnsDeterministicOrder(t *testing.T) {
	graph := depgraph.Graph{
		Version: 1,
		Epics: []depgraph.Epic{
			{
				ID:        "E1",
				DependsOn: nil,
				Stories: []depgraph.Story{
					{ID: "US-1.1", DependsOn: nil},
					{ID: "US-1.2", DependsOn: nil},
				},
			},
			{
				ID:        "E2",
				DependsOn: []string{"E1"},
				Stories: []depgraph.Story{
					{ID: "US-2.2", DependsOn: nil},
					{ID: "US-2.1", DependsOn: nil},
				},
			},
		},
	}

	initial := graph.Unblocked(nil)
	assert.Equal(t, []string{"US-1.1", "US-1.2"}, initial)

	afterEpicDone := graph.Unblocked([]string{"US-1.1", "US-1.2"})
	assert.Equal(t, []string{"US-2.1", "US-2.2"}, afterEpicDone)
}
