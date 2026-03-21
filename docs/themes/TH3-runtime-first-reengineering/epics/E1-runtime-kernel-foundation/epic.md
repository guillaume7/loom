# E1 — Runtime Kernel Foundation

> Theme: [TH3 — Runtime-First Reengineering](../../README.md)
> ADRs: [ADR-008](../../../../ADRs/ADR-008-runtime-first-control-plane-and-wake-model.md), [ADR-010](../../../../ADRs/ADR-010-bounded-agent-jobs-and-run-locking.md)
> Priority: P0

## Goal

Establish the minimum runtime-first foundation: decide the operating mode,
define durable run-state extensions, add a controller lifecycle, and preserve
safe operator pause and override behavior.

## Dependencies

- None

## Stories

| Story | Title | Size | Depends On |
| ----- | ----- | ---- | ---------- |
| US1 | Runtime mode decision spike | S | — |
| US2 | Persisted run state and wake record model | M | US1 |
| US3 | Background controller lifecycle | M | US2 |
| US4 | Pause and manual override controls | M | US3 |

## Acceptance

Epic is done when:

- Loom has a documented runtime mode baseline for VP3
- Durable state extensions for wake-ups and run ownership are defined without weakening checkpoint truth
- A controller lifecycle is specified for start, claim, sleep, resume, and shutdown behavior
- Manual pause and override remain explicit operator actions rather than prompt side effects
