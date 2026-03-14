// Package mcp implements the Loom MCP stdio server.
//
// The Server struct exposes MCP tools to VS Code Copilot master sessions:
//   - loom_next_step:   returns the current workflow state and the next action
//   - loom_checkpoint:  fires an FSM event and persists the new state
//   - loom_heartbeat:   health-check returning current state, phase, and wait guidance
//   - loom_get_state:   read-only view of the current state and phase
//   - loom_abort:       universally transitions the FSM to PAUSED
//   - loom_elicitation_response: handles operator response when elicitation is active
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

// resourceEntry holds a single registered MCP resource and its handler.
type resourceEntry struct {
	resource mcplib.Resource
	handler  mcpserver.ResourceHandlerFunc
}

type elicitationContext struct {
	Active   bool
	PRNumber int
	State    fsm.State
	Event    fsm.Event
}

// Server is the Loom MCP server that exposes workflow tools.
type Server struct {
	mu                 sync.RWMutex // guards all access to machine and lastActivity
	machine            FSM
	st                 store.Store
	gh                 loomgh.Client
	emitter            *TaskEmitter
	elicitationEmitter *ElicitationEmitter

	// Session-scoped cache populated during initialize capability negotiation.
	sessionTaskSupport        map[string]bool
	sessionElicitationSupport map[string]bool
	activeElicitation         elicitationContext

	// Session management (E6).
	clock        Clock
	monCfg       MonitorConfig
	lastActivity time.Time // wall-clock time of the last loom_checkpoint call

	// Resources registered via AddResource (E3).
	resources []resourceEntry
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
		machine:                   machine,
		st:                        st,
		gh:                        gh,
		clock:                     RealClock,
		monCfg:                    DefaultMonitorConfig(),
		sessionTaskSupport:        make(map[string]bool),
		sessionElicitationSupport: make(map[string]bool),
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

// AddResource registers an MCP resource and its handler on the Server.
// Resources registered here are applied to every MCPServer built by MCPServer().
func (s *Server) AddResource(resource mcplib.Resource, handler mcpserver.ResourceHandlerFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resources = append(s.resources, resourceEntry{resource: resource, handler: handler})
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
	Detail        string `json:"detail,omitempty"`
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

func (s *Server) setSessionTaskSupport(sessionID string, supported bool) {
	if sessionID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessionTaskSupport[sessionID] = supported
}

func (s *Server) setSessionElicitationSupport(sessionID string, supported bool) {
	if sessionID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessionElicitationSupport[sessionID] = supported
}

func (s *Server) sessionSupportsTasks(ctx context.Context) bool {
	sessionID := sessionIDFromContext(ctx)
	if sessionID == "" {
		return false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessionTaskSupport[sessionID]
}

func (s *Server) sessionSupportsElicitation(ctx context.Context) bool {
	sessionID := sessionIDFromContext(ctx)
	if sessionID == "" {
		return false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessionElicitationSupport[sessionID]
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
