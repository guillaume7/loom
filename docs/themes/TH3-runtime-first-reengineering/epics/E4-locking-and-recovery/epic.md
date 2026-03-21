# E4 — Locking and Recovery

> Theme: [TH3 — Runtime-First Reengineering](../../README.md)
> ADR: [ADR-010](../../../../ADRs/ADR-010-bounded-agent-jobs-and-run-locking.md)
> Priority: P1

## Goal

Prevent concurrent run corruption by defining runtime leases, PR-scoped locks,
lease-expiry recovery, and conflict handling for resumed work.

## Dependencies

- **E2** (Wake-Up and Resumption) — deduplicated resume behavior
- **E3** (Deterministic Policy Engine) — explicit policy outcomes and auditability

## Stories

| Story | Title | Size | Depends On |
| ----- | ----- | ---- | ---------- |
| US1 | Runtime lease and ownership claims | M | TH3.E1.US2 |
| US2 | PR-scoped locking | M | US1 |
| US3 | Lease expiry and recovery | M | US1, TH3.E2.US4 |
| US4 | Resume conflict handling | M | US2, US3, TH3.E3.US4 |

## Acceptance

Epic is done when:

- Loom can determine who owns active runtime work
- PR- or run-scoped operations cannot be executed concurrently by multiple controllers
- Expired or abandoned ownership can be recovered safely
- Resume conflicts are surfaced deterministically and without repeated side effects
