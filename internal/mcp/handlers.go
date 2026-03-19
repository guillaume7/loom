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

func isBudgetExhaustionPause(previousState fsm.State, event fsm.Event, newState fsm.State) bool {
	if newState != fsm.StatePaused {
		return false
	}

	switch previousState {
	case fsm.StateAwaitingPR:
		return event == fsm.EventTimeout
	case fsm.StateAwaitingCI:
		return event == fsm.EventTimeout || event == fsm.EventCIRed
	case fsm.StateReviewing:
		return event == fsm.EventReviewChangesRequested
	default:
		return false
	}
}

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
	args := req.GetArguments()
	actionStr, _ := args["action"].(string)
	if actionStr == "" {
		// Accept "event" for backward compatibility.
		actionStr, _ = args["event"].(string)
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
		event := fsm.Event(actionStr)
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

		newState, transErr := s.machine.Transition(event)
		if transErr != nil {
			slog.InfoContext(ctx, "tool completed", "tool", toolName, "state", string(previousState), "duration_ms", time.Since(start).Milliseconds())
			return mcplib.NewToolResultError(transErr.Error()), nil
		}

		emitBudgetExhaustion := false
		emitPRNumber := 0

		if isBudgetExhaustionPause(previousState, event, newState) {
			supportsElicitation := s.sessionElicitationSupport[sessionIDFromContext(ctx)]
			if supportsElicitation {
				s.machine.Rollback(snap)
				newState = previousState

				cp, readErr := s.readCheckpointWithErr(ctx)
				if readErr != nil {
					return mcplib.NewToolResultError(fmt.Sprintf("failed to read checkpoint: %v", readErr)), nil
				}
				emitBudgetExhaustion = true
				emitPRNumber = cp.PRNumber
			} else {
				slog.InfoContext(ctx, "elicitation unsupported by client; using v1 pause fallback", "tool", toolName, "state", string(previousState), "event", string(event))
			}
		}

		cp, readErr := s.readCheckpointWithErr(ctx)
		if readErr != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("failed to read checkpoint: %v", readErr)), nil
		}
		nextPhase := cp.Phase
		nextRetryCount := cp.RetryCount
		detail := ""
		if event == fsm.EventSkipStory {
			nextPhase = cp.Phase + 1
			nextRetryCount = 0
			detail = fmt.Sprintf("skipped story at phase %d", cp.Phase)
		}

		result := CheckpointResult{
			PreviousState: string(previousState),
			NewState:      string(newState),
			Phase:         nextPhase,
			Detail:        detail,
		}
		detail, marshalErr := marshalResultText(result)
		if marshalErr != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("failed to marshal result: %v", marshalErr)), nil
		}

		// Atomically persist checkpoint and action log entry so that a partial
		// failure can never leave the store in a state where the checkpoint is
		// written but the action log entry is absent. Either both are committed
		// or neither is, making a retry with the same operation_key safe.
		nextCheckpoint := cp
		nextCheckpoint.State = string(newState)
		nextCheckpoint.Phase = nextPhase
		nextCheckpoint.RetryCount = nextRetryCount
		writeErr := s.writeCheckpointAndAction(ctx,
			nextCheckpoint,
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

		if emitBudgetExhaustion {
			emitter := s.elicitationEmitter
			if emitter == nil {
				return mcplib.NewToolResultError("elicitation emitter is not initialized"), nil
			}
			if emitErr := emitter.BudgetExhaustion(ctx, emitPRNumber, previousState, fsm.Event(actionStr)); emitErr != nil {
				return mcplib.NewToolResultError(fmt.Sprintf("failed to emit elicitation: %v", emitErr)), nil
			}
			s.activeElicitation = elicitationContext{
				Active:   true,
				PRNumber: emitPRNumber,
				State:    previousState,
				Event:    event,
			}
			s.appendTrace(ctx, store.TraceEvent{
				Kind:      TraceKindIntervention,
				FromState: string(previousState),
				ToState:   string(newState),
				Event:     actionStr,
				Reason:    "budget exhaustion — elicitation prompt emitted",
				PRNumber:  emitPRNumber,
			})
		} else {
			s.activeElicitation = elicitationContext{}
			s.appendTrace(ctx, store.TraceEvent{
				Kind:        TraceKindTransition,
				FromState:   string(previousState),
				ToState:     string(newState),
				Event:       actionStr,
				PRNumber:    nextCheckpoint.PRNumber,
				IssueNumber: nextCheckpoint.IssueNumber,
			})
		}

		slog.InfoContext(ctx, "tool completed", "tool", toolName, "state", string(newState), "duration_ms", time.Since(start).Milliseconds())
		return mcplib.NewToolResultText(detail), nil
	}

	s.mu.Lock()
	previousState := s.machine.State()
	event := fsm.Event(actionStr)
	s.lastActivity = s.clock.Now() // Record activity time; resets the stall timer.
	slog.InfoContext(ctx, "tool called", "tool", toolName, "state", string(previousState))
	// Snapshot the FSM before transitioning so the in-memory state can be
	// rolled back if the subsequent non-idempotent checkpoint write fails.
	snap := s.machine.TakeSnapshot()
	newState, transErr := s.machine.Transition(event)
	if transErr != nil {
		s.mu.Unlock()
		slog.InfoContext(ctx, "tool completed", "tool", toolName, "state", string(previousState), "duration_ms", time.Since(start).Milliseconds())
		return mcplib.NewToolResultError(transErr.Error()), nil
	}

	emitBudgetExhaustion := false
	emitPRNumber := 0

	if isBudgetExhaustionPause(previousState, event, newState) {
		supportsElicitation := s.sessionElicitationSupport[sessionIDFromContext(ctx)]
		if supportsElicitation {
			s.machine.Rollback(snap)
			newState = previousState

			cp, readErr := s.readCheckpointWithErr(ctx)
			if readErr != nil {
				s.machine.Rollback(snap)
				s.mu.Unlock()
				return mcplib.NewToolResultError(fmt.Sprintf("failed to read checkpoint: %v", readErr)), nil
			}
			emitBudgetExhaustion = true
			emitPRNumber = cp.PRNumber
		} else {
			slog.InfoContext(ctx, "elicitation unsupported by client; using v1 pause fallback", "tool", toolName, "state", string(previousState), "event", string(event))
		}
	}

	cp, readErr := s.readCheckpointWithErr(ctx)
	if readErr != nil {
		s.machine.Rollback(snap)
		s.mu.Unlock()
		return mcplib.NewToolResultError(fmt.Sprintf("failed to read checkpoint: %v", readErr)), nil
	}
	nextPhase := cp.Phase
	nextRetryCount := cp.RetryCount
	detail := ""
	if event == fsm.EventSkipStory {
		nextPhase = cp.Phase + 1
		nextRetryCount = 0
		detail = fmt.Sprintf("skipped story at phase %d", cp.Phase)
	}

	nextCheckpoint := cp
	nextCheckpoint.State = string(newState)
	nextCheckpoint.Phase = nextPhase
	nextCheckpoint.RetryCount = nextRetryCount
	if writeErr := s.writeCheckpoint(ctx, nextCheckpoint); writeErr != nil {
		s.machine.Rollback(snap)
		s.mu.Unlock()
		slog.ErrorContext(ctx, "store write error", "tool", toolName, "state", string(newState), "error", writeErr)
		slog.InfoContext(ctx, "tool completed", "tool", toolName, "state", string(previousState), "duration_ms", time.Since(start).Milliseconds(), "error", writeErr)
		return mcplib.NewToolResultError(fmt.Sprintf("failed to persist checkpoint: %v", writeErr)), nil
	}
	s.mu.Unlock()

	if emitBudgetExhaustion {
		emitter := s.elicitationEmitter
		if emitter == nil {
			return mcplib.NewToolResultError("elicitation emitter is not initialized"), nil
		}
		if emitErr := emitter.BudgetExhaustion(ctx, emitPRNumber, previousState, fsm.Event(actionStr)); emitErr != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("failed to emit elicitation: %v", emitErr)), nil
		}
		s.mu.Lock()
		s.activeElicitation = elicitationContext{
			Active:   true,
			PRNumber: emitPRNumber,
			State:    previousState,
			Event:    event,
		}
		s.mu.Unlock()
		s.appendTrace(ctx, store.TraceEvent{
			Kind:      TraceKindIntervention,
			FromState: string(previousState),
			ToState:   string(newState),
			Event:     actionStr,
			Reason:    "budget exhaustion — elicitation prompt emitted",
			PRNumber:  emitPRNumber,
		})
	} else {
		s.mu.Lock()
		s.activeElicitation = elicitationContext{}
		s.mu.Unlock()
		s.appendTrace(ctx, store.TraceEvent{
			Kind:        TraceKindTransition,
			FromState:   string(previousState),
			ToState:     string(newState),
			Event:       actionStr,
			PRNumber:    nextCheckpoint.PRNumber,
			IssueNumber: nextCheckpoint.IssueNumber,
		})
	}

	result := CheckpointResult{
		PreviousState: string(previousState),
		NewState:      string(newState),
		Phase:         nextPhase,
		Detail:        detail,
	}

	slog.InfoContext(ctx, "tool completed", "tool", toolName, "state", string(newState), "duration_ms", time.Since(start).Milliseconds())
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
	snap := s.machine.TakeSnapshot()
	newState, transErr := s.machine.Transition(fsm.EventAbort)
	s.mu.Unlock()

	if transErr != nil {
		// EventAbort is universally accepted; this branch is defensive only.
		slog.InfoContext(ctx, "tool completed", "tool", toolName, "state", string(currentState), "duration_ms", time.Since(start).Milliseconds())
		return mcplib.NewToolResultError(transErr.Error()), nil
	}

	cp, readErr := s.readCheckpointWithErr(ctx)
	if readErr != nil {
		s.mu.Lock()
		s.machine.Rollback(snap)
		s.mu.Unlock()
		return mcplib.NewToolResultError(fmt.Sprintf("failed to read checkpoint: %v", readErr)), nil
	}
	cp.State = string(newState)
	if writeErr := s.writeCheckpoint(ctx, cp); writeErr != nil {
		s.mu.Lock()
		s.machine.Rollback(snap)
		s.mu.Unlock()
		slog.ErrorContext(ctx, "store write error", "tool", toolName, "state", string(newState), "error", writeErr)
		slog.InfoContext(ctx, "tool completed", "tool", toolName, "state", string(newState), "duration_ms", time.Since(start).Milliseconds(), "error", writeErr)
		return mcplib.NewToolResultError(fmt.Sprintf("failed to persist abort state: %v", writeErr)), nil
	}

	result := AbortResult{State: string(newState)}

	s.appendTrace(ctx, store.TraceEvent{
		Kind:        TraceKindIntervention,
		FromState:   string(currentState),
		ToState:     string(newState),
		Event:       string(fsm.EventAbort),
		Reason:      "operator requested abort",
		PRNumber:    cp.PRNumber,
		IssueNumber: cp.IssueNumber,
	})

	slog.InfoContext(ctx, "tool completed", "tool", toolName, "state", string(newState), "duration_ms", time.Since(start).Milliseconds())
	return toolResultJSON(result), nil
}

// MCPServer builds and returns a configured *server.MCPServer with all
// Loom tools registered. Each call returns a fresh MCPServer instance sharing
// the same underlying FSM and store.
func (s *Server) MCPServer() *mcpserver.MCPServer {
	srv := mcpserver.NewMCPServer(
		"loom",
		"0.1.0",
		mcpserver.WithInstructions(s.buildServerInstructions(context.Background())),
		mcpserver.WithHooks(s.capabilityHooks()),
	)

	s.mu.Lock()
	s.emitter = NewTaskEmitter(srv)
	s.elicitationEmitter = NewElicitationEmitter(srv)
	s.mu.Unlock()

	srv.AddTool(
		mcplib.NewTool("loom_next_step",
			mcplib.WithDescription("Returns the current workflow state and the next action the agent should take"),
			mcplib.WithReadOnlyHintAnnotation(false),
			mcplib.WithString("operation_key",
				mcplib.Description("Optional idempotency key used to return a cached result for retried calls"),
			),
		),
		s.handleNextStep,
	)
	srv.AddTool(
		mcplib.NewTool("loom_checkpoint",
			mcplib.WithDescription("Fires an event on the Loom FSM to advance the workflow and persists the new state"),
			mcplib.WithReadOnlyHintAnnotation(false),
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
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithBoolean("poll",
				mcplib.Description("Optional: when true, run CI polling mode and emit task lifecycle notifications"),
			),
			mcplib.WithNumber("pr_number",
				mcplib.Description("Optional: pull request number to poll CI for; defaults to checkpoint PR number"),
			),
			mcplib.WithNumber("poll_interval_seconds",
				mcplib.Description("Optional: polling interval in seconds (default 30)"),
			),
			mcplib.WithNumber("max_polls",
				mcplib.Description("Optional: maximum poll iterations before finishing (for testing and bounded waits)"),
			),
		),
		s.handleHeartbeat,
	)
	srv.AddTool(
		mcplib.NewTool("loom_elicitation_response",
			mcplib.WithDescription("Handles an operator action for an active Loom elicitation prompt"),
			mcplib.WithString("action",
				mcplib.Required(),
				mcplib.Description("Operator action to apply: skip, reassign, or pause_epic"),
			),
		),
		s.handleElicitationResponse,
	)
	srv.AddTool(
		mcplib.NewTool("loom_schedule_epic",
			mcplib.WithDescription("Evaluates the dependency DAG and spawns unblocked stories up to the configured parallelism limit"),
			mcplib.WithReadOnlyHintAnnotation(false),
			mcplib.WithString("operation_key",
				mcplib.Description("Optional idempotency key used to return a cached result for retried calls"),
			),
		),
		s.handleScheduleEpic,
	)
	srv.AddTool(
		mcplib.NewTool("loom_get_state",
			mcplib.WithDescription("Returns the current FSM state and epic phase (read-only)"),
			mcplib.WithReadOnlyHintAnnotation(true),
		),
		s.handleGetState,
	)
	srv.AddTool(
		mcplib.NewTool("loom_abort",
			mcplib.WithDescription("Aborts the current workflow by transitioning the FSM to PAUSED"),
			mcplib.WithReadOnlyHintAnnotation(false),
		),
		s.handleAbort,
	)
	s.registerBackgroundAgentTool(srv)

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

			cp := s.readCheckpoint(context.Background(), "loom://state")

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

	// E9: session trace resources.
	s.addTraceResources(srv)

	return srv
}

// Serve starts the MCP stdio server, blocking until ctx is cancelled or an
// error is encountered on stdin/stdout. It also starts the session monitor
// goroutine (heartbeat + stall detection) before beginning to handle messages.
func (s *Server) Serve(ctx context.Context, stdin io.Reader, stdout io.Writer) error {
	s.openTrace(ctx)
	s.startMonitor(ctx)
	mcpSvr := s.MCPServer()
	stdioSvr := mcpserver.NewStdioServer(mcpSvr)
	serveErr := stdioSvr.Listen(ctx, stdin, stdout)
	outcome := traceOutcomeFromState(s.machine.State())
	s.closeTrace(ctx, outcome)
	return serveErr
}

// traceOutcomeFromState maps the current FSM state to a session trace outcome
// string when the server shuts down.
func traceOutcomeFromState(state fsm.State) string {
	switch state {
	case fsm.StateComplete:
		return TraceOutcomeComplete
	case fsm.StatePaused:
		return TraceOutcomePaused
	default:
		return TraceOutcomeAborted
	}
}

// addTraceResources registers the loom://trace and loom://trace/index MCP
// resources for E9 session traceability.
func (s *Server) addTraceResources(srv *mcpserver.MCPServer) {
	srv.AddResource(
		mcplib.Resource{
			URI:         "loom://trace",
			Name:        "Session Trace",
			Description: "Human-readable Markdown session trace for the current run-loom session",
			MIMEType:    "text/markdown",
		},
		func(_ context.Context, _ mcplib.ReadResourceRequest) ([]mcplib.ResourceContents, error) {
			if s.traceSessionID == "" {
				return []mcplib.ResourceContents{
					mcplib.TextResourceContents{
						URI:      "loom://trace",
						MIMEType: "text/markdown",
						Text:     "No active session trace. Start the server with a trace session ID to enable tracing.",
					},
				}, nil
			}
			ctx := context.Background()
			trace, events, err := s.st.ReadSessionTrace(ctx, s.traceSessionID)
			if err != nil {
				return nil, fmt.Errorf("loom://trace: %w", err)
			}
			text := renderSessionTrace(trace, events)
			return []mcplib.ResourceContents{
				mcplib.TextResourceContents{
					URI:      "loom://trace",
					MIMEType: "text/markdown",
					Text:     text,
				},
			}, nil
		},
	)

	srv.AddResource(
		mcplib.Resource{
			URI:         "loom://trace/index",
			Name:        "Session Trace Index",
			Description: "Index of the most recent run-loom session traces (newest first)",
			MIMEType:    "application/json",
		},
		func(_ context.Context, _ mcplib.ReadResourceRequest) ([]mcplib.ResourceContents, error) {
			ctx := context.Background()
			traces, err := s.st.ListSessionTraces(ctx, 50)
			if err != nil {
				return nil, fmt.Errorf("loom://trace/index: %w", err)
			}
			type entry struct {
				SessionID  string `json:"session_id"`
				LoomVer    string `json:"loom_ver"`
				Repository string `json:"repository"`
				StartedAt  string `json:"started_at"`
				EndedAt    string `json:"ended_at,omitempty"`
				Outcome    string `json:"outcome"`
			}
			entries := make([]entry, 0, len(traces))
			for _, t := range traces {
				e := entry{
					SessionID:  t.SessionID,
					LoomVer:    t.LoomVer,
					Repository: t.Repository,
					StartedAt:  t.StartedAt.UTC().Format(time.RFC3339),
					Outcome:    t.Outcome,
				}
				if !t.EndedAt.IsZero() {
					e.EndedAt = t.EndedAt.UTC().Format(time.RFC3339)
				}
				entries = append(entries, e)
			}
			payload, err := json.Marshal(entries)
			if err != nil {
				return nil, fmt.Errorf("loom://trace/index: %w", err)
			}
			return []mcplib.ResourceContents{
				mcplib.TextResourceContents{
					URI:      "loom://trace/index",
					MIMEType: "application/json",
					Text:     string(payload),
				},
			}, nil
		},
	)
}

// renderSessionTrace renders a SessionTrace and its events as Markdown.
func renderSessionTrace(trace store.SessionTrace, events []store.TraceEvent) string {
	var sb strings.Builder

	sb.WriteString("# Session Trace\n\n")
	sb.WriteString(fmt.Sprintf("**Session ID:** `%s`  \n", trace.SessionID))
	sb.WriteString(fmt.Sprintf("**Loom version:** %s  \n", orDash(trace.LoomVer)))
	sb.WriteString(fmt.Sprintf("**Repository:** %s  \n", orDash(trace.Repository)))
	if !trace.StartedAt.IsZero() {
		sb.WriteString(fmt.Sprintf("**Started:** %s  \n", trace.StartedAt.UTC().Format(time.RFC3339)))
	}
	if !trace.EndedAt.IsZero() {
		sb.WriteString(fmt.Sprintf("**Ended:** %s  \n", trace.EndedAt.UTC().Format(time.RFC3339)))
		dur := trace.EndedAt.Sub(trace.StartedAt).Round(time.Second)
		sb.WriteString(fmt.Sprintf("**Duration:** %s  \n", dur))
	} else {
		sb.WriteString("**Ended:** _(in progress)_  \n")
	}
	sb.WriteString(fmt.Sprintf("**Outcome:** %s  \n", orDash(trace.Outcome)))

	sb.WriteString("\n## Event Ledger\n\n")

	if len(events) == 0 {
		sb.WriteString("_No events recorded yet._\n")
		return sb.String()
	}

	sb.WriteString("| # | Time | Kind | From | To | Event | Reason |\n")
	sb.WriteString("|---|------|------|------|----|-------|--------|\n")
	for _, ev := range events {
		ts := ev.CreatedAt.UTC().Format("2006-01-02T15:04:05Z")
		sb.WriteString(fmt.Sprintf("| %d | %s | %s | %s | %s | %s | %s |\n",
			ev.Seq, ts, ev.Kind,
			orDash(ev.FromState), orDash(ev.ToState),
			orDash(ev.Event), orDash(ev.Reason),
		))
	}

	// Summarise GitHub issue/PR touch points.
	type ghRef struct {
		num  int
		kind string
	}
	seen := make(map[ghRef]bool)
	var ghRefs []ghRef
	for _, ev := range events {
		if ev.PRNumber != 0 {
			r := ghRef{num: ev.PRNumber, kind: "PR"}
			if !seen[r] {
				seen[r] = true
				ghRefs = append(ghRefs, r)
			}
		}
		if ev.IssueNumber != 0 {
			r := ghRef{num: ev.IssueNumber, kind: "Issue"}
			if !seen[r] {
				seen[r] = true
				ghRefs = append(ghRefs, r)
			}
		}
	}
	if len(ghRefs) > 0 {
		sb.WriteString("\n## GitHub Ledger\n\n")
		sb.WriteString("| Kind | Number |\n")
		sb.WriteString("|------|--------|\n")
		for _, r := range ghRefs {
			sb.WriteString(fmt.Sprintf("| %s | #%d |\n", r.kind, r.num))
		}
	}

	return sb.String()
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
