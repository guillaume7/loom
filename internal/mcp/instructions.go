package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/guillaume7/loom/internal/depgraph"
)

// buildServerInstructions returns the instructions payload used in initialize
// responses. It is rebuilt for every MCPServer() call so the content reflects
// current state on each server initialization.
func (s *Server) buildServerInstructions(ctx context.Context) string {
	s.mu.RLock()
	currentState := s.machine.State()
	s.mu.RUnlock()

	cp, err := s.st.ReadCheckpoint(ctx)
	if err != nil {
		slog.InfoContext(ctx, "mcp instructions checkpoint read failed", "error", err)
	}

	instructions := fmt.Sprintf(
		"Loom workflow assistant. Current state: %s (phase %d).\n\nPhase summary: %s\n\nDependency digest:\n",
		string(currentState),
		cp.Phase,
		stateInstruction(currentState),
	)

	graph, depErr := depgraph.Load(".loom/dependencies.yaml")
	if depErr != nil {
		if !os.IsNotExist(depErr) {
			slog.InfoContext(ctx, "mcp instructions dependency graph unavailable", "error", depErr)
		}
		return instructions + "No dependency graph loaded"
	}

	unblockedStories := graph.Unblocked(nil)
	unblockedText := "none"
	if len(unblockedStories) > 0 {
		unblockedText = strings.Join(unblockedStories, ", ")
	}

	blockedCount := 0
	for _, epic := range graph.Epics {
		for _, story := range epic.Stories {
			blocked, err := graph.IsBlocked(story.ID, nil)
			if err != nil {
				continue
			}
			if blocked {
				blockedCount++
			}
		}
	}

	return fmt.Sprintf(
		"%s- Unblocked stories: %s\n- Blocked stories: %d story IDs blocked by incomplete dependencies",
		instructions,
		unblockedText,
		blockedCount,
	)
}
