---
name: "E4 — Agent Definitions"
about: Create the custom agent definition files that implement Loom's multi-agent orchestration model.
title: "E4: Agent Definitions"
labels: ["epic", "E4", "TH2"]
---

## Goal

Create four custom agent definition files in `.github/agents/` that implement the multi-agent orchestration model: orchestrator, gate, debug, and merge.

## User Stories

- [ ] TH2.E4.US1 — Orchestrator agent definition
- [ ] TH2.E4.US2 — Gate agent definition
- [ ] TH2.E4.US3 — Debug agent definition
- [ ] TH2.E4.US4 — Merge agent definition

## Acceptance Criteria

- [ ] All four `.agent.md` files exist in `.github/agents/`
- [ ] Orchestrator has handoff wiring to gate, debug, and merge
- [ ] Gate is read-only with structured PASS/FAIL return
- [ ] Debug can only read and comment
- [ ] Merge can only execute merge operations
- [ ] All files have `target: vscode` and are valid custom agent format