# E2 — Wake-Up and Resumption

> Theme: [TH3 — Runtime-First Reengineering](../../README.md)
> ADR: [ADR-008](../../../../ADRs/ADR-008-runtime-first-control-plane-and-wake-model.md)
> Priority: P0

## Goal

Make waiting states durable by introducing a runtime wake queue, polling-based
resumptions, optional GitHub event adapters, and deduplicated resume behavior.

## Dependencies

- **E1** (Runtime Kernel Foundation) — controller lifecycle and persisted runtime model

## Stories

| Story | Title | Size | Depends On |
| ----- | ----- | ---- | ---------- |
| US1 | Wake-up queue and timers | M | TH3.E1.US2 |
| US2 | Poll-driven resumptions | M | US1 |
| US3 | GitHub event adapters | M | US1 |
| US4 | Resume deduplication | M | US2, US3 |

## Acceptance

Epic is done when:

- Loom can persist wake-up intent independently of a live session
- Polling-based resumption works as the guaranteed baseline
- Event-driven resumptions can be added without changing checkpoint truth
- Duplicate resumes against the same run or PR are detected and suppressed safely
