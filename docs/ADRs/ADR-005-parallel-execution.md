# ADR-005: Parallel Execution via Background Agents and Git Worktrees

## Status
Proposed

## Context

VP2 (§7) describes parallel execution of independent user stories within an
epic. Loom v1 processes stories sequentially — one FSM instance, one PR at a
time. This limits throughput to one story per FSM cycle.

VS Code v1.107 introduced:
- **Background agents**: sessions that run without a focused UI tab.
- **Git worktrees**: each background agent operates in an isolated worktree,
  preventing file-system collisions.

VP2 §7 proposes that the orchestrator spawn parallel background agents for
independent (unblocked) stories, each in its own worktree, with the orchestrator
advancing the dependency DAG only when all prerequisites are merged.

This is a **P3 deliverable** (VP2 §10) — not needed for MVP.

## Decision

### Design

1. The orchestrator evaluates the dependency DAG (via `internal/depgraph`) to
   identify all currently unblocked stories.

2. For each unblocked story, the orchestrator spawns a **background agent
   session** via the `code chat` CLI:
   ```bash
   code chat -m loom-orchestrator "Implement US-2.1" --worktree worktree-e2-us1
   ```

3. Each background agent:
   - Runs in an isolated Git worktree (`worktree-<epic>-<story>`).
   - Creates its own GitHub issue and PR.
   - Reports progress via `loom_checkpoint` calls to the shared SQLite store.
   - Opens a PR against the main branch.

4. The orchestrator polls `loom_get_state` for all active sessions and advances
   the DAG when prerequisites are met.

5. **Concurrency limit**: Maximum parallel agents = configurable (default: 3).
   Rate-limit-aware — the orchestrator backs off if GitHub API quota is low.

### State Isolation

| State | Isolation Mechanism |
|-------|-------------------|
| Git working tree | Separate worktree per agent |
| FSM state | Shared SQLite DB with per-story rows (extends checkpoint table) |
| GitHub PRs | One PR per story (different branches) |
| MCP server | Single server; agents share the stdio connection via session IDs |

### Checkpoint Table Extension

The `checkpoint` table gains a `story_id` column to support multiple concurrent
stories:

```sql
ALTER TABLE checkpoint ADD COLUMN story_id TEXT NOT NULL DEFAULT '';
```

The existing single-row behavior is preserved for sequential mode (`story_id = ''`).

### VP2 Traceability

| VP2 Section | Requirement | How Addressed |
|---|---|---|
| §7 | Parallel epic execution | Background agents + worktrees |
| §2 Gap 4 | Concurrency control | Per-story checkpoint rows + worktree isolation |
| §10 P3 | Background agent spawning | `code chat` CLI invocation |
| §10 P3 | Git worktree isolation | `--worktree` flag |

## Consequences

### Positive
- Throughput multiplied by number of independent stories.
- Worktree isolation eliminates file-system conflicts by construction.
- Incremental adoption — sequential mode remains the default.

### Negative
- Increased complexity in checkpoint table (multi-row instead of single-row).
- Requires `code` CLI availability on PATH (dev container or local VS Code install).
- Multiple concurrent GitHub API consumers — rate-limit budget must be shared.

### Risks
- Background agent API (`code chat --worktree`) is VS Code v1.107. May change.
  Mitigation: P3 priority; defer implementation until API stabilizes.
- SQLite concurrent writes from multiple processes. Mitigation: WAL mode (already
  default in `modernc.org/sqlite`); write transactions are short (single checkpoint row).

## Alternatives Considered

### A. Sequential-only execution (v1 status quo)
- Pros: Simple; single FSM instance; no concurrency concerns.
- Cons: Throughput limited to one story at a time.
- Rejected because: VP2 §7 explicitly requires parallel execution for throughput.

### B. Multiple Loom processes (one per story)
- Pros: Full isolation — separate SQLite DBs, separate FSM instances.
- Cons: No shared orchestration; each process needs its own config; complex to coordinate.
- Rejected because: Violates the "single orchestrator" design value.

### C. Go goroutines within Loom binary (no background agents)
- Pros: No VS Code dependency for parallelism.
- Cons: Loom binary would need to compose issue bodies and drive GitHub interactions
  without LLM — contradicts the plumbing/intelligence separation.
- Rejected because: LLM reasoning is needed per story; background agents provide it.
