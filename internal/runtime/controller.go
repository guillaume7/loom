package runtime

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/guillaume7/loom/internal/fsm"
	"github.com/guillaume7/loom/internal/store"
)

const (
	ControllerStateIdle      = "idle"
	ControllerStateStarting  = "starting"
	ControllerStateClaimed   = "claimed"
	ControllerStateSleeping  = "sleeping"
	ControllerStateWakeDue   = "wake_due"
	ControllerStateResuming  = "resuming"
	ControllerStatePaused    = "paused"
	ControllerStateComplete  = "complete"
	ControllerStateShutdown  = "shutdown"
	controllerLeaseScopeRun  = "run"
	defaultRunIdentifier     = "default"
	defaultWakeScanLimit     = 100
	DefaultLeaseTTL          = 2 * time.Minute
	DefaultPollInterval      = 30 * time.Second
)

type Clock func() time.Time

type Config struct {
	HolderID     string
	LeaseTTL     time.Duration
	PollInterval time.Duration
	Now          Clock
}

type Lifecycle struct {
	WorkflowState string
	Controller    string
	Reason        string
	DrivenBy      string
	HolderID      string
	LeaseKey      string
	LeaseExpires  time.Time
	NextWakeKind  string
	NextWakeAt    time.Time
	ResumeState   string
}

type Controller struct {
	store store.Store
	cfg   Config
}

func DefaultConfig() Config {
	return Config{
		HolderID:     DefaultHolderID(),
		LeaseTTL:     DefaultLeaseTTL,
		PollInterval: DefaultPollInterval,
		Now:          time.Now,
	}
}

func DefaultHolderID() string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "unknown-host"
	}
	return fmt.Sprintf("%s:%d", hostname, os.Getpid())
}

func NewController(st store.Store, cfg Config) *Controller {
	if cfg.HolderID == "" {
		cfg.HolderID = DefaultHolderID()
	}
	if cfg.LeaseTTL <= 0 {
		cfg.LeaseTTL = DefaultLeaseTTL
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = DefaultPollInterval
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Controller{store: st, cfg: cfg}
}

func LeaseKey(cp store.Checkpoint) string {
	identifier := cp.StoryID
	if identifier == "" {
		identifier = defaultRunIdentifier
	}
	return "run:" + identifier
}

func RunIdentifier(cp store.Checkpoint) string {
	if cp.StoryID != "" {
		return cp.StoryID
	}
	return defaultRunIdentifier
}

func (c *Controller) Snapshot(ctx context.Context) (Lifecycle, error) {
	cp, err := c.store.ReadCheckpoint(ctx)
	if err != nil {
		return Lifecycle{}, err
	}
	return c.snapshotFromCheckpoint(ctx, cp)
}

func (c *Controller) PendingWakes(ctx context.Context) ([]store.WakeSchedule, error) {
	cp, err := c.store.ReadCheckpoint(ctx)
	if err != nil {
		return nil, err
	}
	return c.store.ReadWakeSchedules(ctx, RunIdentifier(cp), defaultWakeScanLimit)
}

func (c *Controller) Start(ctx context.Context) (Lifecycle, error) {
	cp, err := c.store.ReadCheckpoint(ctx)
	if err != nil {
		return Lifecycle{}, err
	}
	return c.startWithCheckpoint(ctx, cp)
}

func (c *Controller) startWithCheckpoint(ctx context.Context, cp store.Checkpoint) (Lifecycle, error) {
	now := c.cfg.Now().UTC()
	leaseKey := LeaseKey(cp)
	lease, leaseActive, err := c.readLease(ctx, leaseKey, now)
	if err != nil {
		return Lifecycle{}, err
	}
	wake, nextWake, err := c.readWakeState(ctx, cp, now, leaseActive)
	if err != nil {
		return Lifecycle{}, err
	}
	if cp.State == string(fsm.StatePaused) {
		return c.snapshotWithFacts(cp, lease, nextWake, Lifecycle{
			Controller:  ControllerStatePaused,
			Reason:      "checkpoint_paused",
			DrivenBy:    "persisted_runtime_state",
			ResumeState: cp.ResumeState,
		}), nil
	}
	if cp.State == string(fsm.StateComplete) {
		return c.snapshotWithFacts(cp, lease, nextWake, Lifecycle{
			Controller: ControllerStateComplete,
			Reason:     "workflow_complete",
			DrivenBy:   "persisted_runtime_state",
		}), nil
	}
	if leaseActive && lease.HolderID != c.cfg.HolderID {
		return c.snapshotWithFacts(cp, lease, nextWake, Lifecycle{
			Controller: ControllerStateSleeping,
			Reason:     "lease_held_by_other_controller",
			DrivenBy:   "persisted_runtime_state",
		}), nil
	}

	lease = store.RuntimeLease{
		LeaseKey:  leaseKey,
		HolderID:  c.cfg.HolderID,
		Scope:     controllerLeaseScopeRun,
		ExpiresAt: now.Add(c.cfg.LeaseTTL),
		CreatedAt: lease.CreatedAt,
		RenewedAt: now,
	}
	if lease.CreatedAt.IsZero() {
		lease.CreatedAt = now
	}
	if err := c.store.UpsertRuntimeLease(ctx, lease); err != nil {
		return Lifecycle{}, err
	}

	if wake.ID != 0 {
		wake.ClaimedAt = now
		if err := c.store.UpsertWakeSchedule(ctx, wake); err != nil {
			return Lifecycle{}, err
		}
		return c.snapshotWithFacts(cp, lease, wake, Lifecycle{
			Controller: ControllerStateWakeDue,
			Reason:     "persisted_wake_due",
			DrivenBy:   "persisted_runtime_state",
		}), nil
	}

	if shouldSleep(cp.State) {
		if nextWake.ID == 0 {
			nextWake = c.newWake(cp, now)
			if err := c.store.UpsertWakeSchedule(ctx, nextWake); err != nil {
				return Lifecycle{}, err
			}
		}
		return c.snapshotWithFacts(cp, lease, nextWake, Lifecycle{
			Controller: ControllerStateSleeping,
			Reason:     "waiting_for_persisted_wake",
			DrivenBy:   "persisted_runtime_state",
		}), nil
	}

	controllerState := ControllerStateStarting
	reason := "no_checkpoint_yet"
	if cp.State != "" && cp.State != string(fsm.StateIdle) {
		controllerState = ControllerStateResuming
		reason = "checkpoint_requires_resume"
	} else if cp.State == string(fsm.StateIdle) {
		controllerState = ControllerStateClaimed
		reason = "checkpoint_ready"
	}

	return c.snapshotWithFacts(cp, lease, nextWake, Lifecycle{
		Controller: controllerState,
		Reason:     reason,
		DrivenBy:   "persisted_runtime_state",
	}), nil
}

func (c *Controller) Shutdown(ctx context.Context) (Lifecycle, error) {
	cp, err := c.store.ReadCheckpoint(ctx)
	if err != nil {
		return Lifecycle{}, err
	}
	now := c.cfg.Now().UTC()
	lease, _, err := c.readLease(ctx, LeaseKey(cp), now)
	if err != nil {
		return Lifecycle{}, err
	}
	if lease.LeaseKey != "" && lease.HolderID == c.cfg.HolderID {
		lease.ExpiresAt = now
		lease.RenewedAt = now
		if err := c.store.UpsertRuntimeLease(ctx, lease); err != nil {
			return Lifecycle{}, err
		}
	}
	nextWake, err := c.nextWake(ctx, cp, now, false)
	if err != nil {
		return Lifecycle{}, err
	}
	return c.snapshotWithFacts(cp, lease, nextWake, Lifecycle{
		Controller: ControllerStateShutdown,
		Reason:     "controller_shutdown_requested",
		DrivenBy:   "persisted_runtime_state",
	}), nil
}

func (c *Controller) snapshotFromCheckpoint(ctx context.Context, cp store.Checkpoint) (Lifecycle, error) {
	now := c.cfg.Now().UTC()
	lease, leaseActive, err := c.readLease(ctx, LeaseKey(cp), now)
	if err != nil {
		return Lifecycle{}, err
	}
	nextWake, err := c.nextWake(ctx, cp, now, leaseActive)
	if err != nil {
		return Lifecycle{}, err
	}
	base := Lifecycle{
		Controller: ControllerStateIdle,
		Reason:     "no_checkpoint_yet",
		DrivenBy:   "persisted_runtime_state",
	}
	switch {
	case cp.State == string(fsm.StatePaused):
		base.Controller = ControllerStatePaused
		base.Reason = "checkpoint_paused"
		base.ResumeState = cp.ResumeState
	case cp.State == string(fsm.StateComplete):
		base.Controller = ControllerStateComplete
		base.Reason = "workflow_complete"
	case !lease.ExpiresAt.IsZero() && lease.ExpiresAt.After(now):
		base.Controller = ControllerStateClaimed
		base.Reason = "active_lease_present"
	case nextWake.ID != 0 && nextWake.DueAt.After(now):
		base.Controller = ControllerStateSleeping
		base.Reason = "awaiting_next_wake"
	case nextWake.ID != 0 && !nextWake.DueAt.After(now):
		base.Controller = ControllerStateWakeDue
		base.Reason = "persisted_wake_due"
	case cp.State != "" && cp.State != string(fsm.StateIdle):
		base.Controller = ControllerStateResuming
		base.Reason = "checkpoint_requires_resume"
	case cp.State == string(fsm.StateIdle):
		base.Controller = ControllerStateClaimed
		base.Reason = "checkpoint_ready"
	}
	return c.snapshotWithFacts(cp, lease, nextWake, base), nil
}

func (c *Controller) snapshotWithFacts(cp store.Checkpoint, lease store.RuntimeLease, wake store.WakeSchedule, lifecycle Lifecycle) Lifecycle {
	lifecycle.WorkflowState = cp.State
	lifecycle.HolderID = lease.HolderID
	lifecycle.LeaseKey = lease.LeaseKey
	lifecycle.LeaseExpires = lease.ExpiresAt.UTC()
	lifecycle.NextWakeKind = wake.WakeKind
	lifecycle.NextWakeAt = wake.DueAt.UTC()
	if lifecycle.ResumeState == "" {
		lifecycle.ResumeState = cp.ResumeState
	}
	return lifecycle
}

func (c *Controller) readWakeState(ctx context.Context, cp store.Checkpoint, now time.Time, leaseActive bool) (store.WakeSchedule, store.WakeSchedule, error) {
	nextWake, err := c.nextWake(ctx, cp, now, leaseActive)
	if err != nil {
		return store.WakeSchedule{}, store.WakeSchedule{}, err
	}
	if nextWake.ID != 0 && !nextWake.DueAt.After(now) {
		return nextWake, nextWake, nil
	}
	return store.WakeSchedule{}, nextWake, nil
}

func (c *Controller) nextWake(ctx context.Context, cp store.Checkpoint, now time.Time, leaseActive bool) (store.WakeSchedule, error) {
	wakes, err := c.store.ReadWakeSchedules(ctx, RunIdentifier(cp), defaultWakeScanLimit)
	if err != nil {
		return store.WakeSchedule{}, err
	}
	var next store.WakeSchedule
	for _, wake := range wakes {
		if !c.isWakeVisible(wake, now, leaseActive) {
			continue
		}
		if next.ID == 0 || wake.DueAt.Before(next.DueAt) {
			next = wake
		}
	}
	return next, nil
}

func (c *Controller) isWakeVisible(wake store.WakeSchedule, now time.Time, leaseActive bool) bool {
	if wake.ClaimedAt.IsZero() || wake.ClaimedAt.After(now) {
		return true
	}
	return !leaseActive
}

func (c *Controller) readLease(ctx context.Context, leaseKey string, now time.Time) (store.RuntimeLease, bool, error) {
	lease, err := c.store.ReadRuntimeLease(ctx, leaseKey)
	if err != nil {
		if err == store.ErrRuntimeLeaseNotFound {
			return store.RuntimeLease{}, false, nil
		}
		return store.RuntimeLease{}, false, err
	}
	return lease, lease.ExpiresAt.After(now), nil
}

func (c *Controller) newWake(cp store.Checkpoint, now time.Time) store.WakeSchedule {
	wakeKind := inferWakeKind(cp.State)
	return store.WakeSchedule{
		SessionID: RunIdentifier(cp),
		WakeKind:  wakeKind,
		DueAt:     now.Add(c.cfg.PollInterval),
		DedupeKey: fmt.Sprintf("%s:%s", LeaseKey(cp), wakeKind),
	}
}

func shouldSleep(state string) bool {
	return inferWakeKind(state) != ""
}

func inferWakeKind(state string) string {
	switch fsm.State(state) {
	case fsm.StateAwaitingPR:
		return "poll_pr"
	case fsm.StateAwaitingReady:
		return "poll_pr_ready"
	case fsm.StateAwaitingCI:
		return "poll_ci"
	case fsm.StateReviewing:
		return "poll_review"
	case fsm.StateDebugging:
		return "poll_fix"
	case fsm.StateAddressingFeedback:
		return "poll_feedback"
	case fsm.StateRefactoring:
		return "poll_refactor"
	default:
		return ""
	}
}