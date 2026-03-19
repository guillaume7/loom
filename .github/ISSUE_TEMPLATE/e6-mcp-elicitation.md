---
name: "E6 — MCP Elicitation"
about: Implement structured operator prompts on budget exhaustion so recovery decisions happen in chat instead of the terminal.
title: "E6: MCP Elicitation"
labels: ["epic", "E6", "TH2"]
---

## Goal

Implement structured MCP elicitation prompts on retry budget exhaustion, enabling the operator to choose recovery actions from within the chat UI.

## User Stories

- [ ] TH2.E6.US1 — Elicitation schema definition and emission
- [ ] TH2.E6.US2 — skip_story FSM event and transition
- [ ] TH2.E6.US3 — Elicitation response handler with fallback

## Acceptance Criteria

- [ ] Budget exhaustion emits a structured elicitation (not immediate PAUSED)
- [ ] Operator can choose skip, reassign, or pause_epic from chat UI
- [ ] `skip_story` FSM event advances to the next story
- [ ] `reassign` closes PR and resets to ISSUE_CREATED
- [ ] Clients without elicitation support fall back to PAUSED