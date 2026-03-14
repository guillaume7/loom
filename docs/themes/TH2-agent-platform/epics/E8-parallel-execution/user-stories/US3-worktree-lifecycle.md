---
id: TH2.E8.US3
title: "Git worktree lifecycle management"
type: standard
priority: low
size: M
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "Worktree is created in a deterministic path based on story ID"
  - AC2: "Worktree is created from the main branch HEAD"
  - AC3: "Worktree is cleaned up (removed) when the agent session completes"
  - AC4: "Stale worktrees from crashed sessions can be cleaned up via loom reset"
depends-on: [TH2.E8.US2]
---

# TH2.E8.US3 — Git Worktree Lifecycle Management

**As a** Loom orchestrator, **I want** Git worktrees to be automatically created and cleaned up for parallel agents, **so that** agents have isolated working directories without file-system collisions.

## Acceptance Criteria

- [ ] AC1: Worktree is created in a deterministic path based on story ID
- [ ] AC2: Worktree is created from the main branch HEAD
- [ ] AC3: Worktree is cleaned up (removed) when the agent session completes
- [ ] AC4: Stale worktrees from crashed sessions can be cleaned up via `loom reset`

## BDD Scenarios

### Scenario: Create worktree for story
- **Given** story US-2.1 is being assigned to a background agent
- **When** the orchestrator creates a worktree
- **Then** a worktree exists at `../worktree-us-2.1` (relative to repo root)
- **And** the worktree is based on the main branch HEAD

### Scenario: Clean up worktree on completion
- **Given** a worktree at `../worktree-us-2.1` for a completed story
- **When** the agent session reports done
- **Then** the worktree is removed via `git worktree remove`

### Scenario: Clean up stale worktrees
- **Given** stale worktrees from crashed sessions exist
- **When** `loom reset` is called
- **Then** all loom-managed worktrees are removed
- **And** `git worktree prune` is called

### Scenario: Worktree already exists
- **Given** a worktree at `../worktree-us-2.1` already exists
- **When** the orchestrator tries to create it again
- **Then** the existing worktree is reused (not a fatal error)
