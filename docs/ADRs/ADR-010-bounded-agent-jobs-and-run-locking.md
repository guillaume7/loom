# ADR-010: Bounded Agent Jobs and Run Locking

## Status
Accepted

## Context

VP2 made agents more capable and more central. VP3 keeps agents, but narrows
their role. The system still benefits from AI for drafting, summarization, and
response composition, but those tasks must not control orchestration liveness.

At the same time, runtime-first orchestration requires stronger concurrency
control. Multiple loops or resumes must not operate on the same PR or run at the
same time.

### Forces

- **Containment**: a failed agent invocation must not corrupt workflow state.
- **Clarity**: inputs and outputs for AI work must be explicit and inspectable.
- **Concurrency safety**: one runtime owner per run or PR at a time.
- **Migration**: existing agent assets should remain useful while their role is
  narrowed.

## Decision

Adopt a **bounded job contract** for agents and a **runtime lock model**.

1. Agents are invoked only for bounded jobs with explicit structured inputs and
   outputs.
2. Typical bounded jobs include drafting an issue body, summarizing failing CI,
   composing a review response, or generating operator-facing summaries.
3. The runtime acquires a run lease before progressing a workflow and uses
   narrower locks, such as per-PR locks, when needed.
4. Agent failures are treated as isolated job failures. They may trigger retry,
   fallback, or escalation, but do not directly mutate authoritative runtime
   state.
5. Parallel execution remains optional and subordinate to the locking model.

## Consequences

### Positive

- Agent work becomes easier to retry, replace, or stub in tests.
- Concurrency rules become explicit rather than implicit in background sessions.
- Runtime-first orchestration and agent usefulness can coexist cleanly.

### Negative

- Some previously agent-driven workflow patterns will need to be rewritten.
- Structured job contracts add design work before implementation speed-ups.

### Risks

- If job contracts are too loose, agents will continue to leak orchestration
  responsibility.
- If locks are too coarse, Loom can become safe but unnecessarily slow.

## Alternatives Considered

### Keep multi-agent orchestration as the primary workflow engine

- Pros: Reuses more VP2 work directly.
- Cons: Leaves orchestration liveness distributed across agent sessions.
- Rejected because: VP3 requires runtime-owned control.

### Remove agents from the product entirely

- Pros: Maximum determinism.
- Cons: Loses valuable bounded reasoning capabilities for summarization and
  authoring tasks.
- Rejected because: VP3 narrows agent scope; it does not eliminate agents.