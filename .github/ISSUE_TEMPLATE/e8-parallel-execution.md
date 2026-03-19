---
name: "E8 — Parallel Execution"
about: Enable parallel execution of independent stories via background agents in isolated Git worktrees.
title: "E8: Parallel Execution"
labels: ["epic", "E8", "TH2"]
---

## Goal

Enable parallel execution of independent user stories via background agents in isolated Git worktrees.

## User Stories

- [ ] TH2.E8.US1 — Checkpoint table story_id extension
- [ ] TH2.E8.US2 — Background agent spawning via code chat
- [ ] TH2.E8.US3 — Git worktree lifecycle management
- [ ] TH2.E8.US4 — DAG-aware parallel scheduling with concurrency limits

## Acceptance Criteria

- [ ] Checkpoint table supports per-story rows via `story_id` column
- [ ] `code chat` CLI spawns background agents in isolated worktrees
- [ ] Worktrees are created on spawn and cleaned up on completion
- [ ] Orchestrator evaluates DAG to identify parallel-safe stories
- [ ] Concurrency is capped at configurable limit (default: 3)
- [ ] GitHub API rate-limit budget is shared across parallel agents