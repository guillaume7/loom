# E3 — Deterministic Policy Engine

> Theme: [TH3 — Runtime-First Reengineering](../../README.md)
> ADR: [ADR-009](../../../../ADRs/ADR-009-deterministic-runtime-policy-engine.md)
> Priority: P0

## Goal

Move gate, retry, escalation, and merge decisions into deterministic runtime
policy evaluation based on persisted observations instead of prompt-local
reasoning.

## Dependencies

- **E1** (Runtime Kernel Foundation) — durable runtime records and controller lifecycle

## Stories

| Story | Title | Size | Depends On |
| ----- | ----- | ---- | ---------- |
| US1 | Runtime observation model | M | TH3.E1.US2 |
| US2 | CI review and merge policy decisions | M | US1 |
| US3 | Escalation and wait outcome taxonomy | M | US1 |
| US4 | Policy evaluation audit trail | M | US2, US3 |

## Acceptance

Epic is done when:

- The runtime consumes explicit observations instead of relying on prompt reconstruction
- CI, review, merge, and retry decisions are expressed as deterministic policy outcomes
- Wait, retry, and escalate branches are named and auditable
- Operators can inspect why a policy result was produced
