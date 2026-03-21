# ADR-009: Deterministic Runtime Policy Engine for Gates, Retries, and Escalation

## Status
Accepted

## Context

Loom currently mixes some gate and retry behavior across MCP polling, prompt
instructions, and ad hoc agent reasoning. That makes it difficult to answer a
basic operational question: why did Loom progress, pause, retry, or stop at a
given moment?

VP3 requires merge readiness, CI readiness, review readiness, retry exhaustion,
and escalation outcomes to become deterministic runtime decisions with explicit
inputs and explicit outputs.

### Forces

- **Determinism**: the same observed GitHub state should produce the same Loom
  decision.
- **Auditability**: operators need to inspect why a gate failed or an escalation
  fired.
- **Replayability**: stalled runs must be reproducible from captured inputs.
- **Safety**: merge and escalation decisions must not depend on free-form prompt
  interpretation.

## Decision

Introduce a **runtime policy engine** in Go for orchestration decisions.

1. Policy inputs are explicit snapshots: checkpoint state, relevant GitHub
   state, dependency readiness, lock state, and retry counters.
2. Policy outputs are explicit decision records such as `continue`, `wait`,
   `retry`, `escalate`, `pause`, or `merge_safe`.
3. CI, review, mergeability, dependency readiness, and retry escalation become
   runtime policy evaluations rather than prompt logic.
4. Every material policy outcome is persisted or logged with enough context to
   support trace rendering and replay.
5. MCP Tasks and elicitations become surface mechanisms, not the place where
   policy is decided.

## Consequences

### Positive

- Gate behavior becomes explainable and testable.
- Replay harnesses can assert policy outcomes from recorded inputs.
- The operator can distinguish runtime decisions from agent outputs.

### Negative

- More GitHub state needs to be normalized into a stable decision input model.
- Policy evolution requires disciplined versioning as the runtime grows.

### Risks

- Overfitting the first policy model can make later changes painful if decision
  records are not designed for evolution.
- If policy boundaries are unclear, some logic may still leak back into prompt
  space.

## Alternatives Considered

### Keep gates in agent reasoning

- Pros: Lower upfront implementation effort.
- Cons: Non-deterministic, difficult to audit, and hard to replay.
- Rejected because: VP3 explicitly requires runtime-owned liveness and policy.

### Encode all policies in static configuration only

- Pros: Simplifies testing.
- Cons: Insufficient for dynamic GitHub state and nuanced escalation decisions.
- Rejected because: Loom still needs code-level policy evaluation over runtime
  inputs.