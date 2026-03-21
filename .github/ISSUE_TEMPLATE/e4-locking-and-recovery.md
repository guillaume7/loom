---
name: "E4 — Locking and Recovery"
about: Prevent concurrent run corruption with leases, PR locks, and recovery rules.
title: "E4: Locking and Recovery"
labels: ["epic", "E4", "TH3"]
---

## Goal

Prevent concurrent run corruption by defining runtime leases, PR-scoped locks, lease-expiry recovery, and explicit resume conflict handling.

## User Stories

- [ ] TH3.E4.US1 — Runtime lease and ownership claims
- [ ] TH3.E4.US2 — PR-scoped locking
- [ ] TH3.E4.US3 — Lease expiry and recovery
- [ ] TH3.E4.US4 — Resume conflict handling

## Acceptance Criteria

- [ ] Active run ownership is explicit
- [ ] PR mutations are serialized safely
- [ ] Expired ownership can be recovered without duplicating side effects
- [ ] Resume conflicts are surfaced deterministically and audibly
