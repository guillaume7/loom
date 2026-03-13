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

// Unblocked returns deterministic story IDs eligible for execution:
// stories that are not done, whose story dependencies are done,
// and whose epic-level dependencies are satisfied.
func (g Graph) Unblocked(done []string) []string {
	doneSet := make(map[string]struct{}, len(done))
	for _, id := range done {
		doneSet[id] = struct{}{}
	}

	epicsByID, storyByID, storyEpic := g.indexes()

	ids := make([]string, 0)
	for id := range storyByID {
		if _, alreadyDone := doneSet[id]; alreadyDone {
			continue
		}

		blocked, err := g.isBlockedWithIndex(id, doneSet, epicsByID, storyByID, storyEpic)
		if err != nil {
			// Indexes come from a validated graph; unknown IDs are not expected here.
			continue
		}

		if !blocked {
			ids = append(ids, id)
		}
	}

	sort.Strings(ids)
	return ids
}

// IsBlocked reports whether a story is blocked given the done set.
// Returns an error when the story ID does not exist in the graph.
func (g Graph) IsBlocked(id string, done []string) (bool, error) {
	doneSet := make(map[string]struct{}, len(done))
	for _, doneID := range done {
		doneSet[doneID] = struct{}{}
	}

	epicsByID, storyByID, storyEpic := g.indexes()
	return g.isBlockedWithIndex(id, doneSet, epicsByID, storyByID, storyEpic)
}

func (g Graph) isBlockedWithIndex(
	id string,
	doneSet map[string]struct{},
	epicsByID map[string]Epic,
	storyByID map[string]Story,
	storyEpic map[string]string,
) (bool, error) {
	story, ok := storyByID[id]
	if !ok {
		return false, fmt.Errorf("unknown story id %q", id)
	}

	for _, dep := range story.DependsOn {
		if _, done := doneSet[dep]; !done {
			return true, nil
		}
	}

	epicID := storyEpic[id]
	epic := epicsByID[epicID]
	for _, depEpicID := range epic.DependsOn {
		if !epicComplete(depEpicID, doneSet, epicsByID) {
			return true, nil
		}
	}

	return false, nil
}

func epicComplete(epicID string, doneSet map[string]struct{}, epicsByID map[string]Epic) bool {
	epic, ok := epicsByID[epicID]
	if !ok {
		return false
	}

	for _, s := range epic.Stories {
		if _, done := doneSet[s.ID]; !done {
			return false
		}
	}

	return true
}

func (g Graph) indexes() (map[string]Epic, map[string]Story, map[string]string) {
	epicsByID := make(map[string]Epic, len(g.Epics))
	storyByID := make(map[string]Story)
	storyEpic := make(map[string]string)

	for _, epic := range g.Epics {
		epicsByID[epic.ID] = epic
		for _, story := range epic.Stories {
			storyByID[story.ID] = story
			storyEpic[story.ID] = epic.ID
		}
	}

	return epicsByID, storyByID, storyEpic
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
