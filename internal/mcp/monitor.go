// Package mcp — session management (E6).
//
// This file provides:
//   - Clock interface for injectable wall-clock time (enables fake-clock tests)
//   - MonitorConfig with production-spec defaults
//   - isGateState helper identifying states that require keep-alive
//   - RunStallCheck: exported stall-detection method, callable directly in tests
//   - startMonitor: background goroutine emitting heartbeat logs and running stall checks
package mcp

import (
	"context"
	"log/slog"
	"time"

	"github.com/guillaume7/loom/internal/fsm"
	"github.com/guillaume7/loom/internal/store"
)

// Clock provides the current time. Inject a fake implementation in tests
// to avoid time.Sleep calls in stall-detection logic.
type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// RealClock is the production wall-clock implementation of Clock.
var RealClock Clock = realClock{}

// MonitorConfig holds tuning parameters for session management.
type MonitorConfig struct {
	// StallTimeout is the maximum duration without a loom_checkpoint call
	// before a stall is declared and the workflow is paused.
	// Default: 5 minutes.
	StallTimeout time.Duration

	// HeartbeatInterval is how often a heartbeat log entry is emitted while
	// in a gate state. Default: 60 seconds.
	HeartbeatInterval time.Duration

	// TickInterval controls how often the monitor goroutine wakes to check
	// for stalls and emit heartbeats. Default: 10 seconds.
	TickInterval time.Duration
}

// DefaultMonitorConfig returns a MonitorConfig with production-spec defaults.
func DefaultMonitorConfig() MonitorConfig {
	return MonitorConfig{
		StallTimeout:      5 * time.Minute,
		HeartbeatInterval: 60 * time.Second,
		TickInterval:      10 * time.Second,
	}
}

// gateStates is the set of FSM states where the workflow is waiting for an
// external event and the Copilot session must be kept alive via heartbeats.
var gateStates = map[fsm.State]bool{
	fsm.StateAwaitingPR:         true,
	fsm.StateAwaitingReady:      true,
	fsm.StateAwaitingCI:         true,
	fsm.StateDebugging:          true,
	fsm.StateAddressingFeedback: true,
	fsm.StateRefactoring:        true,
}

// isGateState reports whether s is a gate state — one where the session is
// expected to wait for an extended period and heartbeats must be emitted.
func isGateState(s fsm.State) bool {
	return gateStates[s]
}

// retryInSeconds is the fixed retry delay returned by loom_heartbeat when the
// machine is in a gate state (US-6.5).
const retryInSeconds = 30

// RunStallCheck inspects the current machine state and last-activity timestamp.
// If the machine is in a gate state and no loom_checkpoint has been received
// for longer than MonitorConfig.StallTimeout, RunStallCheck transitions the
// machine to PAUSED, persists the checkpoint, and logs the stall event.
// It returns true when a stall is detected, false otherwise.
//
// RunStallCheck is exported so that tests can exercise stall-detection logic
// directly without relying on the background goroutine (no time.Sleep needed).
func (s *Server) RunStallCheck(ctx context.Context) bool {
	s.mu.RLock()
	currentState := s.machine.State()
	lastAct := s.lastActivity
	s.mu.RUnlock()

	if !isGateState(currentState) {
		return false
	}

	elapsed := s.clock.Now().Sub(lastAct)
	if elapsed < s.monCfg.StallTimeout {
		return false
	}

	// Stall detected: log, transition to PAUSED, persist checkpoint.
	// Re-check lastActivity under the write lock to close the TOCTOU window:
	// handleCheckpoint may have updated lastActivity between our RLock read
	// above and the Lock acquisition below.
	slog.WarnContext(ctx, "session stall detected",
		"state", string(currentState),
		"elapsed", elapsed,
		"stall_timeout", s.monCfg.StallTimeout,
	)

	s.mu.Lock()
	// Re-verify that no checkpoint arrived while we were waiting for the lock.
	if s.clock.Now().Sub(s.lastActivity) < s.monCfg.StallTimeout {
		s.mu.Unlock()
		return false
	}
	_, _ = s.machine.Transition(fsm.EventAbort)
	s.mu.Unlock()

	cp := s.readCheckpoint(ctx, "stall_check")
	if err := s.st.WriteCheckpoint(ctx, store.Checkpoint{
		State: string(fsm.StatePaused),
		Phase: cp.Phase,
	}); err != nil {
		slog.ErrorContext(ctx, "failed to persist PAUSED checkpoint on stall", "error", err)
	} else {
		slog.WarnContext(ctx, "stall recovery: state written to PAUSED",
			"previous_state", string(currentState),
			"elapsed", elapsed,
		)
	}

	return true
}

// startMonitor launches a background goroutine that:
//   - emits a heartbeat log entry at MonitorConfig.HeartbeatInterval while in a gate state
//   - calls RunStallCheck at every MonitorConfig.TickInterval
//
// The goroutine exits when ctx is cancelled.
func (s *Server) startMonitor(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(s.monCfg.TickInterval)
		defer ticker.Stop()

		lastHeartbeat := s.clock.Now()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.mu.RLock()
				currentState := s.machine.State()
				s.mu.RUnlock()

				if !isGateState(currentState) {
					continue
				}

				now := s.clock.Now()

				// Emit a heartbeat log entry when the interval has elapsed.
				if now.Sub(lastHeartbeat) >= s.monCfg.HeartbeatInterval {
					slog.InfoContext(ctx, "heartbeat", "state", string(currentState))
					lastHeartbeat = now
				}

				// Check for stall.
				s.RunStallCheck(ctx)
			}
		}
	}()
}
