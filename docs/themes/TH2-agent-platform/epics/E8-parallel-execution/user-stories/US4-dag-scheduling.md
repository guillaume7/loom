---
id: TH2.E8.US4
title: "DAG-aware parallel scheduling with concurrency limits"
type: standard
priority: low
size: L
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "Orchestrator evaluates depgraph to identify all currently unblocked stories"
  - AC2: "Up to max_parallel (configurable, default 3) stories are spawned concurrently"
  - AC3: "When a story completes, the DAG is re-evaluated and newly unblocked stories are spawned"
  - AC4: "GitHub API rate-limit budget is checked before spawning new agents"
  - AC5: "If rate limit is low, spawning is deferred until budget recovers"
depends-on: [TH2.E8.US1, TH2.E8.US3]
---

# TH2.E8.US4 — DAG-Aware Parallel Scheduling with Concurrency Limits

**As a** Loom orchestrator, **I want** to schedule parallel story execution based on the dependency graph and a concurrency limit, **so that** throughput is maximized without exceeding resource constraints.

## Acceptance Criteria

- [ ] AC1: Orchestrator evaluates depgraph to identify all currently unblocked stories
- [ ] AC2: Up to `max_parallel` (configurable, default 3) stories are spawned concurrently
- [ ] AC3: When a story completes, the DAG is re-evaluated and newly unblocked stories are spawned
- [ ] AC4: GitHub API rate-limit budget is checked before spawning new agents
- [ ] AC5: If rate limit is low, spawning is deferred until budget recovers

## BDD Scenarios

### Scenario: Spawn multiple unblocked stories
- **Given** a depgraph with stories US-2.1, US-2.2, US-2.3 all unblocked and max_parallel = 3
- **When** the scheduler evaluates the graph
- **Then** all 3 stories are spawned as background agents

### Scenario: Concurrency limit caps spawning
- **Given** 5 unblocked stories and max_parallel = 3
- **When** the scheduler evaluates the graph
- **Then** only 3 stories are spawned
- **And** the remaining 2 wait until a slot becomes available

### Scenario: Completion unblocks dependents
- **Given** US-2.1 is running and US-2.4 depends on US-2.1
- **When** US-2.1 completes (merged)
- **Then** the DAG is re-evaluated
- **And** US-2.4 becomes eligible and is spawned

### Scenario: Rate limit deferral
- **Given** GitHub API rate limit remaining is below threshold (e.g., < 100)
- **When** the scheduler tries to spawn a new agent
- **Then** spawning is deferred
- **And** a log message indicates rate-limit backoff

### Scenario: All stories complete
- **Given** all stories in an epic are done
- **When** the scheduler evaluates the graph
- **Then** no new agents are spawned
- **And** the epic is marked complete
