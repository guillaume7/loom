# ADR-008: Runtime-First Control Plane and Wake Model

## Status
Accepted

## Context

Loom v1 and v2 assume that a long-lived Copilot session can remain the effective
control plane for asynchronous workflow progress. In practice, the system stalls
at the moments where autonomy matters most: CI waits, review waits, delayed
GitHub state changes, and session interruptions.

VP3 establishes a different boundary. Durable progress must not depend on an
interactive session surviving across waits. Loom's runtime already owns the
authoritative checkpoint state; it now also needs to own wake-ups, retry timing,
resume decisions, and forward progress scheduling.

### Forces

- **Reliability**: waiting states must continue safely without a chat tab or
  live session heartbeat.
- **Recoverability**: crashes and disconnects must preserve the exact next safe
  action.
- **Local-first operation**: the product must still run on a developer machine
  without requiring hosted infrastructure.
- **Operator control**: pause, override, and inspection paths must remain clear.
- **Compatibility**: existing CLI and MCP surfaces should remain viable operator
  entry points during migration.

## Decision

Adopt a **runtime-first control plane** for Loom.

1. The Go runtime becomes the primary owner of orchestration liveness:
   wake-ups, timers, retries, resume scheduling, and transition execution.
2. Interactive agent sessions are no longer required to stay alive during
   waiting states.
3. Loom must support a **resumable runner** as the minimum execution mode, with
   an optional daemonized mode as an additive operational variant.
4. CLI and MCP remain the operator and integration surfaces, but neither is the
   authoritative source of orchestration state.
5. Session traces remain derived audit artifacts. Checkpoints remain the source
   of truth.

## Consequences

### Positive

- Asynchronous waits no longer depend on chat-session liveness.
- Recovery after crash or disconnect becomes a runtime concern, not a prompt
  recovery exercise.
- The architecture better matches Loom's purpose: deterministic orchestration
  with bounded AI assistance.

### Negative

- The runtime surface becomes broader and more operationally complex.
- Background session features from VP2 become optional helpers rather than the
  core control mechanism.

### Risks

- If runtime mode is left ambiguous for too long, implementation can fragment
  between daemon and foreground-runner assumptions.
- The migration can accidentally duplicate responsibilities between runtime and
  agents unless the job contract is tightened.

## Alternatives Considered

### Keep the long-lived session as control plane

- Pros: Reuses more of the VP2 operating model.
- Cons: Preserves the exact stall mode that motivated VP3.
- Rejected because: autonomy cannot depend on interactive session continuity.

### Hosted orchestration service

- Pros: Strong durability and eventing model.
- Cons: Conflicts with Loom's local-first constraint and increases operating
  burden.
- Rejected because: VP3 requires a stronger runtime boundary, not a forced move
  to hosted infrastructure.