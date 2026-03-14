// Package mcp implements the Loom MCP stdio server.
//
// The Server struct exposes five MCP tools to VS Code Copilot master sessions:
//   - loom_next_step:   returns the current workflow state and the next action
//   - loom_checkpoint:  fires an FSM event and persists the new state
//   - loom_heartbeat:   health-check returning current state, phase, and wait guidance
//   - loom_get_state:   read-only view of the current state and phase
//   - loom_abort:       universally transitions the FSM to PAUSED
//
// Session management (E6) is handled by monitor.go: Clock interface,
// MonitorConfig, heartbeat log emission, and stall detection via RunStallCheck.
//
// Construct a Server with NewServer and call Serve to start the stdio loop,
// or call MCPServer to obtain the underlying *server.MCPServer for testing.
package mcp

import (
	"context"
	"encoding/json"
	"errors"
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
	// TakeSnapshot returns an opaque snapshot of all mutable machine state.
	// The snapshot must be passed to Rollback if the operation that follows
	// the Transition call fails and the in-memory state must be undone.
	TakeSnapshot() fsm.Snapshot
	// Rollback restores the machine to the state captured by TakeSnapshot.
	Rollback(snap fsm.Snapshot)
}

// Server is the Loom MCP server that exposes workflow tools.
type Server struct {
	mu      sync.RWMutex // guards all access to machine and lastActivity
	machine FSM
	st      store.Store
	gh      loomgh.Client

	// Session management (E6).
	clock        Clock
	monCfg       MonitorConfig
	lastActivity time.Time // wall-clock time of the last loom_checkpoint call
}

// Option configures a Server.
type Option func(*Server)

// WithClock sets the Clock implementation. Inject a fake clock in tests to
// avoid time.Sleep calls in stall-detection logic.
func WithClock(c Clock) Option { return func(s *Server) { s.clock = c } }

// WithMonitorConfig overrides the default MonitorConfig.
func WithMonitorConfig(cfg MonitorConfig) Option { return func(s *Server) { s.monCfg = cfg } }

// NewServer constructs a Server with the provided dependencies.
// gh may be nil when GitHub connectivity is not required by the active tools.
// Optional Option values can be passed to override the clock or monitor config.
func NewServer(machine FSM, st store.Store, gh loomgh.Client, opts ...Option) *Server {
	s := &Server{
		machine: machine,
		st:      st,
		gh:      gh,
		clock:   RealClock,
		monCfg:  DefaultMonitorConfig(),
	}
	for _, opt := range opts {
		opt(s)
	}
	// Initialize lastActivity to now so a fresh server is not immediately stale.
	s.lastActivity = s.clock.Now()
	return s
}

// Store returns the underlying store.Store. Exposed for use in tests that need
// to read the checkpoint state after a RunStallCheck call.
func (s *Server) Store() store.Store { return s.st }

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
	State          string `json:"state"`
	Phase          int    `json:"phase"`
	Wait           bool   `json:"wait"`
	RetryInSeconds int    `json:"retry_in_seconds,omitempty"`
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

// checkCtx checks whether the context has been cancelled. If so, it returns
// a tool-error result and true; otherwise it returns nil and false.
// Callers should return immediately when the second return value is true.
func checkCtx(ctx context.Context, toolName string) (*mcplib.CallToolResult, bool) {
	if err := ctx.Err(); err != nil {
		slog.WarnContext(ctx, "request cancelled", "tool", toolName, "error", err)
		return mcplib.NewToolResultError(fmt.Sprintf("request cancelled: %v", err)), true
	}
	return nil, false
}

func marshalResultText(v interface{}) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func optionalStringArgument(req mcplib.CallToolRequest, name string) (string, bool, error) {
	v, ok := req.Params.Arguments[name]
	if !ok {
		return "", false, nil
	}
	s, ok := v.(string)
	if !ok || s == "" {
		return "", false, fmt.Errorf("missing or invalid '%s' argument: must be a non-empty string", name)
	}
	return s, true, nil
}

func sessionIDFromContext(ctx context.Context) string {
	session := mcpserver.ClientSessionFromContext(ctx)
	if session == nil {
		return ""
	}
	return session.SessionID()
}

func (s *Server) readActionByOperationKey(ctx context.Context, toolName, operationKey string) (store.Action, bool, *mcplib.CallToolResult) {
	action, err := s.st.ReadActionByOperationKey(ctx, operationKey)
	if errors.Is(err, store.ErrActionNotFound) {
		return store.Action{}, false, nil
	}
	if err != nil {
		slog.ErrorContext(ctx, "action log lookup error", "tool", toolName, "operation_key", operationKey, "error", err)
		return store.Action{}, false, mcplib.NewToolResultError(fmt.Sprintf("failed to read action log: %v", err))
	}
	return action, true, nil
}

func (s *Server) writeActionOrReturnCached(ctx context.Context, toolName string, action store.Action) (*mcplib.CallToolResult, bool) {
	err := s.st.WriteAction(ctx, action)
	if err == nil {
		return nil, false
	}
	if !errors.Is(err, store.ErrDuplicateOperationKey) {
		slog.ErrorContext(ctx, "action log write error", "tool", toolName, "operation_key", action.OperationKey, "error", err)
		return mcplib.NewToolResultError(fmt.Sprintf("failed to write action log: %v", err)), true
	}

	cached, found, lookupErr := s.readActionByOperationKey(ctx, toolName, action.OperationKey)
	if lookupErr != nil {
		return lookupErr, true
	}
	if !found {
		return mcplib.NewToolResultError(fmt.Sprintf("duplicate operation key without cached result: %s", action.OperationKey)), true
	}

	slog.InfoContext(ctx, "returning cached tool result", "tool", toolName, "operation_key", action.OperationKey)
	return mcplib.NewToolResultText(cached.Detail), true
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
	srv := mcpserver.NewMCPServer("loom", "0.1.0")

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
