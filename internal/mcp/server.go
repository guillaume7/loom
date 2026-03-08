// Package mcp implements the Loom MCP stdio server.
//
// The Server struct exposes five MCP tools to VS Code Copilot master sessions:
//   - loom_next_step:   returns the current workflow state and the next action
//   - loom_checkpoint:  fires an FSM event and persists the new state
//   - loom_heartbeat:   health-check returning current state and phase
//   - loom_get_state:   read-only view of the current state and phase
//   - loom_abort:       universally transitions the FSM to PAUSED
//
// Construct a Server with NewServer and call Serve to start the stdio loop,
// or call MCPServer to obtain the underlying *server.MCPServer for testing.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/guillaume7/loom/internal/fsm"
	loomgh "github.com/guillaume7/loom/internal/github"
	"github.com/guillaume7/loom/internal/store"
	mcplib "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// FSM is the subset of fsm.Machine that the MCP server requires.
// Using an interface rather than the concrete *fsm.Machine keeps this package
// independently testable.
type FSM interface {
	State() fsm.State
	Transition(event fsm.Event) (fsm.State, error)
}

// Server is the Loom MCP server that exposes workflow tools.
type Server struct {
	mu      sync.RWMutex // guards all access to machine
	machine FSM
	st      store.Store
	gh      loomgh.Client
}

// NewServer constructs a Server with the provided dependencies.
// gh may be nil when GitHub connectivity is not required by the active tools.
func NewServer(machine FSM, st store.Store, gh loomgh.Client) *Server {
	return &Server{
		machine: machine,
		st:      st,
		gh:      gh,
	}
}

// CheckpointRequest is the typed input struct for loom_checkpoint.
// Action is the spec-aligned field name (see US-4.3 acceptance criteria);
// Event is accepted for backward compatibility when Action is absent.
type CheckpointRequest struct {
	Action string `json:"action"`
	Event  string `json:"event"`
}

// NextStepResult is returned by loom_next_step.
type NextStepResult struct {
	State       string `json:"state"`
	Instruction string `json:"instruction"`
}

// CheckpointResult is returned by loom_checkpoint.
type CheckpointResult struct {
	PreviousState string `json:"previous_state"`
	NewState      string `json:"new_state"`
	Phase         int    `json:"phase"`
}

// HeartbeatResult is returned by loom_heartbeat.
type HeartbeatResult struct {
	State string `json:"state"`
	Phase int    `json:"phase"`
}

// GetStateResult is returned by loom_get_state.
type GetStateResult struct {
	State string `json:"state"`
	Phase int    `json:"phase"`
}

// AbortResult is returned by loom_abort.
type AbortResult struct {
	State string `json:"state"`
}

// stateInstruction maps an FSM state to a human-readable next-action string.
func stateInstruction(state fsm.State) string {
	switch state {
	case fsm.StateIdle:
		return "Call loom_checkpoint with action=start to begin the workflow"
	case fsm.StateScanning:
		return "Scan the project for the next unimplemented phase and create a GitHub issue"
	case fsm.StateIssueCreated:
		return "Assign @copilot to the created issue"
	case fsm.StateAwaitingPR:
		return "Poll GitHub for a PR opened by @copilot; call loom_checkpoint with action=pr_opened when found"
	case fsm.StateAwaitingReady:
		return "Wait for the PR to leave draft status; call loom_checkpoint with action=pr_ready when ready"
	case fsm.StateAwaitingCI:
		return "Poll CI check runs; call loom_checkpoint with action=ci_green or action=ci_red"
	case fsm.StateReviewing:
		return "Request a review; call loom_checkpoint with action=review_approved or action=review_changes_requested"
	case fsm.StateDebugging:
		return "Wait for @copilot to push a fix; call loom_checkpoint with action=fix_pushed"
	case fsm.StateAddressingFeedback:
		return "Wait for @copilot to address review feedback; call loom_checkpoint with action=feedback_addressed"
	case fsm.StateMerging:
		return "Merge the approved PR; call loom_checkpoint with action=merged or action=merged_epic_boundary"
	case fsm.StateRefactoring:
		return "Wait for the refactor PR to merge; call loom_checkpoint with action=refactor_merged"
	case fsm.StateComplete:
		return "All phases complete \u2014 workflow finished"
	case fsm.StatePaused:
		return "Workflow paused \u2014 manual intervention required before resuming"
	default:
		return "Unknown state"
	}
}

// toolResultJSON marshals v to a JSON text CallToolResult.
// If marshaling unexpectedly fails, a tool-error result is returned so that
// handlers never panic or propagate a raw Go error.
func toolResultJSON(v interface{}) *mcplib.CallToolResult {
	b, err := json.Marshal(v)
	if err != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("failed to marshal result: %v", err))
	}
	return mcplib.NewToolResultText(string(b))
}

// readCheckpoint reads the store checkpoint, logging any error and falling
// back to a zero Checkpoint value (store errors are non-fatal to tool calls).
func (s *Server) readCheckpoint(ctx context.Context, toolName string) store.Checkpoint {
	cp, err := s.st.ReadCheckpoint(ctx)
	if err != nil {
		slog.InfoContext(ctx, "store read error", "tool", toolName, "error", err)
	}
	return cp
}

func (s *Server) handleNextStep(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	const toolName = "loom_next_step"
	start := time.Now()

	if err := ctx.Err(); err != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("request cancelled: %v", err)), nil
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
	if err := ctx.Err(); err != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("request cancelled: %v", err)), nil
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

	s.mu.Lock()
	previousState := s.machine.State()
	slog.InfoContext(ctx, "tool called", "tool", toolName, "state", string(previousState))
	newState, transErr := s.machine.Transition(fsm.Event(actionStr))
	s.mu.Unlock()

	if transErr != nil {
		slog.InfoContext(ctx, "tool completed", "tool", toolName, "state", string(previousState), "duration_ms", time.Since(start).Milliseconds())
		return mcplib.NewToolResultError(transErr.Error()), nil
	}

	cp := s.readCheckpoint(ctx, toolName)
	if writeErr := s.st.WriteCheckpoint(ctx, store.Checkpoint{State: string(newState), Phase: cp.Phase}); writeErr != nil {
		slog.ErrorContext(ctx, "store write error", "tool", toolName, "state", string(newState), "error", writeErr)
		slog.InfoContext(ctx, "tool completed", "tool", toolName, "state", string(newState), "duration_ms", time.Since(start).Milliseconds(), "error", writeErr)
		return mcplib.NewToolResultError(fmt.Sprintf("failed to persist checkpoint: %v", writeErr)), nil
	}

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

	if err := ctx.Err(); err != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("request cancelled: %v", err)), nil
	}

	s.mu.RLock()
	currentState := s.machine.State()
	s.mu.RUnlock()

	slog.InfoContext(ctx, "tool called", "tool", toolName, "state", string(currentState))

	cp := s.readCheckpoint(ctx, toolName)
	result := HeartbeatResult{
		State: string(currentState),
		Phase: cp.Phase,
	}

	slog.InfoContext(ctx, "tool completed", "tool", toolName, "state", string(currentState), "duration_ms", time.Since(start).Milliseconds())
	return toolResultJSON(result), nil
}

func (s *Server) handleGetState(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	const toolName = "loom_get_state"
	start := time.Now()

	if err := ctx.Err(); err != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("request cancelled: %v", err)), nil
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
	if err := ctx.Err(); err != nil {
		return mcplib.NewToolResultError(fmt.Sprintf("request cancelled: %v", err)), nil
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
	srv := mcpserver.NewMCPServer("loom", "0.1.0")

	srv.AddTool(
		mcplib.NewTool("loom_next_step",
			mcplib.WithDescription("Returns the current workflow state and the next action the agent should take"),
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

	return srv
}

// Serve starts the MCP stdio server, blocking until ctx is cancelled or an
// error is encountered on stdin/stdout.
func (s *Server) Serve(ctx context.Context, stdin io.Reader, stdout io.Writer) error {
	mcpSvr := s.MCPServer()
	stdioSvr := mcpserver.NewStdioServer(mcpSvr)
	return stdioSvr.Listen(ctx, stdin, stdout)
}
