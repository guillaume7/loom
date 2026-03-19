# Loom v2 — Data Model

> Traces to: [VP2 §4](../vision_of_product/VP2-agent-platform/02-vision-agent-platform.md) (new tool surface), [VP2 §8](../vision_of_product/VP2-agent-platform/02-vision-agent-platform.md) (what Loom still owns)

## 1. SQLite Database (`.loom/state.db`)

### 1.1 Checkpoint Table (existing)

Single-row table holding the latest FSM snapshot. Written on every `loom_checkpoint` call.

```sql
CREATE TABLE checkpoint (
    id            INTEGER PRIMARY KEY,  -- always 1
    state         TEXT    NOT NULL,      -- FSM state name (e.g. "AWAITING_CI")
    phase         INTEGER NOT NULL DEFAULT 0,
    pr_number     INTEGER NOT NULL DEFAULT 0,
    issue_number  INTEGER NOT NULL DEFAULT 0,
    retry_count   INTEGER NOT NULL DEFAULT 0,
    updated_at    TEXT    NOT NULL DEFAULT ''
);
```

**Go struct**: `store.Checkpoint` — fields map 1:1.

### 1.2 Action Log Table (v2 — new)

Append-only log of every action taken by Loom. Used for idempotency enforcement, the `loom://log` MCP resource, and the `loom log` CLI command.

```sql
CREATE TABLE action_log (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id     TEXT    NOT NULL,      -- identifies the agent session
    operation_key  TEXT    NOT NULL,       -- idempotency key (e.g. "create_issue:E2-US1")
    state_before   TEXT    NOT NULL,       -- FSM state before action
    state_after    TEXT    NOT NULL,       -- FSM state after action
    event          TEXT    NOT NULL,       -- FSM event fired
    detail         TEXT    NOT NULL DEFAULT '',  -- JSON payload (PR number, error, etc.)
    created_at     TEXT    NOT NULL        -- ISO 8601 timestamp
);

CREATE UNIQUE INDEX idx_operation_key ON action_log(operation_key);
```

**Idempotency contract**: Before executing a write operation, the MCP server checks `SELECT 1 FROM action_log WHERE operation_key = ?`. If the row exists, the operation is skipped and the existing result is returned.

### 1.3 Session Run Table (v2 — new)

Single header row per `/run-loom` execution. This is the stable metadata shown at the top of the session trace surface.

```sql
CREATE TABLE session_run (
  session_id    TEXT PRIMARY KEY,
  loom_version  TEXT NOT NULL,
  release_tag   TEXT NOT NULL DEFAULT '',
  repo_owner    TEXT NOT NULL,
  repo_name     TEXT NOT NULL,
  started_at    TEXT NOT NULL,
  ended_at      TEXT NOT NULL DEFAULT '',
  outcome       TEXT NOT NULL DEFAULT 'running'
);
```

### 1.4 Session Trace Event Table (v2 — new)

Append-only event stream for post-mortem replay. Entries are written in chronological order and folded at read time into a timeline, FSM ledger, and GitHub issue/PR evolution ledger.

```sql
CREATE TABLE session_trace_event (
  id               INTEGER PRIMARY KEY AUTOINCREMENT,
  session_id       TEXT NOT NULL,
  event_kind       TEXT NOT NULL,
  sequence_number  INTEGER NOT NULL,
  state_before     TEXT NOT NULL DEFAULT '',
  state_after      TEXT NOT NULL DEFAULT '',
  reason           TEXT NOT NULL DEFAULT '',
  correlation_id   TEXT NOT NULL DEFAULT '',
  payload          TEXT NOT NULL DEFAULT '',
  created_at       TEXT NOT NULL,
  FOREIGN KEY (session_id) REFERENCES session_run(session_id)
);

CREATE UNIQUE INDEX idx_session_trace_sequence
  ON session_trace_event(session_id, sequence_number);
```

Recommended `event_kind` values:

- `session_started`
- `fsm_transition`
- `retry_incremented`
- `budget_exhausted`
- `elicitation_emitted`
- `elicitation_resolved`
- `paused`
- `resumed`
- `github_issue_state_changed`
- `github_pr_state_changed`
- `session_completed`

### 1.5 Migration Strategy

Forward-compatible, additive-only. The existing `migrate()` function in `internal/store/sqlite.go` already handles `ALTER TABLE ADD COLUMN` for missing columns. The v2 migration adds the `action_log` table via `CREATE TABLE IF NOT EXISTS`.

The session trace capability extends the migration set with `session_run` and `session_trace_event`, also created via additive `CREATE TABLE IF NOT EXISTS` statements.

---

## 2. Dependency Graph (`.loom/dependencies.yaml`)

Machine-readable epic/story dependency DAG. Parsed by the `internal/depgraph` package and served via the `loom://dependencies` MCP resource.

### 2.1 Schema

```yaml
# .loom/dependencies.yaml
version: 1
epics:
  - id: E2
    depends_on: [E1]
    stories:
      - id: US-2.1
        depends_on: []
      - id: US-2.2
        depends_on: []
      - id: US-2.3
        depends_on: [US-2.1]
  - id: E3
    depends_on: [E2]
    stories:
      - id: US-3.1
        depends_on: []
```

### 2.2 Rules

- `depends_on` references use the same ID scheme as the story files.
- A story is **unblocked** when all entries in its `depends_on` list (and the epic's `depends_on` list) have status `done` in the checkpoint store.
- Circular dependencies are a parse-time error.
- The schema `version` field enables future format evolution.

---

## 3. MCP Resources (v2)

Resources are read-only views served by the MCP server to any connected agent session.

| URI | Format | Source | Description |
| ----- | -------- | -------- | ------------- |
| `loom://dependencies` | YAML | `.loom/dependencies.yaml` file | Full dependency graph |
| `loom://state` | JSON | Checkpoint table + FSM in-memory state | Current state, phase, PR, retry counts, last action |
| `loom://log` | NDJSON | Action log table (last 200 entries) | Structured history of all Loom actions |
| `loom://sessions` | JSON | `session_run` table | Retained session index for discoverability |
| `loom://session/<id>` | Markdown | `session_run` + `session_trace_event` | Human-readable trace tab for one run |

### 3.1 `loom://state` Schema

```json
{
  "state": "AWAITING_CI",
  "phase": 2,
  "pr_number": 42,
  "issue_number": 37,
  "retry_count": 1,
  "updated_at": "2026-03-13T10:30:00Z",
  "unblocked_stories": ["US-2.3", "US-2.4"]
}
```

### 3.2 `loom://log` Schema (per line)

```json
{
  "id": 15,
  "session_id": "abc-123",
  "operation_key": "merge_pr:42",
  "state_before": "MERGING",
  "state_after": "SCANNING",
  "event": "merged",
  "detail": "{\"pr\":42,\"sha\":\"a1b2c3\"}",
  "created_at": "2026-03-13T10:31:00Z"
}
```

### 3.3 `loom://sessions` Schema

```json
[
  {
    "session_id": "run-20260319-001",
    "repo": "guillaume7/loom",
    "started_at": "2026-03-19T19:29:41Z",
    "ended_at": "2026-03-19T20:04:13Z",
    "outcome": "paused",
    "loom_version": "1.2.0",
    "release_tag": "v1.2.0"
  }
]
```

### 3.4 `loom://session/<id>` Rendering Contract

The session trace resource renders a single Markdown document with:

1. A stable header showing Loom binary version, release tag, session identifier, repository, start time, end time, and final outcome.
2. A chronological event timeline.
3. An FSM transition ledger with `from_state`, `to_state`, and reason.
4. A GitHub issue/PR ledger showing state evolution over time.
5. Explicit entries for retries, elicitations, operator responses, pauses, and resumes.

The resource is derived from the append-only `session_trace_event` stream rather than stored as a mutable document blob.

---

## 4. MCP Elicitation Schema (v2)

Structured prompt sent to the operator when a retry budget is exhausted.

```json
{
  "type": "elicitation",
  "title": "PR #42 — CI budget exhausted",
  "description": "check_suite 'build' has failed 5 times. Choose an action.",
  "schema": {
    "action": {
      "type": "string",
      "enum": ["skip", "reassign", "pause_epic"],
      "enumDescriptions": [
        "Skip this user story and advance to the next",
        "Re-assign the PR to a fresh @copilot session",
        "Pause the epic and require human intervention"
      ]
    }
  }
}
```

The Go binary validates the response and maps it to an FSM event.

---

## 5. Configuration (unchanged)

`~/.loom/config.toml` with environment variable overrides. Schema documented in `internal/config/config.go`.

---

## 6. Entity Relationships

```text
Config ──loads──→ Loom binary
                     │
                     ├── FSM (in-memory state)
                     │     │
                     │     └── persisted via ──→ Checkpoint (SQLite row)
                     │
                     ├── Action Log (SQLite table) ←── written by MCP tools
                     │     │
                     │     └── served via ──→ loom://log (MCP resource)
                     │
                     ├── Session Run + Trace Events (SQLite tables)
                     │     │
                     │     ├── served via ──→ loom://sessions (MCP resource)
                     │     └── rendered via ──→ loom://session/<id> (MCP resource)
                     │
                     ├── DepGraph (in-memory) ←── parsed from .loom/dependencies.yaml
                     │     │
                     │     └── served via ──→ loom://dependencies (MCP resource)
                     │
                     └── GitHub Client ──→ GitHub.com (Issues, PRs, Checks, Reviews)
```
