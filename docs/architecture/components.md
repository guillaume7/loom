# Loom Runtime-First Architecture — Component Breakdown

> Traces to: [VP3](../vision_of_product/VP3-runtime-first/03-vision-runtime-first.md), [ADR-008](../ADRs/ADR-008-runtime-first-control-plane-and-wake-model.md), [ADR-009](../ADRs/ADR-009-deterministic-runtime-policy-engine.md), [ADR-010](../ADRs/ADR-010-bounded-agent-jobs-and-run-locking.md)

## Component Map

```text
┌────────────────────────────────────────────────────────────────┐
│  Operator Surfaces (CLI + MCP)                               │
└─────────┬─────────────────────────────────────────────────────┘
      ▼
┌────────────────────────────────────────────────────────────────┐
│  Runtime Controller                                             │
│  ┌────────────┐ ┌──────────────┐ ┌───────────────┐            │
│  │ Scheduler  │ │ Policy Engine│ │ Lock Manager  │            │
│  └─────┬──────┘ └──────┬───────┘ └──────┬────────┘            │
│        │               │                │                     │
│  ┌─────▼──────┐ ┌──────▼──────┐ ┌──────▼────────┐             │
│  │ Event Inbox │ │ Agent Jobs  │ │ Recovery Loop │             │
│  └─────────────┘ └─────────────┘ └───────────────┘             │
└────────┼──────────────┼─────────────┼───────────────┼──────────┘
         │              │             │               │
         ▼              ▼             ▼               ▼
┌────────────────────────────────────────────────────────────────┐
│  Persistence + Integration                                      │
│  ┌────────────┐ ┌─────────────┐ ┌──────────────┐ ┌───────────┐ │
│  │ SQLite     │ │ MCP Surface │ │ GitHub Client│ │ Config    │ │
│  └─────┬──────┘ └──────┬──────┘ └──────┬───────┘ └────┬──────┘ │
└────────┼───────────────┼───────────────┼──────────────┼────────┘
     │               │               │              │
     ▼               ▼               ▼              ▼
┌────────────────────────────────────────────────────────────────┐
│  External Systems                                               │
│  GitHub REST API · Optional GitHub Event Sources · Agent Hosts  │
└────────────────────────────────────────────────────────────────┘
```

---

## Component Details

### 1. Runtime Controller

- **Responsibility**: Own orchestration liveness, wake-ups, retries, locks, and recovery.
- **Interface**: Internal Go APIs plus MCP/CLI surfaces.
- **Data ownership**: Runtime leases, wake schedules, policy outcomes.
- **Dependencies**: FSM, Store, GitHub Client, Agent Job Runner.
- **ADR**: [ADR-008](../ADRs/ADR-008-runtime-first-control-plane-and-wake-model.md)

### 2. Policy Engine

- **Responsibility**: Evaluate CI, review, merge, dependency, and escalation decisions deterministically.
- **Interface**: `Evaluate(snapshot) -> decision`.
- **Data ownership**: Policy decision records.
- **Dependencies**: GitHub state snapshots, dependency graph, retry counters.
- **ADR**: [ADR-009](../ADRs/ADR-009-deterministic-runtime-policy-engine.md)

### 3. Lock Manager

- **Responsibility**: Prevent concurrent control of the same run or PR.
- **Interface**: Acquire, renew, release lease/lock.
- **Data ownership**: Run leases and narrower lock rows.
- **Dependencies**: Store and runtime clock.
- **ADR**: [ADR-010](../ADRs/ADR-010-bounded-agent-jobs-and-run-locking.md)

### 4. Agent Job Runner

- **Responsibility**: Execute bounded AI-assisted jobs with structured inputs/outputs.
- **Interface**: Submit job, await result, record failure/timeout.
- **Data ownership**: Agent job records and output envelopes.
- **Dependencies**: MCP surface or agent host integration.
- **ADR**: [ADR-010](../ADRs/ADR-010-bounded-agent-jobs-and-run-locking.md)

### 5. Store (`internal/store`)

- **Responsibility**: Persist checkpoints, locks, schedules, events, policy decisions, action log, and traces.
- **Interface**: Existing checkpoint methods plus additive methods for runtime scheduling, leases, external events, policy decisions, and agent jobs.
- **Data ownership**: SQLite database at `.loom/state.db`.
- **Dependencies**: `modernc.org/sqlite`.

### 6. MCP Surface (`internal/mcp`)

- **Responsibility**: Expose runtime tools and resources without becoming the control plane.
- **Interface**: MCP stdio transport with tools and read surfaces.
- **Data ownership**: None — delegates to runtime and store.
- **Dependencies**: Runtime controller, store, GitHub client.

### 7. Dependency Graph (`internal/depgraph`)

- **Responsibility**: Parse `.loom/dependencies.yaml` and answer dependency readiness.
- **Interface**: `Load(path) -> Graph`, blocking/unblocked queries.
- **Data ownership**: `.loom/dependencies.yaml` schema and validation.
- **Dependencies**: `gopkg.in/yaml.v3`.

### 8. GitHub Client (`internal/github`)

- **Responsibility**: Read and write GitHub issue, PR, review, and check state.
- **Interface**: Typed GitHub client methods.
- **Data ownership**: None.
- **Dependencies**: `google/go-github`, `net/http`.

### 9. CLI (`cmd/loom`)

- **Responsibility**: Human operator control over runtime lifecycle and inspection.
- **Interface**: Shell commands via `spf13/cobra`.
- **Data ownership**: None.
- **Dependencies**: All internal packages.

---

## Boundary Rules

1. **Runtime controller owns liveness** — no agent or surface may bypass runtime scheduling and recovery.
2. **Store owns durable state** — no other component writes SQLite directly.
3. **Policy engine is deterministic** — decisions are computed from explicit snapshots, not prompt history.
4. **Agent jobs are bounded** — they may return outputs, not mutate authoritative orchestration state directly.
5. **Locking precedes concurrency** — resumed or parallel work must acquire the correct lease first.
6. **Session trace remains append-only** — observability artifacts are never rewritten into source-of-truth state.
