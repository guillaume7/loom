# Loom v2 — Tech Stack

> Traces to: [VP2 §11](../vision_of_product/VP2-agent-platform/02-vision-agent-platform.md) (what has not changed) and [VP2 §3](../vision_of_product/VP2-agent-platform/02-vision-agent-platform.md) (revised architecture)

## Retained from v1

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

## New in v2

| Component | Technology | Rationale | ADR |
|-----------|-----------|-----------|-----|
| Agent definitions | `.github/agents/*.agent.md` (VS Code custom agents) | Declarative, version-controlled, portable to GitHub cloud | ADR-002 |
| Dependency schema | `.loom/dependencies.yaml` (YAML) | Machine-readable DAG; consumed by Go binary and MCP resource | ADR-003 |
| MCP resources | `mcp-go` resource registration | Expose dependencies, state, log to any agent session | ADR-003 |
| MCP Tasks | MCP spec 2025-11-25 task lifecycle | Resilient long-running polls; client disconnect/reconnect | ADR-004 |
| MCP elicitation | MCP spec elicitation schema | Structured human-in-the-loop on budget exhaustion | ADR-004 |
| YAML parsing | `gopkg.in/yaml.v3` (already indirect dep) | Parse `.loom/dependencies.yaml` | ADR-003 |

## Dependency Policy

- **Direct dependencies are pinned** in `go.mod` with exact minor versions.
- **Lockfile** (`go.sum`) is committed and verified on CI.
- **Updates** are reviewed at epic boundaries. Security patches via Dependabot.
- **Major version bumps** require an ADR.

## Explicitly Not Chosen

| Technology | Reason |
|------------|--------|
| External orchestrator (Temporal, Prefect) | Over-engineered for single-project tool (ADR-001 §Alternatives D) |
| TypeScript / VS Code Extension | Tighter coupling, harder to test outside editor (ADR-001 §Alternatives C) |
| PostgreSQL / external database | SQLite is sufficient; no network dependency needed |
| gRPC for MCP transport | MCP spec uses stdio/SSE; gRPC is non-standard |
| Protobuf for dependency schema | YAML is human-readable and editable; protobuf adds tooling overhead |
