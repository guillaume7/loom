# E5 — Replay and Bounded Agent Jobs

> Theme: [TH3 — Runtime-First Reengineering](../../README.md)
> ADRs: [ADR-009](../../../../ADRs/ADR-009-deterministic-runtime-policy-engine.md), [ADR-010](../../../../ADRs/ADR-010-bounded-agent-jobs-and-run-locking.md)
> Priority: P1

## Goal

Turn runtime behavior into something reproducible by defining replay fixtures,
bounded agent job contracts, failure containment, and regression coverage for the
new control plane.

## Dependencies

- **E2** (Wake-Up and Resumption) — persisted observations and deduplicated wake-ups
- **E3** (Deterministic Policy Engine) — named outcomes and decision audit records
- **E4** (Locking and Recovery) — safe ownership and conflict handling

## Stories

| Story | Title | Size | Depends On |
| ----- | ----- | ---- | ---------- |
| US1 | Deterministic replay fixtures | M | TH3.E2.US2, TH3.E2.US3, TH3.E3.US4 |
| US2 | Bounded agent job contract | M | TH3.E1.US3, TH3.E4.US1 |
| US3 | Agent failure containment | M | US2, TH3.E4.US4 |
| US4 | Replay-driven runtime regression suite | M | US1, US3 |

## Acceptance

Epic is done when:

- Runtime behavior can be reproduced from stored observations and decisions
- Agent work is bounded, resumable, and not the source of orchestration truth
- Agent failures degrade to explicit runtime outcomes instead of global stalls
- Regression coverage exists for representative runtime-first scenarios
