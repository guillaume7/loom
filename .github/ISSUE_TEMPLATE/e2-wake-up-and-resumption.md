---
name: "E2 — Wake-Up and Resumption"
about: Make waiting states durable through wake queues, polling, event adapters, and resume deduplication.
title: "E2: Wake-Up and Resumption"
labels: ["epic", "E2", "TH3"]
---

## Goal

Make waiting states durable by introducing a runtime wake queue, polling-based resumptions, optional GitHub event adapters, and deduplicated resume behavior.

## User Stories

- [ ] TH3.E2.US1 — Wake-up queue and timers
- [ ] TH3.E2.US2 — Poll-driven resumptions
- [ ] TH3.E2.US3 — GitHub event adapters
- [ ] TH3.E2.US4 — Resume deduplication

## Acceptance Criteria

- [ ] Wake-up intent persists independently of live sessions
- [ ] Polling is the guaranteed baseline resume path
- [ ] Event adapters remain additive to polling and checkpoint truth
- [ ] Duplicate resumes are safely suppressed
