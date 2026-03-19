---
name: "E3 — MCP Resources"
about: Register MCP resources and server instructions so agents can inspect Loom internals without tool calls.
title: "E3: MCP Resources"
labels: ["epic", "E3", "TH2"]
---

## Goal

Register three MCP resources (`loom://dependencies`, `loom://state`, `loom://log`) and add server instructions to the MCP server.

## User Stories

- [ ] TH2.E3.US1 — MCP resource registration framework
- [ ] TH2.E3.US2 — loom://dependencies MCP resource
- [ ] TH2.E3.US3 — loom://state MCP resource
- [ ] TH2.E3.US4 — loom://log MCP resource
- [ ] TH2.E3.US5 — MCP server instructions with phase summary and dependency digest

## Acceptance Criteria

- [ ] MCP server responds to `resources/list` with three registered resources
- [ ] `loom://dependencies` returns YAML from `.loom/dependencies.yaml`
- [ ] `loom://state` returns JSON with FSM state, PR, retry counts
- [ ] `loom://log` returns NDJSON of recent actions
- [ ] Server instructions include phase summary and dependency digest