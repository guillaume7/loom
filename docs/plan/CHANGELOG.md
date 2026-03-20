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

## Epic TH2.E5 — MCP Tasks

### Stories Completed
- TH2.E5.US1 — MCP Task lifecycle event emission
- TH2.E5.US2 — Heartbeat polling wrapped as MCP Task
- TH2.E5.US3 — Client capability negotiation and fallback

### Key Changes
- Added reusable MCP Task lifecycle emitter for start/progress/done notifications.
- Implemented heartbeat polling task flow with deterministic PR-based task IDs and CI status progress updates.
- Added client capability negotiation at initialization and session-cached fallback to blocking heartbeat behavior.
- Expanded MCP test harness to validate session initialization, emitted notifications, and capability-specific behavior.

### Files Modified
- internal/mcp/tasks.go
- internal/mcp/tasks_test.go
- internal/mcp/heartbeat_task.go
- internal/mcp/heartbeat_task_test.go
- internal/mcp/capabilities.go
- internal/mcp/handlers.go
- internal/mcp/server.go
- internal/mcp/server_test.go
- docs/plan/backlog.yaml
- docs/plan/session-log.md

## Epic TH2.E6 — MCP Elicitation

### Stories Completed
- TH2.E6.US1 — Elicitation schema definition and emission
- TH2.E6.US2 — skip_story FSM event and transition
- TH2.E6.US3 — Elicitation response handler with fallback

### Key Changes
- Added structured elicitation payload emission for budget-exhaustion paths with enum-based operator actions.
- Introduced `skip_story` FSM event and transitions with checkpoint phase advancement and skip detail tracking.
- Added elicitation response tool handling `skip`, `reassign`, and `pause_epic`, plus unsupported-client fallback to immediate `PAUSED`.
- Expanded capability negotiation and session-scoped handling for elicitation support.
- Added focused MCP and integration tests for emission ordering, response mapping, fallback behavior, and state transitions.

### Files Modified
- internal/mcp/elicitation.go
- internal/mcp/elicitation_test.go
- internal/mcp/elicitation_checkpoint_test.go
- internal/mcp/elicitation_response.go
- internal/mcp/elicitation_response_test.go
- internal/mcp/capabilities.go
- internal/mcp/handlers.go
- internal/mcp/server.go
- internal/mcp/server_atomic_test.go
- internal/mcp/server_core_test.go
- internal/mcp/server_skip_story_test.go
- internal/fsm/fsm.go
- internal/fsm/fsm_test.go
- internal/fsm/fsm_budget_test.go
- integration/helpers_test.go
- integration/budget_exhaustion_test.go
- docs/plan/backlog.yaml
- docs/plan/session-log.md

## Epic TH2.E7 — Security Hardening

### Stories Completed
- TH2.E7.US1 — GitHub token scope validation on startup
- TH2.E7.US2 — MCP tool readOnlyHint annotations
- TH2.E7.US3 — Config file permission warning

### Key Changes
- Added startup validation of GitHub token scopes (`repo` required, `read:org` warning-only) with a test bypass flag.
- Added explicit MCP tool `readOnlyHint` annotations in tool registration for policy-aware client behavior.
- Added startup warning for insecure `~/.loom/config.toml` permissions while keeping startup non-blocking.
- Expanded command and client tests for token scope handling, annotation metadata, and config permission checks.

### Files Modified
- cmd/loom/cmd_start.go
- cmd/loom/cmd_start_test.go
- internal/github/client.go
- internal/github/client_test.go
- internal/mcp/handlers.go
- internal/mcp/server_core_test.go
- internal/mcp/server.go
- internal/mcp/heartbeat_task.go
- internal/mcp/elicitation_response.go
- internal/mcp/server_test.go
- integration/helpers_test.go
- go.mod
- go.sum
- docs/plan/backlog.yaml
- docs/plan/session-log.md

## Epic TH3.E1 — Runtime Kernel Foundation

### Stories Completed
- TH3.E1.US1 — Runtime mode decision spike
- TH3.E1.US2 — Persisted run state and wake record model
- TH3.E1.US3 — Background controller lifecycle
- TH3.E1.US4 — Pause and manual override controls

### Key Changes
- Documented the VP3 runtime-mode baseline and tied it to the runtime-first control-plane direction.
- Added additive runtime persistence for wake schedules, external events, runtime leases, and policy decisions while preserving checkpoint truth.
- Introduced a persisted controller lifecycle for start, claim, sleep, wake, resume, pause, and shutdown across CLI and MCP state surfaces.
- Centralized operator pause and resume controls in the runtime controller so CLI pause, CLI resume, elicitation `pause_epic`, and MCP `loom_abort` all use the same audited manual-override path.
- Added regression coverage for recoverable pause semantics, resume safety, and story-scoped MCP pause behavior.

### Files Modified
- docs/themes/TH3-runtime-first-reengineering/epics/E1-runtime-kernel-foundation/runtime-mode-decision.md
- internal/store/store.go
- internal/store/sqlite_runtime.go
- internal/store/store_sqlite_runtime_test.go
- internal/runtime/controller.go
- internal/runtime/controller_test.go
- internal/runtime/operator_controls.go
- cmd/loom/cmd_start.go
- cmd/loom/cmd_pause.go
- cmd/loom/cmd_resume.go
- cmd/loom/cmd_log.go
- cmd/loom/main_test.go
- internal/mcp/handlers.go
- internal/mcp/runtime_state.go
- internal/mcp/elicitation_response.go
- internal/mcp/elicitation_response_test.go
- internal/mcp/story_checkpoint.go
- internal/mcp/story_checkpoint_test.go
- internal/mcp/server_core_test.go
- internal/mcp/server_resources_test.go
- docs/plan/backlog.yaml
- docs/plan/session-log.md
