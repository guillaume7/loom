# Release: Runtime-First Reengineering (TH3)

## Summary
TH3 delivers the runtime-first control plane for Loom. Runtime state, wake scheduling, policy decisions, lock ownership, recovery, replay fixtures, and bounded worker-job containment now execute through deterministic persisted flows owned by the runtime.

## Epics Delivered
- TH3.E1 Runtime Kernel Foundation
- TH3.E2 Wake-Up and Resumption
- TH3.E3 Deterministic Policy Engine
- TH3.E4 Locking and Recovery
- TH3.E5 Replay and Bounded Agent Jobs

## Breaking Changes
- Runtime policy verdict storage now uses explicit policy outcomes (for example continue/wait/block/escalate) in persisted policy decision records.
- Runtime lease and resume behavior now enforce explicit ownership and conflict handling before resume progression.

## Migration Notes
- Existing persisted stores should run current migrations before operating with TH3 runtime behavior.
- Operators should treat replay fixtures and policy audit trails as the canonical debugging path for runtime branch divergence.
- Background agent execution is now bounded by explicit job contracts (job id, attempt, deadline, expected output) and failure outcomes are recorded explicitly for retry/escalation/block handling.
