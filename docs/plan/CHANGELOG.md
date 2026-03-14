## Epic TH2.E4 — Agent Definitions

### Stories Completed
- TH2.E4.US1 — Orchestrator agent definition
- TH2.E4.US2 — Gate agent definition
- TH2.E4.US3 — Debug agent definition
- TH2.E4.US4 — Merge agent definition

### Key Changes
- Added VS Code custom agent definition for the Loom orchestrator loop: drives FSM, checkpoints, and hands off to specialist agents.
- Added read-only gate agent that evaluates PR merge readiness (CI, approvals, draft state, conflicts) and returns structured PASS/FAIL verdict.
- Added debug agent that diagnoses CI failures, posts structured debug comments, and returns comment ID.
- Added merge agent with minimal merge-only tool scope, respects branch protection, returns structured JSON result.

### Files Modified
- .github/agents/loom-orchestrator.agent.md
- .github/agents/loom-gate.agent.md
- .github/agents/loom-debug.agent.md
- .github/agents/loom-merge.agent.md

## Epic TH2.E1 — Dependency Graph Engine

### Stories Completed
- TH2.E1.US1 — YAML schema definition and parser
- TH2.E1.US2 — DAG validation cycle and reference checks
- TH2.E1.US3 — Unblocked and blocked evaluation functions

### Key Changes
- Added new depgraph package with typed YAML parsing for .loom/dependencies.yaml and strict schema-version checks.
- Added graph validation for duplicate IDs, unknown dependency references, and cycle detection with cycle path reporting.
- Added orchestrator-facing eligibility APIs for blocked/unblocked evaluation that account for story-level and epic-level dependencies.
- Expanded unit test coverage for parser behavior, validation errors, and eligibility evaluation flows.

### Files Modified
- internal/depgraph/depgraph.go
- internal/depgraph/depgraph_test.go
- docs/plan/backlog.yaml
- docs/plan/session-log.md

## Epic TH2.E2 — Action Log & Idempotency

### Stories Completed
- TH2.E2.US1 — Action log table migration
- TH2.E2.US2 — WriteAction and ReadActions store methods
- TH2.E2.US3 — Idempotency check in MCP tool handlers
- TH2.E2.US4 — loom log CLI shows action history

### Key Changes
- Added additive SQLite migration for the action_log table and unique operation keys.
- Added durable store APIs for writing, listing, and looking up action-log entries, plus atomic checkpoint-and-action persistence.
- Enforced MCP checkpoint idempotency with cached replay and rollback-safe failure handling.
- Updated loom log to display structured action history from the database with limit support.
- Expanded tests across store, MCP, FSM rollback, integration helpers, and CLI log behavior.

### Files Modified
- internal/store/store.go
- internal/store/sqlite.go
- internal/store/store_test.go
- internal/store/migrate_test.go
- internal/mcp/server.go
- internal/mcp/server_test.go
- internal/fsm/fsm.go
- internal/fsm/fsm_test.go
- integration/helpers_test.go
- cmd/loom/cmd_log.go
- cmd/loom/main_test.go
- docs/plan/backlog.yaml
- docs/plan/session-log.md
