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
	loomruntime "github.com/guillaume7/loom/internal/runtime"
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
		cached, found, lookupErr := s.readActionByOperationKey(ctx, toolName, operationKey)
		if lookupErr != nil {
			return lookupErr, nil
		}
		if found {
			slog.InfoContext(ctx, "tool completed", "tool", toolName, "operation_key", operationKey, "cached", true, "duration_ms", time.Since(start).Milliseconds())
			return mcplib.NewToolResultText(cached.Detail), nil
		}

		_, currentState, syncErr := s.syncMachineToCheckpoint(ctx, toolName)
		if syncErr != nil {
			return mcplib.NewToolResultError(syncErr.Error()), nil
		}
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

	_, currentState, syncErr := s.syncMachineToCheckpoint(ctx, toolName)
	if syncErr != nil {
		return mcplib.NewToolResultError(syncErr.Error()), nil
	}

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
	prNumber, hasPRNumber, err := optionalIntArgument(req, "pr_number")
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	issueNumber, hasIssueNumber, err := optionalIntArgument(req, "issue_number")
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}

	if hasOperationKey {
		_, _, syncErr := s.syncMachineToCheckpoint(ctx, toolName)
		if syncErr != nil {
			return mcplib.NewToolResultError(syncErr.Error()), nil
		}

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
		nextPRNumber := cp.PRNumber
		if hasPRNumber {
			nextPRNumber = prNumber
		}
		nextIssueNumber := cp.IssueNumber
		if hasIssueNumber {
			nextIssueNumber = issueNumber
		}
		if requestErr := s.requestReviewIfEnteringReviewing(ctx, previousState, newState, nextPRNumber); requestErr != nil {
			s.machine.Rollback(snap)
			return mcplib.NewToolResultError(requestErr.Error()), nil
		}
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
		if newState == fsm.StatePaused {
			nextCheckpoint.ResumeState = string(previousState)
		} else {
			nextCheckpoint.ResumeState = ""
		}
		nextCheckpoint.Phase = nextPhase
		nextCheckpoint.PRNumber = nextPRNumber
		nextCheckpoint.IssueNumber = nextIssueNumber
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
		} else {
			s.activeElicitation = elicitationContext{}
		}

		slog.InfoContext(ctx, "tool completed", "tool", toolName, "state", string(newState), "duration_ms", time.Since(start).Milliseconds())
		return mcplib.NewToolResultText(detail), nil
	}

	_, _, syncErr := s.syncMachineToCheckpoint(ctx, toolName)
	if syncErr != nil {
		return mcplib.NewToolResultError(syncErr.Error()), nil
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
	nextPRNumber := cp.PRNumber
	if hasPRNumber {
		nextPRNumber = prNumber
	}
	nextIssueNumber := cp.IssueNumber
	if hasIssueNumber {
		nextIssueNumber = issueNumber
	}
	if requestErr := s.requestReviewIfEnteringReviewing(ctx, previousState, newState, nextPRNumber); requestErr != nil {
		s.machine.Rollback(snap)
		s.mu.Unlock()
		return mcplib.NewToolResultError(requestErr.Error()), nil
	}
	detail := ""
	if event == fsm.EventSkipStory {
		nextPhase = cp.Phase + 1
		nextRetryCount = 0
		detail = fmt.Sprintf("skipped story at phase %d", cp.Phase)
	}

	nextCheckpoint := cp
	nextCheckpoint.State = string(newState)
	if newState == fsm.StatePaused {
		nextCheckpoint.ResumeState = string(previousState)
	} else {
		nextCheckpoint.ResumeState = ""
	}
	nextCheckpoint.Phase = nextPhase
	nextCheckpoint.PRNumber = nextPRNumber
	nextCheckpoint.IssueNumber = nextIssueNumber
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
	} else {
		s.mu.Lock()
		s.activeElicitation = elicitationContext{}
		s.mu.Unlock()
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

func (s *Server) requestReviewIfEnteringReviewing(ctx context.Context, previousState, newState fsm.State, prNumber int) error {
	if previousState != fsm.StateAwaitingCI || newState != fsm.StateReviewing {
		return nil
	}
	if prNumber <= 0 {
		return nil
	}
	reviewer, ok := s.gh.(reviewRequestingClient)
	if !ok {
		return nil
	}
	if err := reviewer.RequestReview(ctx, prNumber, copilotReviewer); err != nil {
		return fmt.Errorf("failed to request Copilot review for PR #%d: %w", prNumber, err)
	}
	return nil
}

func (s *Server) handleGetState(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	const toolName = "loom_get_state"
	start := time.Now()

	if res, cancelled := checkCtx(ctx, toolName); cancelled {
		return res, nil
	}

	cp, currentState, syncErr := s.syncMachineToCheckpoint(ctx, toolName)
	if syncErr != nil {
		return mcplib.NewToolResultError(syncErr.Error()), nil
	}
	lifecycle, err := s.controllerLifecycle(ctx)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	wakes, err := s.pendingWakes(ctx)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}

	slog.InfoContext(ctx, "tool called", "tool", toolName, "state", string(currentState))

	result := GetStateResult{
		State:            string(currentState),
		Phase:            cp.Phase,
		ControllerState:  lifecycle.Controller,
		ControllerReason: lifecycle.Reason,
		ControllerHolder: lifecycle.HolderID,
		ControllerLease:  lifecycle.LeaseKey,
		LeaseExpiresAt:   formatLifecycleTime(lifecycle.LeaseExpires),
		NextWakeKind:     lifecycle.NextWakeKind,
		NextWakeAt:       formatLifecycleTime(lifecycle.NextWakeAt),
		PendingWakes:     buildWakeDiagnostics(wakes),
		ResumeState:      lifecycle.ResumeState,
		DrivenBy:         lifecycle.DrivenBy,
	}

	slog.InfoContext(ctx, "tool completed", "tool", toolName, "state", string(currentState), "duration_ms", time.Since(start).Milliseconds())
	return toolResultJSON(result), nil
}

func (s *Server) handleAbort(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	const toolName = "loom_abort"
	start := time.Now()
	_ = req

	// Guard against cancelled context before acquiring any locks or mutating state.
	if res, cancelled := checkCtx(ctx, toolName); cancelled {
		return res, nil
	}

	_, _, syncErr := s.syncMachineToCheckpoint(ctx, toolName)
	if syncErr != nil {
		return mcplib.NewToolResultError(syncErr.Error()), nil
	}

	s.mu.Lock()
	currentState := s.machine.State()
	s.mu.Unlock()
	slog.InfoContext(ctx, "tool called", "tool", toolName, "state", string(currentState))

	controller := loomruntime.NewController(s.runtimeStore(), loomruntime.DefaultConfig())
	lifecycle, err := controller.ApplyManualOverride(ctx, loomruntime.ManualOverrideRequest{
		Action:      loomruntime.ManualOverridePause,
		Source:      "mcp",
		RequestedBy: sessionIDFromContext(ctx),
		Reason:      "paused via loom_abort",
	})
	if err != nil {
		slog.InfoContext(ctx, "tool completed", "tool", toolName, "state", string(currentState), "duration_ms", time.Since(start).Milliseconds(), "error", err)
		return mcplib.NewToolResultError(err.Error()), nil
	}

	_, newState, syncErr := s.syncMachineToCheckpoint(ctx, toolName)
	if syncErr != nil {
		slog.InfoContext(ctx, "tool completed", "tool", toolName, "state", lifecycle.WorkflowState, "duration_ms", time.Since(start).Milliseconds(), "error", syncErr)
		return mcplib.NewToolResultError(syncErr.Error()), nil
	}

	result := AbortResult{State: string(newState)}

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
			mcplib.WithDescription("Persists a workflow checkpoint after applying an FSM event"),
			mcplib.WithReadOnlyHintAnnotation(false),
			mcplib.WithString("action",
				mcplib.Description("FSM event to apply, for example start, pr_opened, ci_green, or merged"),
			),
			mcplib.WithString("event",
				mcplib.Description("Deprecated alias for action, accepted for backward compatibility"),
			),
			mcplib.WithString("operation_key",
				mcplib.Description("Optional idempotency key used to return a cached result for retried calls"),
			),
			mcplib.WithNumber("pr_number",
				mcplib.Description("Optional pull request number to persist with the checkpoint"),
			),
			mcplib.WithNumber("issue_number",
				mcplib.Description("Optional issue number to persist with the checkpoint"),
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
		func(ctx context.Context, _ mcplib.ReadResourceRequest) ([]mcplib.ResourceContents, error) {
			cp, currentState, err := s.syncMachineToCheckpoint(ctx, "loom://state")
			if err != nil {
				return nil, err
			}

			lifecycle, err := s.controllerLifecycle(ctx)
			if err != nil {
				return nil, err
			}

			wakes, err := s.pendingWakes(ctx)
			if err != nil {
				return nil, err
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
				ControllerState  string   `json:"controller_state,omitempty"`
				ControllerReason string   `json:"controller_reason,omitempty"`
				ControllerHolder string   `json:"controller_holder,omitempty"`
				ControllerLease  string   `json:"controller_lease,omitempty"`
				LeaseExpiresAt   string   `json:"lease_expires_at,omitempty"`
				NextWakeKind     string   `json:"next_wake_kind,omitempty"`
				NextWakeAt       string   `json:"next_wake_at,omitempty"`
				PendingWakes     []WakeDiagnostic `json:"pending_wakes"`
				ResumeState      string   `json:"resume_state,omitempty"`
				DrivenBy         string   `json:"driven_by,omitempty"`
			}{
				State:            string(currentState),
				Phase:            cp.Phase,
				PRNumber:         cp.PRNumber,
				IssueNumber:      cp.IssueNumber,
				RetryCount:       cp.RetryCount,
				UpdatedAt:        updatedAt,
				UnblockedStories: unblockedStories,
				ControllerState:  lifecycle.Controller,
				ControllerReason: lifecycle.Reason,
				ControllerHolder: lifecycle.HolderID,
				ControllerLease:  lifecycle.LeaseKey,
				LeaseExpiresAt:   formatLifecycleTime(lifecycle.LeaseExpires),
				NextWakeKind:     lifecycle.NextWakeKind,
				NextWakeAt:       formatLifecycleTime(lifecycle.NextWakeAt),
				PendingWakes:     buildWakeDiagnostics(wakes),
				ResumeState:      lifecycle.ResumeState,
				DrivenBy:         lifecycle.DrivenBy,
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
