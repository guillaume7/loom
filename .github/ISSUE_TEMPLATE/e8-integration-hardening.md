---
name: "E8 — Integration & Hardening"
about: Verify the complete Loom workflow end-to-end with simulated GitHub interactions and ship cross-compiled release binaries.
title: "E8: Integration & Hardening"
labels: ["epic", "E8"]
---

## Assigned Agents

| Role | Agent | Required Skills |
|---|---|---|
| Owner | [Test Engineer](../agents/test-engineer.md) | [`tdd-workflow`](../skills/tdd-workflow.md) · [`loom-architecture`](../skills/loom-architecture.md) |
| Support | [Debugger](../agents/debugger.md) | [`loom-architecture`](../skills/loom-architecture.md) · [`tdd-workflow`](../skills/tdd-workflow.md) |
| Support | [DevOps / Release Manager](../agents/devops-release.md) | [`git-branching-workflow`](../skills/git-branching-workflow.md) |

## Goal

Verify the complete Loom workflow end-to-end with a simulated GitHub interaction, exercise all retry-budget exhaustion paths, and ship cross-compiled release binaries.

## Description

With all individual packages complete, E8 runs the full FSM + MCP + GitHub client + Store stack together. Integration tests use an `httptest` server that simulates GitHub API responses across a complete 3-phase lifecycle.

This epic also adds the release automation (git tag → CI artifact upload).

## User Stories

- [ ] US-8.1 — Integration test: simulate full 3-phase lifecycle (`IDLE` → `COMPLETE`)
- [ ] US-8.2 — Integration test: CI failure → `DEBUGGING` → fix → `AWAITING_CI` → `REVIEWING` → `MERGING`
- [ ] US-8.3 — Integration test: retry budget exhaustion → `PAUSED` → resume
- [ ] US-8.4 — Integration test: session stall detection → `PAUSED` → resume
- [ ] US-8.5 — Cross-compilation matrix verified in CI (linux/darwin/windows × amd64/arm64)
- [ ] US-8.6 — Release workflow: `git tag v*` triggers binary upload to GitHub Releases

## Dependencies

- E2 (FSM), E3 (GitHub client), E4 (MCP server), E5 (CLI), E6 (session), E7 (checkpointing)

## Acceptance Criteria

- [ ] US-8.1 integration test passes: machine reaches `COMPLETE` state in simulation
- [ ] US-8.2 integration test passes: debug loop terminates correctly
- [ ] US-8.3 integration test passes: `PAUSED` is reached on budget exhaustion; `loom resume` continues
- [ ] US-8.4 integration test passes: stall → `PAUSED` within configured timeout
- [ ] All 5 platform binaries build without errors in CI matrix
- [ ] Release workflow publishes binaries on `v*` tag push
- [ ] `go test ./... -race -cover` passes with overall coverage ≥ 80%

## Notes

<!-- Any additional context, design decisions, or blockers. -->
