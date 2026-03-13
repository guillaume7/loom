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
