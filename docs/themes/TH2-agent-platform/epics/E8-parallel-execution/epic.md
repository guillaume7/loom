# E8 — Parallel Execution

> Theme: [TH2 — Native Agent Platform](../../README.md)
> ADR: [ADR-005](../../../../ADRs/ADR-005-parallel-execution.md)
> Priority: P3

## Goal

Enable parallel execution of independent user stories via background agents
in isolated Git worktrees. The orchestrator evaluates the dependency DAG and
spawns concurrent agent sessions up to a configurable limit.

## Dependencies

- **E1** (Dependency Graph) — DAG evaluation for unblocked stories
- **E4** (Agent Definitions) — agents to spawn
- **E6** (MCP Elicitation) — full loop capability

## Stories

| Story | Title | Size | Depends On |
|-------|-------|------|------------|
| US1 | Checkpoint table story_id extension | S | — |
| US2 | Background agent spawning via code chat | M | US1 |
| US3 | Git worktree lifecycle management | M | US2 |
| US4 | DAG-aware parallel scheduling with concurrency limits | L | US1, US3 |

## Acceptance

Epic is done when:
- Checkpoint table supports per-story rows via `story_id` column
- `code chat` CLI spawns background agents in isolated worktrees
- Worktrees are created on spawn and cleaned up on completion
- Orchestrator evaluates DAG to identify parallel-safe stories
- Concurrency is capped at configurable limit (default: 3)
- GitHub API rate-limit budget is shared across parallel agents
