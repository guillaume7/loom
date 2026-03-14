package mcp

import (
	"context"
	"log/slog"
	"strings"

	"github.com/guillaume7/loom/internal/store"
)

const storyIDEnvVar = "LOOM_STORY_ID"

type storyCheckpointStore interface {
	ReadCheckpointByStoryID(ctx context.Context, storyID string) (store.Checkpoint, error)
	WriteCheckpointByStoryID(ctx context.Context, storyID string, cp store.Checkpoint) error
	WriteCheckpointAndActionByStoryID(ctx context.Context, storyID string, cp store.Checkpoint, action store.Action) error
}

func (s *Server) readCheckpoint(ctx context.Context, toolName string) store.Checkpoint {
	cp, err := s.readCheckpointWithErr(ctx)
	if err != nil {
		slog.InfoContext(ctx, "store read error", "tool", toolName, "story_id", s.storyID, "error", err)
	}
	return cp
}

func (s *Server) readCheckpointWithErr(ctx context.Context) (store.Checkpoint, error) {
	var (
		cp  store.Checkpoint
		err error
	)
	if scoped, ok := s.st.(storyCheckpointStore); ok && s.storyID != "" {
		cp, err = scoped.ReadCheckpointByStoryID(ctx, s.storyID)
	} else {
		cp, err = s.st.ReadCheckpoint(ctx)
	}
	return cp, err
}

func (s *Server) writeCheckpoint(ctx context.Context, cp store.Checkpoint) error {
	if scoped, ok := s.st.(storyCheckpointStore); ok && s.storyID != "" {
		return scoped.WriteCheckpointByStoryID(ctx, s.storyID, cp)
	}
	return s.st.WriteCheckpoint(ctx, cp)
}

func (s *Server) writeCheckpointAndAction(ctx context.Context, cp store.Checkpoint, action store.Action) error {
	action.OperationKey = s.scopedOperationKey(action.OperationKey)
	if scoped, ok := s.st.(storyCheckpointStore); ok && s.storyID != "" {
		return scoped.WriteCheckpointAndActionByStoryID(ctx, s.storyID, cp, action)
	}
	return s.st.WriteCheckpointAndAction(ctx, cp, action)
}

func normalizeStoryID(storyID string) string {
	return strings.TrimSpace(storyID)
}
