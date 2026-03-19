---
name: "E1 — Dependency Graph Engine"
about: Create the dependency graph engine that parses .loom/dependencies.yaml, validates the DAG, and evaluates blocked or unblocked work.
title: "E1: Dependency Graph Engine"
labels: ["epic", "E1", "TH2"]
---

## Goal

Create an `internal/depgraph` package that parses `.loom/dependencies.yaml`, validates the DAG, and evaluates which stories and epics are blocked or unblocked.

## User Stories

- [ ] TH2.E1.US1 — YAML schema definition and parser
- [ ] TH2.E1.US2 — DAG validation (cycles, ID refs)
- [ ] TH2.E1.US3 — Unblocked/blocked evaluation

## Acceptance Criteria

- [ ] `internal/depgraph` package compiles with zero external deps beyond `gopkg.in/yaml.v3`
- [ ] All three public methods (`Load`, `Unblocked`, `IsBlocked`) are tested
- [ ] Circular dependency detection works
- [ ] Package is importable by `internal/mcp` for resource serving