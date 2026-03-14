# ADR-003: MCP Resources for Dependency Graph, State, and Log

## Status
Proposed

## Context

VP2 (§2 Gap 3, §4.1) identifies that Loom lacks machine-readable dependency
information and that agents have no way to inspect Loom's internal state without
calling a tool. The vision specifies three MCP resources:

1. `loom://dependencies` — the dependency DAG for epics and stories.
2. `loom://state` — current FSM state, active PR, retry counts.
3. `loom://log` — structured action history.

Additionally, VP2 §2 Gap 3 specifies `.loom/dependencies.yaml` as the canonical
store for the dependency graph, exposed as an MCP resource and summarized in
MCP server instructions.

The existing codebase has no dependency tracking — the FSM processes phases
sequentially by number. VP2 §7 requires parallel execution, which needs a DAG.

## Decision

### 1. Dependency Graph

Create a new `internal/depgraph` package that:
- Parses `.loom/dependencies.yaml` using `gopkg.in/yaml.v3`.
- Validates the schema (version field, no circular dependencies, valid ID references).
- Provides `Load(path) → Graph`, `Graph.Unblocked(done []string) → []string`, `Graph.IsBlocked(id, done []string) → bool`.

The YAML schema uses `version: 1` for future evolution:

```yaml
version: 1
epics:
  - id: E2
    depends_on: [E1]
    stories:
      - id: US-2.1
        depends_on: []
      - id: US-2.3
        depends_on: [US-2.1]
```

### 2. MCP Resources

Register three resources in the MCP server:

| URI | Format | Source | Update Frequency |
|-----|--------|--------|-----------------|
| `loom://dependencies` | YAML | File read from `.loom/dependencies.yaml` | On-demand (file read per request) |
| `loom://state` | JSON | Checkpoint table + FSM in-memory state | On-demand (fresh per request) |
| `loom://log` | NDJSON | Action log table (last 200 rows) | On-demand (query per request) |

Resources are read-only. They carry no `readOnlyHint` annotation because MCP
resources are inherently read-only.

### 3. Action Log Table

Add an `action_log` table to SQLite (via additive migration) with columns:
`id`, `session_id`, `operation_key` (unique), `state_before`, `state_after`,
`event`, `detail` (JSON), `created_at`.

The `operation_key` unique index provides idempotency enforcement: before
executing a write operation, the MCP server checks for an existing row.

### 4. MCP Server Instructions

The MCP server's `instructions` field (injected into every agent session) includes
a one-paragraph phase summary and a dependency digest (list of currently blocked
and unblocked stories).

### VP2 Traceability

| VP2 Section | Requirement | How Addressed |
|---|---|---|
| §2 Gap 3 | Machine-readable dependencies | `.loom/dependencies.yaml` + `internal/depgraph` |
| §2 Gap 3 | MCP resource for dependencies | `loom://dependencies` |
| §2 Gap 4 | Idempotency store | `action_log` table with unique `operation_key` |
| §4.1 | Three MCP resources | `loom://dependencies`, `loom://state`, `loom://log` |
| §4.1 | MCP server instructions | Phase summary + dependency digest |

## Consequences

### Positive
- Dependency graph is machine-readable — agents and the Go binary can evaluate it.
- MCP resources give agents read access without tool calls — reduces tool-call overhead.
- Action log enables idempotency enforcement and provides audit trail.
- Server instructions provide automatic context to every agent session.

### Negative
- New `internal/depgraph` package to maintain and test.
- `.loom/dependencies.yaml` must be kept in sync with the actual epic/story structure.
  Mitigation: Loom CLI can validate the file on `loom start`.

### Risks
- `mcp-go` library may not yet support MCP resources. Mitigation: check library
  version; implement as tool fallback if resources are unavailable. The `mcp-go`
  v0.18+ API includes resource registration.

## Alternatives Considered

### A. Embed dependencies in issue bodies (prose)
- Pros: No new file format or package.
- Cons: Not machine-readable; LLM must parse natural language.
- Rejected because: VP2 Gap 3 explicitly requires machine-readable format.

### B. Use GitHub issue labels for dependency tracking
- Pros: Visible in GitHub UI.
- Cons: Fragile; no graph operations; requires API calls to read.
- Rejected because: Labels are not a graph data structure.

### C. Store dependencies in SQLite instead of YAML
- Pros: Single storage backend.
- Cons: Not human-editable or diffable; harder to review in PRs.
- Rejected because: YAML allows humans to inspect and hand-edit the DAG.
