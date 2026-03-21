# Loom Runtime-First Architecture — System Context & High-Level Design

> Traces to: [VP3 — Runtime-First Autonomous Operations](../vision_of_product/VP3-runtime-first/03-vision-runtime-first.md), [ADR-008](../ADRs/ADR-008-runtime-first-control-plane-and-wake-model.md), [ADR-009](../ADRs/ADR-009-deterministic-runtime-policy-engine.md), [ADR-010](../ADRs/ADR-010-bounded-agent-jobs-and-run-locking.md)

## 1. System Context

Loom is a Go CLI tool and MCP server that orchestrates autonomous software
development workflows. In the VP3 architecture, the **runtime controller** is
the primary control plane. MCP, CLI, and agent sessions are surfaces around it,
not substitutes for it.

```text
                 ┌─────────────────────────────────────────┐
                 │        Human Operator                    │
                 │  (loom start | status | pause | resume) │
                 └─────────────────┬───────────────────────┘
                                   │ CLI
                                   ▼
┌──────────────────────────────────────────────────────────────────────┐
│  CLI / MCP / Operator UI                                              │
│  loom start | status | pause | resume | log | MCP resources           │
└──────────────────────────────┬────────────────────────────────────────┘
                               │ commands / tool calls
                               ▼
┌──────────────────────────────────────────────────────────────────────┐
│  Runtime Controller                                                  │
│  Scheduler · Wake Queue · Policy Engine · Lock Manager · Agent Jobs  │
└──────────────────────────────┬───────────────────────────────────────┘
                               │
             ┌─────────────────┼─────────────────┐
             │                 │                 │
            ▼
┌──────────────────────────────────────────────────────────────────────┐
│  Persistence                                                         │
│                                                                      │
│  Checkpoint Store · Wake Schedules · Event Inbox · Locks ·           │
│  Policy Decisions · Agent Jobs · Action Log · Session Trace          │
└──────────────────────────────────┬───────────────────────────────────┘
             │                     │                     │
             ▼                     ▼                     ▼
┌──────────────────────┐  ┌─────────────────────┐  ┌───────────────────┐
│  Agent Job Workers   │  │  Loom MCP Surface   │  │ GitHub Integration │
│  bounded, short-lived│  │ tools + resources   │  │ REST + polling     │
└──────────────────────┘  └─────────────────────┘  └───────────────────┘
                                                        │ GitHub REST API
                                   ▼
┌──────────────────────────────────────────────────────────────────────┐
│  GitHub.com                                                          │
│  Issues · Pull Requests · CI Checks · Copilot Coding Agent           │
└──────────────────────────────────────────────────────────────────────┘
```

## 2. High-Level Design

### 2.1 Core Principle: Runtime First

The Go binary owns **deterministic control**: checkpoint state, wake-ups,
policy evaluation, retry handling, lock ownership, and recovery.

Agents own **bounded contextual work**: drafting artifacts, summarizing external
state, and composing responses from structured inputs.

### 2.2 Layered Architecture

| Layer | Responsibility | Technology |
| ------- | --------------- | ------------ |
| **Operator Surfaces** | Human and tool entry points | CLI, MCP tools/resources |
| **Runtime Controller** | Scheduling, policy, retries, wake-ups, locks, recovery | Go packages under `internal/` |
| **Agent Job Runner** | Bounded AI-assisted work with structured I/O | Custom agents + MCP |
| **Persistence** | Checkpoints, schedules, locks, events, traces | SQLite via `modernc.org/sqlite` |
| **GitHub Integration** | Polling, event ingestion, action execution | `google/go-github`, `net/http` |

### 2.3 v2 → v3 Evolution

VP3 preserves the local-first and deterministic values of earlier versions but
changes who owns liveness.

| v2 | v3 |
| --- | --- |
| Agent-centric orchestration | Runtime-centric orchestration |
| Heartbeats keep sessions alive through waits | Runtime wake-ups own waits and resumptions |
| Gate logic split between runtime and prompts | Gate and escalation policy live in Go |
| Background agents as primary async mechanism | Bounded agent jobs as optional workers |
| Parallelism as a roadmap capability | Locking and safety before broad concurrency |

### 2.4 Key Invariants

1. **Checkpoint state is authoritative** for workflow progress.
2. **Wake-ups and retries are runtime-owned**, not prompt-owned.
3. **Policy decisions are deterministic artifacts** with inspectable inputs and outputs.
4. **Agents never own orchestration liveness**; they execute bounded jobs only.
5. **Locks gate concurrency** before any resumed or parallel work proceeds.
6. **Session traces and replay inputs remain derived observability artifacts** and never replace checkpoint truth.

## 3. Cross-References

| Document | Content |
| ---------- | --------- |
| [tech-stack.md](tech-stack.md) | Runtime-first technology choices and rationale |
| [components.md](components.md) | VP3 component boundaries |
| [data-model.md](data-model.md) | Durable runtime data model |
| [project-setup.md](project-setup.md) | Repository structure and build conventions |
| [deployment.md](deployment.md) | Runtime modes and operator deployment concerns |
| [ADR-008](../ADRs/ADR-008-runtime-first-control-plane-and-wake-model.md) | Runtime-first control plane |
| [ADR-009](../ADRs/ADR-009-deterministic-runtime-policy-engine.md) | Deterministic policy engine |
| [ADR-010](../ADRs/ADR-010-bounded-agent-jobs-and-run-locking.md) | Bounded agent jobs and locking |
