# E1 — Dependency Graph Engine

> Theme: [TH2 — Native Agent Platform](../../README.md)
> ADR: [ADR-003](../../../../ADRs/ADR-003-mcp-resources.md)
> Priority: P0

## Goal

Create an `internal/depgraph` package that parses `.loom/dependencies.yaml`,
validates the DAG, and evaluates which stories/epics are blocked or unblocked.
This is the foundation for MCP resource `loom://dependencies` and parallel
execution scheduling.

## Stories

| Story | Title | Size | Depends On |
|-------|-------|------|------------|
| US1 | YAML schema definition and parser | M | — |
| US2 | DAG validation (cycles, ID refs) | M | US1 |
| US3 | Unblocked/blocked evaluation | M | US2 |

## Acceptance

Epic is done when:
- `internal/depgraph` package compiles with zero external deps beyond `gopkg.in/yaml.v3`
- All three public methods (`Load`, `Unblocked`, `IsBlocked`) are tested
- Circular dependency detection works
- Package is importable by `internal/mcp` for resource serving
