package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/guillaume7/loom/internal/depgraph"
	"github.com/guillaume7/loom/internal/fsm"
	"github.com/guillaume7/loom/internal/store"
	mcplib "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

func (s *Server) handleNextStep(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	const toolName = "loom_next_step"
	start := time.Now()

	if res, cancelled := checkCtx(ctx, toolName); cancelled {
		return res, nil
	}

	operationKey, hasOperationKey, err := optionalStringArgument(req, "operation_key")
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}

	if hasOperationKey {
		s.mu.Lock()
		defer s.mu.Unlock()

		cached, found, lookupErr := s.readActionByOperationKey(ctx, toolName, operationKey)
		if lookupErr != nil {
			return lookupErr, nil
		}
		if found {
			slog.InfoContext(ctx, "tool completed", "tool", toolName, "operation_key", operationKey, "cached", true, "duration_ms", time.Since(start).Milliseconds())
			return mcplib.NewToolResultText(cached.Detail), nil
		}

		currentState := s.machine.State()
		slog.InfoContext(ctx, "tool called", "tool", toolName, "state", string(currentState))

		result := NextStepResult{
			State:       string(currentState),
			Instruction: stateInstruction(currentState),
		}
		detail, marshalErr := marshalResultText(result)
		if marshalErr != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("failed to marshal result: %v", marshalErr)), nil
		}

		action := store.Action{
			SessionID:    sessionIDFromContext(ctx),
			OperationKey: operationKey,
			StateBefore:  string(currentState),
			StateAfter:   string(currentState),
			Event:        "next_step",
			Detail:       detail,
		}
		if res, handled := s.writeActionOrReturnCached(ctx, toolName, action); handled {
			return res, nil
		}

		slog.InfoContext(ctx, "tool completed", "tool", toolName, "state", string(currentState), "duration_ms", time.Since(start).Milliseconds())
		return mcplib.NewToolResultText(detail), nil
	}

	s.mu.RLock()
	currentState := s.machine.State()
	s.mu.RUnlock()

	slog.InfoContext(ctx, "tool called", "tool", toolName, "state", string(currentState))

	result := NextStepResult{
		State:       string(currentState),
		Instruction: stateInstruction(currentState),
	}

	slog.InfoContext(ctx, "tool completed", "tool", toolName, "state", string(currentState), "duration_ms", time.Since(start).Milliseconds())
	return toolResultJSON(result), nil
}

func (s *Server) handleCheckpoint(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	const toolName = "loom_checkpoint"
	start := time.Now()

	// Guard against cancelled context before acquiring any locks or mutating state.
	if res, cancelled := checkCtx(ctx, toolName); cancelled {
		return res, nil
	}

	// Decode the typed checkpoint request from generic MCP arguments using
	// direct type assertions, avoiding an unnecessary JSON round-trip.
	actionStr, _ := req.Params.Arguments["action"].(string)
	if actionStr == "" {
		// Accept "event" for backward compatibility.
		actionStr, _ = req.Params.Arguments["event"].(string)
	}
	if actionStr == "" {
		return mcplib.NewToolResultError("missing or invalid 'action' argument: must be a non-empty string"), nil
	}

	operationKey, hasOperationKey, err := optionalStringArgument(req, "operation_key")
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}

	if hasOperationKey {
		s.mu.Lock()
		defer s.mu.Unlock()

		previousState := s.machine.State()
		s.lastActivity = s.clock.Now()
		slog.InfoContext(ctx, "tool called", "tool", toolName, "state", string(previousState))

		cached, found, lookupErr := s.readActionByOperationKey(ctx, toolName, operationKey)
		if lookupErr != nil {
			return lookupErr, nil
		}
		if found {
			slog.InfoContext(ctx, "tool completed", "tool", toolName, "state", string(previousState), "operation_key", operationKey, "cached", true, "duration_ms", time.Since(start).Milliseconds())
			return mcplib.NewToolResultText(cached.Detail), nil
		}

		// Snapshot the FSM before transitioning so we can roll back if the
		// subsequent store write fails. This guarantees that a retry with the
		// same operation_key fires the event from the correct starting state.
		snap := s.machine.TakeSnapshot()

		newState, transErr := s.machine.Transition(fsm.Event(actionStr))
		if transErr != nil {
			slog.InfoContext(ctx, "tool completed", "tool", toolName, "state", string(previousState), "duration_ms", time.Since(start).Milliseconds())
			return mcplib.NewToolResultError(transErr.Error()), nil
		}

		cp := s.readCheckpoint(ctx, toolName)
		result := CheckpointResult{
			PreviousState: string(previousState),
			NewState:      string(newState),
			Phase:         cp.Phase,
		}
		detail, marshalErr := marshalResultText(result)
		if marshalErr != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("failed to marshal result: %v", marshalErr)), nil
		}

		// Atomically persist checkpoint and action log entry so that a partial
		// failure can never leave the store in a state where the checkpoint is
		// written but the action log entry is absent. Either both are committed
		// or neither is, making a retry with the same operation_key safe.
		writeErr := s.st.WriteCheckpointAndAction(ctx,
			store.Checkpoint{State: string(newState), Phase: cp.Phase},
			store.Action{
				SessionID:    sessionIDFromContext(ctx),
				OperationKey: operationKey,
				StateBefore:  string(previousState),
				StateAfter:   string(newState),
				Event:        actionStr,
				Detail:       detail,
			},
		)
		if writeErr != nil {
			if errors.Is(writeErr, store.ErrDuplicateOperationKey) {
				// A concurrent attempt already committed both writes; replay the
				// cached result.
				cached, found, lookupErr := s.readActionByOperationKey(ctx, toolName, operationKey)
				if lookupErr != nil {
					s.machine.Rollback(snap)
					return lookupErr, nil
				}
				if !found {
					s.machine.Rollback(snap)
					return mcplib.NewToolResultError(fmt.Sprintf("duplicate operation key without cached result: %s", operationKey)), nil
				}
				slog.InfoContext(ctx, "returning cached tool result", "tool", toolName, "operation_key", operationKey)
				return mcplib.NewToolResultText(cached.Detail), nil
			}
			// Non-duplicate write failure: roll back the in-memory FSM so that
			// a retry with the same operation_key fires the event from the
			// correct starting state instead of failing with an invalid transition.
			s.machine.Rollback(snap)
			slog.ErrorContext(ctx, "store write error", "tool", toolName, "state", string(newState), "error", writeErr)
			slog.InfoContext(ctx, "tool completed", "tool", toolName, "state", string(previousState), "duration_ms", time.Since(start).Milliseconds(), "error", writeErr)
			return mcplib.NewToolResultError(fmt.Sprintf("failed to persist checkpoint: %v", writeErr)), nil
		}

		slog.InfoContext(ctx, "tool completed", "tool", toolName, "state", string(newState), "duration_ms", time.Since(start).Milliseconds())
		return mcplib.NewToolResultText(detail), nil
	}

	s.mu.Lock()
	previousState := s.machine.State()
	s.lastActivity = s.clock.Now() // Record activity time; resets the stall timer.
	slog.InfoContext(ctx, "tool called", "tool", toolName, "state", string(previousState))
	// Snapshot the FSM before transitioning so the in-memory state can be
	// rolled back if the subsequent non-idempotent checkpoint write fails.
	snap := s.machine.TakeSnapshot()
	newState, transErr := s.machine.Transition(fsm.Event(actionStr))
	if transErr != nil {
		s.mu.Unlock()
		slog.InfoContext(ctx, "tool completed", "tool", toolName, "state", string(previousState), "duration_ms", time.Since(start).Milliseconds())
		return mcplib.NewToolResultError(transErr.Error()), nil
	}

	cp := s.readCheckpoint(ctx, toolName)
	if writeErr := s.st.WriteCheckpoint(ctx, store.Checkpoint{State: string(newState), Phase: cp.Phase}); writeErr != nil {
		s.machine.Rollback(snap)
		s.mu.Unlock()
		slog.ErrorContext(ctx, "store write error", "tool", toolName, "state", string(newState), "error", writeErr)
		slog.InfoContext(ctx, "tool completed", "tool", toolName, "state", string(previousState), "duration_ms", time.Since(start).Milliseconds(), "error", writeErr)
		return mcplib.NewToolResultError(fmt.Sprintf("failed to persist checkpoint: %v", writeErr)), nil
	}
	s.mu.Unlock()

	result := CheckpointResult{
		PreviousState: string(previousState),
		NewState:      string(newState),
		Phase:         cp.Phase,
	}

	slog.InfoContext(ctx, "tool completed", "tool", toolName, "state", string(newState), "duration_ms", time.Since(start).Milliseconds())
	return toolResultJSON(result), nil
}

func (s *Server) handleHeartbeat(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	const toolName = "loom_heartbeat"
	start := time.Now()

	if res, cancelled := checkCtx(ctx, toolName); cancelled {
		return res, nil
	}

	s.mu.RLock()
	currentState := s.machine.State()
	s.mu.RUnlock()

	slog.InfoContext(ctx, "tool called", "tool", toolName, "state", string(currentState))

	cp := s.readCheckpoint(ctx, toolName)

	gate := isGateState(currentState)
	retry := 0
	if gate {
		retry = retryInSeconds
	}
	result := HeartbeatResult{
		State:          string(currentState),
		Phase:          cp.Phase,
		Wait:           gate,
		RetryInSeconds: retry,
	}

	slog.InfoContext(ctx, "tool completed", "tool", toolName, "state", string(currentState), "duration_ms", time.Since(start).Milliseconds())
	return toolResultJSON(result), nil
}

func (s *Server) handleGetState(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	const toolName = "loom_get_state"
	start := time.Now()

	if res, cancelled := checkCtx(ctx, toolName); cancelled {
		return res, nil
	}

	s.mu.RLock()
	currentState := s.machine.State()
	s.mu.RUnlock()

	slog.InfoContext(ctx, "tool called", "tool", toolName, "state", string(currentState))

	cp := s.readCheckpoint(ctx, toolName)
	result := GetStateResult{
		State: string(currentState),
		Phase: cp.Phase,
	}

	slog.InfoContext(ctx, "tool completed", "tool", toolName, "state", string(currentState), "duration_ms", time.Since(start).Milliseconds())
	return toolResultJSON(result), nil
}

func (s *Server) handleAbort(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	const toolName = "loom_abort"
	start := time.Now()

	// Guard against cancelled context before acquiring any locks or mutating state.
	if res, cancelled := checkCtx(ctx, toolName); cancelled {
		return res, nil
	}

	s.mu.Lock()
	currentState := s.machine.State()
	slog.InfoContext(ctx, "tool called", "tool", toolName, "state", string(currentState))
	newState, transErr := s.machine.Transition(fsm.EventAbort)
	s.mu.Unlock()

	if transErr != nil {
		// EventAbort is universally accepted; this branch is defensive only.
		slog.InfoContext(ctx, "tool completed", "tool", toolName, "state", string(currentState), "duration_ms", time.Since(start).Milliseconds())
		return mcplib.NewToolResultError(transErr.Error()), nil
	}

	cp := s.readCheckpoint(ctx, toolName)
	if writeErr := s.st.WriteCheckpoint(ctx, store.Checkpoint{State: string(newState), Phase: cp.Phase}); writeErr != nil {
		slog.ErrorContext(ctx, "store write error", "tool", toolName, "state", string(newState), "error", writeErr)
		slog.InfoContext(ctx, "tool completed", "tool", toolName, "state", string(newState), "duration_ms", time.Since(start).Milliseconds(), "error", writeErr)
		return mcplib.NewToolResultError(fmt.Sprintf("failed to persist abort state: %v", writeErr)), nil
	}

	result := AbortResult{State: string(newState)}

	slog.InfoContext(ctx, "tool completed", "tool", toolName, "state", string(newState), "duration_ms", time.Since(start).Milliseconds())
	return toolResultJSON(result), nil
}

// MCPServer builds and returns a configured *server.MCPServer with all five
// Loom tools registered. Each call returns a fresh MCPServer instance sharing
// the same underlying FSM and store.
func (s *Server) MCPServer() *mcpserver.MCPServer {
	srv := mcpserver.NewMCPServer("loom", "0.1.0", mcpserver.WithInstructions(s.buildServerInstructions(context.Background())))

	s.mu.Lock()
	s.emitter = NewTaskEmitter(srv)
	s.mu.Unlock()

	srv.AddTool(
		mcplib.NewTool("loom_next_step",
			mcplib.WithDescription("Returns the current workflow state and the next action the agent should take"),
			mcplib.WithString("operation_key",
				mcplib.Description("Optional idempotency key used to return a cached result for retried calls"),
			),
		),
		s.handleNextStep,
	)
	srv.AddTool(
		mcplib.NewTool("loom_checkpoint",
			mcplib.WithDescription("Fires an event on the Loom FSM to advance the workflow and persists the new state"),
			mcplib.WithString("action",
				mcplib.Required(),
				mcplib.Description("The action to fire (e.g. start, pr_opened, ci_green, ci_red, review_approved, abort)"),
			),
			mcplib.WithString("operation_key",
				mcplib.Description("Optional idempotency key used to return a cached result for retried calls"),
			),
		),
		s.handleCheckpoint,
	)
	srv.AddTool(
		mcplib.NewTool("loom_heartbeat",
			mcplib.WithDescription("Returns the current FSM state and phase as a health-check"),
		),
		s.handleHeartbeat,
	)
	srv.AddTool(
		mcplib.NewTool("loom_get_state",
			mcplib.WithDescription("Returns the current FSM state and epic phase (read-only)"),
		),
		s.handleGetState,
	)
	srv.AddTool(
		mcplib.NewTool("loom_abort",
			mcplib.WithDescription("Aborts the current workflow by transitioning the FSM to PAUSED"),
		),
		s.handleAbort,
	)

	srv.AddResource(
		mcplib.Resource{
			URI:         "loom://dependencies",
			Name:        "Dependency Graph",
			Description: "Raw YAML content of .loom/dependencies.yaml",
			MIMEType:    "text/yaml",
		},
		func(_ context.Context, _ mcplib.ReadResourceRequest) ([]mcplib.ResourceContents, error) {
			content, err := os.ReadFile(".loom/dependencies.yaml")
			if err != nil {
				if os.IsNotExist(err) {
					return nil, fmt.Errorf("loom://dependencies: .loom/dependencies.yaml not found")
				}
				return nil, fmt.Errorf("loom://dependencies: %w", err)
			}
			return []mcplib.ResourceContents{
				mcplib.TextResourceContents{
					URI:      "loom://dependencies",
					MIMEType: "text/yaml",
					Text:     string(content),
				},
			}, nil
		},
	)

	srv.AddResource(
		mcplib.Resource{
			URI:         "loom://state",
			Name:        "Workflow State",
			Description: "Current FSM state and phase as JSON",
			MIMEType:    "application/json",
		},
		func(_ context.Context, _ mcplib.ReadResourceRequest) ([]mcplib.ResourceContents, error) {
			s.mu.RLock()
			currentState := s.machine.State()
			s.mu.RUnlock()

			cp, err := s.st.ReadCheckpoint(context.Background())
			if err != nil {
				slog.Info("loom://state checkpoint read failed", "error", err)
			}

			unblockedStories := []string{}
			graph, depErr := depgraph.Load(".loom/dependencies.yaml")
			if depErr == nil {
				unblockedStories = graph.Unblocked(nil)
			} else if !os.IsNotExist(depErr) {
				slog.Info("loom://state dependency graph unavailable", "error", depErr)
			}

			updatedAt := ""
			if !cp.UpdatedAt.IsZero() {
				updatedAt = cp.UpdatedAt.UTC().Format(time.RFC3339Nano)
			}

			result := struct {
				State            string   `json:"state"`
				Phase            int      `json:"phase"`
				PRNumber         int      `json:"pr_number"`
				IssueNumber      int      `json:"issue_number"`
				RetryCount       int      `json:"retry_count"`
				UpdatedAt        string   `json:"updated_at"`
				UnblockedStories []string `json:"unblocked_stories"`
			}{
				State:            string(currentState),
				Phase:            cp.Phase,
				PRNumber:         cp.PRNumber,
				IssueNumber:      cp.IssueNumber,
				RetryCount:       cp.RetryCount,
				UpdatedAt:        updatedAt,
				UnblockedStories: unblockedStories,
			}

			payload, marshalErr := marshalResultText(result)
			if marshalErr != nil {
				return nil, fmt.Errorf("loom://state: %w", marshalErr)
			}

			return []mcplib.ResourceContents{
				mcplib.TextResourceContents{
					URI:      "loom://state",
					MIMEType: "application/json",
					Text:     payload,
				},
			}, nil
		},
	)

	srv.AddResource(
		mcplib.Resource{
			URI:         "loom://log",
			Name:        "Action Log",
			Description: "NDJSON of the last 200 action_log entries",
			MIMEType:    "application/x-ndjson",
		},
		func(_ context.Context, _ mcplib.ReadResourceRequest) ([]mcplib.ResourceContents, error) {
			actions, err := s.st.ReadActions(context.Background(), 200)
			if err != nil {
				return nil, fmt.Errorf("loom://log: %w", err)
			}
			var sb strings.Builder
			for index := len(actions) - 1; index >= 0; index-- {
				a := actions[index]
				line, err := json.Marshal(map[string]any{"id": a.ID, "session_id": a.SessionID, "operation_key": a.OperationKey, "state_before": a.StateBefore, "state_after": a.StateAfter, "event": a.Event, "detail": a.Detail, "created_at": a.CreatedAt})
				if err != nil {
					return nil, fmt.Errorf("loom://log: failed to marshal action %d: %w", a.ID, err)
				}
				sb.Write(line)
				sb.WriteByte('\n')
			}
			return []mcplib.ResourceContents{mcplib.TextResourceContents{URI: "loom://log", MIMEType: "application/x-ndjson", Text: sb.String()}}, nil
		},
	)
	s.mu.RLock()
	resources := make([]resourceEntry, len(s.resources))
	copy(resources, s.resources)
	s.mu.RUnlock()

	for _, r := range resources {
		srv.AddResource(r.resource, r.handler)
	}

	return srv
}

// Serve starts the MCP stdio server, blocking until ctx is cancelled or an
// error is encountered on stdin/stdout. It also starts the session monitor
// goroutine (heartbeat + stall detection) before beginning to handle messages.
func (s *Server) Serve(ctx context.Context, stdin io.Reader, stdout io.Writer) error {
	s.startMonitor(ctx)
	mcpSvr := s.MCPServer()
	stdioSvr := mcpserver.NewStdioServer(mcpSvr)
	return stdioSvr.Listen(ctx, stdin, stdout)
}
