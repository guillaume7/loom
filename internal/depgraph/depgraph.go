package depgraph

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const supportedVersion = 1

// Graph is the root schema for .loom/dependencies.yaml.
type Graph struct {
	Version int    `yaml:"version"`
	Epics   []Epic `yaml:"epics"`
}

// Epic is a dependency container for stories.
type Epic struct {
	ID        string  `yaml:"id"`
	DependsOn []string `yaml:"depends_on"`
	Stories   []Story `yaml:"stories"`
}

// Story is a dependency node in an epic.
type Story struct {
	ID        string   `yaml:"id"`
	DependsOn []string `yaml:"depends_on"`
}

// Load parses a .loom/dependencies.yaml file into a typed Graph.
func Load(path string) (Graph, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Graph{}, fmt.Errorf("read dependencies file %q: %w", path, err)
	}

	if strings.TrimSpace(string(data)) == "" {
		return Graph{}, fmt.Errorf("dependencies file %q is empty", path)
	}

	var graph Graph
	if err := yaml.Unmarshal(data, &graph); err != nil {
		return Graph{}, fmt.Errorf("parse dependencies YAML %q: %w", path, err)
	}

	if graph.Version != supportedVersion {
		return Graph{}, fmt.Errorf("unsupported version %d in %q (expected %d)", graph.Version, path, supportedVersion)
	}

	return graph, nil
}
