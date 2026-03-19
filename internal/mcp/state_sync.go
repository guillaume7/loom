package mcp

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/guillaume7/loom/internal/fsm"
	"github.com/guillaume7/loom/internal/store"
)

// syncMachineToCheckpoint rehydrates the in-memory FSM from the persisted
// checkpoint when they diverge. The persisted checkpoint is the authoritative
// source of truth across long-lived MCP sessions.
func (s *Server) syncMachineToCheckpoint(ctx context.Context, toolName string) (store.Checkpoint, fsm.State, error) {
	cp, err := s.readCheckpointWithErr(ctx)
	if err != nil {
		return store.Checkpoint{}, "", fmt.Errorf("failed to read checkpoint: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if !cp.UpdatedAt.IsZero() && cp.UpdatedAt.After(s.lastActivity) {
		s.lastActivity = cp.UpdatedAt
	}

	currentState := s.machine.State()
	if cp.State == "" {
		return cp, currentState, nil
	}

	desiredState := fsm.State(cp.State)
	if desiredState == currentState {
		return cp, currentState, nil
	}

	if err := s.machine.Hydrate(desiredState); err != nil {
		return cp, currentState, fmt.Errorf("failed to hydrate machine state from checkpoint: %w", err)
	}

	s.activeElicitation = elicitationContext{}
	slog.InfoContext(ctx, "rehydrated machine from checkpoint", "tool", toolName, "state", cp.State)
	return cp, desiredState, nil
}
