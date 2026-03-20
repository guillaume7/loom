# TH3.E1.US1 Runtime Mode Decision

> Story: [TH3.E1.US1](./user-stories/US1-runtime-mode-decision-spike.md)
> Inputs: [VP3 runtime-first vision](../../../../vision_of_product/VP3-runtime-first/03-vision-runtime-first.md), [ADR-008](../../../../ADRs/ADR-008-runtime-first-control-plane-and-wake-model.md), [deployment architecture](../../../../architecture/deployment.md), [tech stack](../../../../architecture/tech-stack.md)

## Decision Summary

TH3 selects the **resumable runner** as the baseline runtime mode.

The resumable runner best matches VP3's minimum contract: the Go runtime owns
checkpoint truth, wake-up intent, retries, and recovery without making a
long-lived local daemon mandatory for development, debugging, or migration.

The **local daemon** remains a valid future additive mode for always-on local
operation. A **hybrid baseline** is rejected for initial TH3 delivery because it
would force Loom to stabilize two operator models before the runtime contract,
lock model, and recovery semantics are proven.

## VP3 Constraints Used For Comparison

| Constraint | Why it matters in VP3 |
| ---------- | --------------------- |
| Local-first operation | Loom must run on a developer machine without hosted infrastructure or mandatory always-on services. |
| Durable progress without session liveness | Waits, retries, and wake-ups cannot depend on an editor tab or chat session surviving. |
| Deterministic and recoverable runtime state | The next safe action must come from persisted runtime state, not prompt reconstruction. |
| Safe pause and manual override | Operators must be able to inspect, pause, resume, and override without hidden side effects. |
| Incremental migration of existing surfaces | CLI, MCP, and operator workflows must stay viable while TH3 replaces session-centric liveness. |

## Runtime Mode Comparison

| Mode | Strengths against VP3 constraints | Weaknesses against VP3 constraints | Verdict |
| ---- | -------------------------------- | --------------------------------- | ------- |
| Resumable runner | Preserves local-first delivery, keeps a single-binary mental model, centers state recovery in SQLite-backed runtime records, and lets TH3 harden pause/resume semantics before adding another operating form. It fits ADR-008's minimum execution mode directly. | Continuous background progression depends on the active Loom process or a later re-invocation path, so always-on convenience is weaker than a daemon on day one. | **Selected baseline** |
| Local daemon | Strongest fit for unattended always-on wake-ups on one machine and reduces operator need to relaunch Loom between waits. | Makes daemon lifecycle, supervision, health reporting, and local debug ergonomics part of the minimum TH3 contract too early. That raises migration and support complexity before lock/recovery behavior is settled. | Rejected as baseline; keep as additive future mode |
| Hybrid baseline (runner + daemon as equal first-class modes) | Offers flexibility and a possible long-term product shape if both modes eventually exist. | Fragments implementation and testing immediately: two lifecycle models, two operator expectations, and more ambiguity about which path is authoritative during migration. It increases the chance that CLI, MCP, and policy work are built around incompatible assumptions. | Rejected for TH3 baseline |

## Recommended Baseline

Choose the **resumable runner** as the TH3 baseline runtime mode.

### Rationale

1. It satisfies ADR-008's explicit minimum: a resumable runner owned by the Go runtime, with daemonization as an optional additive variant.
2. It is the smallest change that still moves liveness out of interactive sessions and into persisted runtime state.
3. It keeps local development and debugging simple: start Loom, inspect persisted state, stop Loom, and resume without needing a separate service manager.
4. It reduces migration risk for TH3.E1 through TH3.E4 by letting wake records, policy decisions, leases, and recovery semantics stabilize before introducing an always-on wrapper.
5. It preserves a clear future path: once the runner contract is trustworthy, a daemon can wrap the same checkpoint, wake, and lock model without redefining source-of-truth ownership.

## Rejected Alternatives

### Local daemon as the baseline

Rejected because it front-loads operational complexity that VP3 does not need to prove first. A daemon-first baseline would require Loom to solve process supervision, foreground/background observability, and daemon-specific troubleshooting at the same time as core runtime correctness.

### Hybrid baseline

Rejected because it creates ambiguity too early. If both runner and daemon are treated as equal primary modes from the start, future implementation stories can drift into different assumptions about wake ownership, operator commands, and test coverage.

## Impact On Existing Workflows

### CLI impact

- `loom start` remains the primary operator entry point and becomes the baseline runtime controller launch path.
- `loom status`, `loom pause`, `loom resume`, and `loom log` must report and act on persisted runtime state, wake intent, and lock ownership rather than session-heartbeat assumptions.
- No mandatory `loom daemon` command is required for TH3 baseline delivery. If added later, it should wrap the same runtime contract rather than introduce new state semantics.

### MCP impact

- MCP remains an operator and agent integration surface, not the owner of liveness.
- `loom_get_state` and related resources should expose runtime-owned facts such as checkpoint state, next wake-up, policy outcomes, and active leases.
- Long-running MCP task behavior becomes optional observability or integration support, not the primary mechanism that keeps waiting states alive.

### Operator workflow impact

- Operators continue to use CLI and MCP to inspect and control Loom, but the thing they are inspecting is now the runtime's persisted state rather than the health of a long-lived agent session.
- Pause, resume, and override remain explicit human actions with auditable effects.
- Local debugging stays simple because the baseline does not require a background service to be installed, supervised, or cleaned up before the runtime model is trusted.

## Follow-Up Risks And Gaps

- The resumable runner baseline still needs a concrete unattended wake strategy in later stories: either a controller that stays alive locally, a daemon wrapper, or a safe re-invocation path driven by persisted wake records.
- TH3.E1.US3 must define controller lifecycle expectations clearly enough that a future daemon mode can reuse them without inventing parallel semantics.
- TH3.E2 must keep polling as the guaranteed baseline resumption path even if event adapters are added later.