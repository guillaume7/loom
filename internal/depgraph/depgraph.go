package depgraph

import (
	"fmt"
	"os"
	"sort"
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

	if err := graph.validate(); err != nil {
		return Graph{}, err
	}

	return graph, nil
}

func (g Graph) validate() error {
	epicIDs := make(map[string]struct{}, len(g.Epics))
	storyIDs := make(map[string]struct{})

	epicAdj := make(map[string][]string, len(g.Epics))
	storyAdj := make(map[string][]string)

	for _, epic := range g.Epics {
		if _, exists := epicIDs[epic.ID]; exists {
			return fmt.Errorf("duplicate epic id %q", epic.ID)
		}
		epicIDs[epic.ID] = struct{}{}
		epicAdj[epic.ID] = append([]string(nil), epic.DependsOn...)

		for _, story := range epic.Stories {
			if _, exists := storyIDs[story.ID]; exists {
				return fmt.Errorf("duplicate story id %q", story.ID)
			}
			storyIDs[story.ID] = struct{}{}
			storyAdj[story.ID] = append([]string(nil), story.DependsOn...)
		}
	}

	for node, deps := range epicAdj {
		for _, dep := range deps {
			if _, exists := epicIDs[dep]; !exists {
				return fmt.Errorf("unknown dependency %q referenced by epic %q", dep, node)
			}
		}
	}

	for node, deps := range storyAdj {
		for _, dep := range deps {
			if _, exists := storyIDs[dep]; !exists {
				return fmt.Errorf("unknown dependency %q referenced by story %q", dep, node)
			}
		}
	}

	if cyclePath, hasCycle := findCycle(epicAdj); hasCycle {
		return fmt.Errorf("circular dependency detected: %s", strings.Join(cyclePath, " -> "))
	}

	if cyclePath, hasCycle := findCycle(storyAdj); hasCycle {
		return fmt.Errorf("circular dependency detected: %s", strings.Join(cyclePath, " -> "))
	}

	return nil
}

func findCycle(adj map[string][]string) ([]string, bool) {
	const (
		stateUnvisited = iota
		stateVisiting
		stateVisited
	)

	state := make(map[string]int, len(adj))
	stack := make([]string, 0, len(adj))
	indexByNode := make(map[string]int, len(adj))

	var visit func(node string) ([]string, bool)
	visit = func(node string) ([]string, bool) {
		state[node] = stateVisiting
		indexByNode[node] = len(stack)
		stack = append(stack, node)

		for _, dep := range adj[node] {
			if state[dep] == stateUnvisited {
				if cycle, ok := visit(dep); ok {
					return cycle, true
				}
				continue
			}

			if state[dep] == stateVisiting {
				start := indexByNode[dep]
				cycle := append([]string(nil), stack[start:]...)
				cycle = append(cycle, dep)
				return cycle, true
			}
		}

		stack = stack[:len(stack)-1]
		delete(indexByNode, node)
		state[node] = stateVisited
		return nil, false
	}

	nodes := make([]string, 0, len(adj))
	for node := range adj {
		nodes = append(nodes, node)
	}
	sort.Strings(nodes)

	for _, node := range nodes {
		if state[node] != stateUnvisited {
			continue
		}

		if cycle, ok := visit(node); ok {
			return cycle, true
		}
	}

	return nil, false
}
