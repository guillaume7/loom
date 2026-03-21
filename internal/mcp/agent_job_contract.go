package mcp

import (
	"fmt"
	"strings"
	"time"

	"github.com/guillaume7/loom/internal/agentspawn"
	"github.com/guillaume7/loom/internal/store"
)

const (
	defaultAgentJobTimeout        = 30 * time.Minute
	defaultAgentExpectedOutput    = "Implement the requested story and return completed code changes with tests"
	unknownAgentJobSessionID      = "session-unknown"
	unknownAgentJobAttemptStoryID = "story-unknown"
)

// nextAttemptNumber counts prior background_agent_spawned actions for storyID
// in the given action list and returns count+1 (minimum 1).
func nextAttemptNumber(storyID string, actions []store.Action) int {
	count := 0
	for _, a := range actions {
		if a.Event == "background_agent_spawned" && storyIDFromActionDetail(a.Detail) == storyID {
			count++
		}
	}
	return count + 1
}

func newAgentJobContract(now time.Time, sessionID, storyID, input string, attempt int) agentspawn.JobContract {
	trimmedSessionID := strings.TrimSpace(sessionID)
	if trimmedSessionID == "" {
		trimmedSessionID = unknownAgentJobSessionID
	}
	trimmedStoryID := strings.TrimSpace(storyID)
	if trimmedStoryID == "" {
		trimmedStoryID = unknownAgentJobAttemptStoryID
	}
	timestamp := now.UTC()
	return agentspawn.JobContract{
		JobID:          fmt.Sprintf("job:%s:%s:%d:%d", trimmedSessionID, trimmedStoryID, attempt, timestamp.UnixNano()),
		StoryID:        storyID,
		Attempt:        attempt,
		Input:          input,
		ExpectedOutput: defaultAgentExpectedOutput,
		Deadline:       timestamp.Add(defaultAgentJobTimeout),
	}
}