# E6 — MCP Elicitation

> Theme: [TH2 — Native Agent Platform](../../README.md)
> ADR: [ADR-004](../../../../ADRs/ADR-004-mcp-tasks-and-elicitation.md)
> Priority: P2

## Goal

Implement structured MCP elicitation prompts on retry budget exhaustion,
enabling the operator to choose recovery actions (skip/reassign/pause) from
within the chat UI instead of switching to the terminal.

## Dependencies

- **E5** (MCP Tasks) — elicitation builds on MCP protocol extensions

## Stories

| Story | Title | Size | Depends On |
|-------|-------|------|------------|
| US1 | Elicitation schema and emission | M | — |
| US2 | skip_story FSM event and transition | M | — |
| US3 | Response handler with fallback | M | US1, US2 |

## Acceptance

Epic is done when:
- Budget exhaustion emits a structured elicitation (not immediate PAUSED)
- Operator can choose skip, reassign, or pause_epic from chat UI
- `skip_story` FSM event advances to next story
- `reassign` closes PR and resets to ISSUE_CREATED
- Clients without elicitation support fall back to PAUSED
