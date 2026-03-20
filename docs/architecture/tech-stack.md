# Loom Runtime-First Architecture — Tech Stack

> Traces to: [VP3](../vision_of_product/VP3-runtime-first/03-vision-runtime-first.md), [ADR-008](../ADRs/ADR-008-runtime-first-control-plane-and-wake-model.md), [ADR-009](../ADRs/ADR-009-deterministic-runtime-policy-engine.md), [ADR-010](../ADRs/ADR-010-bounded-agent-jobs-and-run-locking.md)

## Retained From Earlier Versions

These choices are unchanged from ADR-001. Rationale is documented there.

| Component | Technology | Version | Rationale |
|-----------|-----------|---------|-----------|
| Language | Go | 1.23+ | Single binary, strong stdlib, `go-github` ecosystem |
| State persistence | SQLite via `modernc.org/sqlite` | 1.30+ | Pure Go (no CGo), single file, crash-resilient |
| GitHub REST client | `google/go-github/v68` | 68.x | Well-maintained, typed API surface |
| MCP server | `mark3labs/mcp-go` | 0.18+ | MCP stdio transport, tool/resource registration |
| Structured logging | `log/slog` (stdlib) | — | Zero-dependency, JSON output |
| CLI framework | `spf13/cobra` | 1.8+ | Standard Go CLI framework |
| Configuration | `pelletier/go-toml/v2` | 2.2+ | TOML parsing for `~/.loom/config.toml` |
| Testing | `stretchr/testify` | 1.9+ | Assertions and mocking |

## Runtime-First Additions

| Component | Technology | Rationale | ADR |
|-----------|-----------|-----------|-----|
| Runtime controller | Go packages under `internal/` | Preserve single-binary local-first execution | ADR-008 |
| Wake schedules | SQLite tables + Go scheduler loop | Durable resumptions without active session heartbeat | ADR-008 |
| Policy engine | Pure Go evaluation code | Deterministic gate and escalation decisions | ADR-009 |
| Lock management | SQLite lease model | Safe resume and concurrency control | ADR-010 |
| Agent jobs | `.github/agents/*.agent.md` + MCP | Keep AI assistance bounded and replaceable | ADR-010 |
| Replay harness | JSON/YAML fixtures derived from runtime observations | Reproduce stalled runs locally | ADR-009 |

## Dependency Policy

- **Direct dependencies are pinned** in `go.mod` with exact minor versions.
- **Lockfile** (`go.sum`) is committed and verified on CI.
- **Updates** are reviewed at epic boundaries. Security patches via Dependabot.
- **Major version bumps** require an ADR.

## Runtime Mode Choice

Detailed comparison, rationale, and workflow impact are recorded in
[TH3.E1.US1 runtime mode decision](../themes/TH3-runtime-first-reengineering/epics/E1-runtime-kernel-foundation/runtime-mode-decision.md).

- **Chosen baseline**: resumable local runner owned by the Go binary.
- **Deferred additive option**: long-lived local daemon for teams that want
	always-on runtime behavior.
- **Signal model**: polling-first with a future hybrid event path. VP3 planning
	keeps polling as the guaranteed baseline while allowing GitHub event adapters
	to be added without changing checkpoint truth.

## Explicitly Not Chosen

| Technology | Reason |
|------------|--------|
| External orchestrator (Temporal, Prefect) | Over-engineered for single-project tool (ADR-001 §Alternatives D) |
| TypeScript / VS Code Extension | Tighter coupling, harder to test outside editor (ADR-001 §Alternatives C) |
| PostgreSQL / external database | SQLite is sufficient; no network dependency needed |
| gRPC for MCP transport | MCP spec uses stdio/SSE; gRPC is non-standard |
| Protobuf for dependency schema | YAML is human-readable and editable; protobuf adds tooling overhead |
| Agent session as primary control plane | Fails the VP3 liveness requirement under long waits |
