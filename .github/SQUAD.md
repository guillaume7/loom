# Loom Agent Squad — Overview

This directory contains the **agent definitions** and **shared skills** for the Loom development squad — a team of specialist AI agents collaborating to implement Loom from design documents to a working Go binary + MCP server.

---

## Squad Roster

| Agent | File | Primary Responsibility |
|---|---|---|
| **Orchestrator** | [`agents/orchestrator.md`](agents/orchestrator.md) | Coordinates work, enforces implementation order, tracks progress |
| **Product Manager** | [`agents/product-manager.md`](agents/product-manager.md) | Owns backlog, acceptance criteria, scope decisions |
| **Architect** | [`agents/architect.md`](agents/architect.md) | Module boundaries, Go interfaces, ADRs, MCP protocol design |
| **Backend Developer** | [`agents/backend-developer.md`](agents/backend-developer.md) | Go implementation: FSM, GitHub client, MCP server, store |
| **Test Engineer** | [`agents/test-engineer.md`](agents/test-engineer.md) | Test strategy, fixtures, coverage enforcement |
| **Reviewer** | [`agents/reviewer.md`](agents/reviewer.md) | PR review, Definition of Done, quality gate |
| **Refactoring Agent** | [`agents/refactoring-agent.md`](agents/refactoring-agent.md) | Continuous code quality, tech debt, cleanup |
| **Debugger** | [`agents/debugger.md`](agents/debugger.md) | Root-cause analysis, regression tests, bug fixes |
| **DevOps / Release Manager** | [`agents/devops-release.md`](agents/devops-release.md) | CI/CD, Go build config, versioning, deployment |

---

## Shared Skills

| Skill | File | Used By |
|---|---|---|
| Loom Architecture | [`skills/loom-architecture.md`](skills/loom-architecture.md) | All agents — authoritative architecture reference |
| Go Standards | [`skills/go-standards.md`](skills/go-standards.md) | Backend Dev, Reviewer, Refactoring Agent |
| TDD Workflow (Go) | [`skills/tdd-workflow.md`](skills/tdd-workflow.md) | Backend Dev, Test Engineer, Debugger |
| Git Branching Workflow | [`skills/git-branching-workflow.md`](skills/git-branching-workflow.md) | All agents that commit code |
| Review Checklist | [`skills/review-checklist.md`](skills/review-checklist.md) | Reviewer, Refactoring Agent |
| Epic & Story Breakdown | [`skills/epic-story-breakdown.md`](skills/epic-story-breakdown.md) | Orchestrator, Product Manager |
| GitHub Phase Loop | [`skills/github-phase-loop.md`](skills/github-phase-loop.md) | Orchestrator — drives issue→PR→review→CI→merge loop |

---

## Implementation Plan

The squad follows the implementation order defined in [`docs/epics/README.md`](../docs/epics/README.md):

```
E1 → E2 → E3 → E4 → E5 → E6 → E7 → E8
```

### Phase 0 — Foundation (E1)
**Owner**: DevOps + Backend Developer
Go module init, project layout, CI workflow, Makefile.

### Phase 1 — State Machine (E2)
**Owner**: Architect + Backend Developer
FSM states, transitions, guards, retry budgets, PAUSED escape hatch.

### Phase 2 — GitHub Client (E3)
**Owner**: Backend Developer + Test Engineer
GitHub REST API wrapper: issue CRUD, PR polling, CI check-run status, merge.

### Phase 3 — MCP Server (E4)
**Owner**: Backend Developer
MCP stdio server: tool registration, `loom_next_step`, `loom_checkpoint`, `loom_heartbeat`, `loom_get_state`, `loom_abort`.

### Phase 4 — CLI (E5)
**Owner**: Backend Developer
`loom start`, `loom status`, `loom pause`, `loom resume`, `loom reset`, `loom log`.

### Phase 5 — Session Management (E6)
**Owner**: Backend Developer
Session keepalive detection, stall detection, graceful restart on connection loss.

### Phase 6 — Checkpointing (E7)
**Owner**: Backend Developer
SQLite store: read/write checkpoints, idempotent step execution, replay on restart.

### Phase 7 — Integration & Hardening (E8)
**Owner**: Test Engineer + Debugger
End-to-end integration tests, retry budget exercises, PAUSED state recovery scenarios.

---

## Definition of Done

A user story is **Done** when:

1. All acceptance criteria from the user story file are checked off
2. Implementation merged to `main` via approved PR
3. All CI checks pass: `go vet`, `golangci-lint`, `go test ./... -race -cover`
4. New code has ≥ 90% branch coverage
5. Reviewer approved the PR
6. `go build ./cmd/loom` produces a clean binary with no warnings

---

## Communication Protocol

- **Agent → Orchestrator**: blocked? report immediately with context
- **Orchestrator → Agent**: new assignment? provide story link + acceptance criteria
- **Any agent → Architect**: architecture question? provide the specific decision needed
- **Reviewer → Author**: feedback in PR comments using BLOCKER / SUGGESTION / NIT labels
