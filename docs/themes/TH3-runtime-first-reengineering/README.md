# TH3 — Runtime-First Reengineering

> Vision Reference: [VP3 — Runtime-First Autonomous Operations](../../vision_of_product/VP3-runtime-first/03-vision-runtime-first.md)

## Summary

TH3 captures Loom's runtime-first reengineering program. Unlike TH1 and TH2,
which are settled historical planning artifacts, TH3 is the active planning line
for moving orchestration liveness into the Go runtime while preserving agents as
bounded workers.

## Epics

| Epic | Name | Priority | Description |
| ----- | ----- | -------- | ----------- |
| E1 | Runtime Kernel Foundation | P0 | Runtime mode choice, persisted run model, controller lifecycle, operator controls |
| E2 | Wake-Up and Resumption | P0 | Wake queue, polling resumptions, GitHub event adapters, resume deduplication |
| E3 | Deterministic Policy Engine | P0 | Runtime policy inputs, CI/review/merge safety, escalation outcomes |
| E4 | Locking and Recovery | P1 | Run leases, per-PR locks, lease expiry recovery, resume conflict handling |
| E5 | Replay and Bounded Agent Jobs | P1 | Replay fixtures, bounded job contracts, agent failure containment |

## Dependency Graph

```text
E1 (runtime kernel) ──┬──→ E2 (wake-up and resumption)
                      ├──→ E3 (policy engine)
                      └──→ E4 (locking and recovery)

E2 + E3 + E4 ──→ E5 (replay and bounded agent jobs)
```

## Implementation Waves

- **Wave 1**: E1
- **Wave 2**: E2 and E3 in parallel
- **Wave 3**: E4
- **Wave 4**: E5

## Planning Note

TH3 is intentionally separate from TH1 and TH2. It carries the VP3 redesign as
new planning work instead of rewriting settled theme history.
