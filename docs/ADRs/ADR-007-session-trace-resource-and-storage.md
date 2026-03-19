# ADR-007: Session Trace Storage and Operator Surface

## Status

Proposed

## Context

VP2 section 10 adds a new capability: every `/run-loom` session should produce
a persistent trace tab that captures Loom build metadata, chronological events,
FSM transitions, GitHub issue and pull request evolution, and operator
interventions.

Existing observability surfaces are not sufficient on their own:

1. `loom://state` is a current snapshot, not a replayable history.
2. `loom://log` is structured and append-only, but optimized for machines and
   terminal inspection rather than an operator-facing narrative.
3. The FSM checkpoint is the source of truth for current orchestration state,
   but it intentionally retains only the latest snapshot.

The missing decision boundary is how Loom should persist and expose a durable,
auditable, human-readable session trace without introducing a second source of
truth for orchestration.

## Decision

### 1. Durable Storage Model

Store session traces in SQLite using two additive tables:

- `session_run` for one header row per `/run-loom` execution
- `session_trace_event` for append-only event entries keyed by `session_id`

The trace is a derived audit artifact. The FSM checkpoint remains the
authoritative current-state store.

### 2. Event Model

Persist normalized trace events rather than mutable rendered documents.

Each event entry records:

- `session_id`
- monotonic `sequence_number`
- `event_kind`
- `state_before` and `state_after` when applicable
- human-readable `reason`
- `correlation_id` for checkpoint/log/GitHub linkage
- JSON `payload`
- `created_at`

This supports replay and multiple renderings while preserving append-only
auditing.

### 3. GitHub State Transcription

GitHub issues and pull requests are transcribed as event entries when Loom
observes a meaningful state change. Loom does not persist a mutable
"current GitHub state" table as part of the primary design.

Instead, the trace reader folds the event stream into:

- a chronological timeline
- an FSM transition ledger
- a GitHub issue/PR evolution ledger

Duplicate polls with unchanged GitHub state do not append new trace events.

### 4. Operator Surface

Expose session traces through MCP resources:

- `loom://sessions` returns a JSON index of retained session runs
- `loom://session/<id>` returns a Markdown document suitable for a persistent
  tab in the client

The Markdown rendering includes:

1. A stable header with Loom version, release tag, session ID, repository,
   start time, end time, and terminal outcome.
2. A chronological event timeline.
3. An FSM transition ledger.
4. A GitHub issue/PR evolution ledger.
5. Explicit entries for retries, elicitations, operator responses, pauses, and
   resumes.

### 5. Retention Policy

Retention operates at whole-session granularity. Loom may prune expired or
excess traces by removing entire sessions; it does not rewrite retained trace
entries.

## Consequences

### Positive

- Operators can reconstruct a `/run-loom` session from one retained artifact.
- The architecture keeps a clean boundary between current authoritative state
  and derived audit history.
- Multiple read surfaces are possible from the same durable event model.
- GitHub state evolution can be analyzed even after the live GitHub state has
  changed.

### Negative

- Additional SQLite tables and query paths must be implemented and tested.
- Markdown rendering introduces presentation logic in the MCP resource layer.
- Sequence management and deduplication must be handled carefully to preserve
  audit quality.

### Risks

- Over-capturing low-signal events could make traces noisy and expensive.
  Mitigation: only append meaningful state changes and operator-visible events.
- Trace rendering could become slow for long sessions.
  Mitigation: bounded retention, indexed reads, and event folding scoped to one
  session.

## Alternatives Considered

### A. Store only rendered Markdown blobs

- Pros: Simple read path for the client.
- Cons: Hard to audit, hard to re-render, and expensive to update safely.
- Rejected because: append-only event storage is a better fit for replay and
  correlation.

### B. Reconstruct traces on demand from checkpoint plus action log only

- Pros: No new tables.
- Cons: Missing run header semantics, incomplete GitHub evolution state, and no
  explicit operator intervention ledger.
- Rejected because: the existing stores do not carry enough structured data.

### C. Make the session trace the orchestration source of truth

- Pros: Single durable history model.
- Cons: Blurs audit history with execution control and complicates FSM
  correctness guarantees.
- Rejected because: the checkpoint/FSM model is already the authoritative state
  mechanism.
