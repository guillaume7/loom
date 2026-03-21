---
name: "E3 — Deterministic Policy Engine"
about: Move gate and escalation logic into deterministic runtime policy evaluation.
title: "E3: Deterministic Policy Engine"
labels: ["epic", "E3", "TH3"]
---

## Goal

Move gate, retry, escalation, and merge decisions into deterministic runtime policy evaluation based on persisted observations.

## User Stories

- [ ] TH3.E3.US1 — Runtime observation model
- [ ] TH3.E3.US2 — CI review and merge policy decisions
- [ ] TH3.E3.US3 — Escalation and wait outcome taxonomy
- [ ] TH3.E3.US4 — Policy evaluation audit trail

## Acceptance Criteria

- [ ] Policy decisions consume typed persisted observations
- [ ] CI, review, and merge gates produce explicit runtime outcomes
- [ ] Wait, retry, block, and escalate semantics are fixed and auditable
- [ ] Operators can inspect why a decision was produced
