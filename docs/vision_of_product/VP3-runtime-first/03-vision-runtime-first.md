# Loom — Vision Phase 3: Runtime-First Autonomous Operations

## Phase Summary

Phase 3 contains the Loom reengineering program. Phase 1 and Phase 2 remain settled historical product artifacts; the redesign work now lives entirely inside VP3. This phase re-centers Loom around a durable runtime that can keep making forward progress without depending on a long-lived chat session. Agents remain important, but they become bounded workers inside Loom's operating model rather than the control plane itself.

## Problem Statement

The current ceiling on productivity is not a lack of agent intelligence. It is the absence of a strong runtime boundary. When waits, retries, and wake-ups depend on an active session, the system stalls at exactly the moments where autonomy matters most. CI gates, review gates, flaky jobs, and delayed GitHub events become session-management problems instead of orchestration problems.

Phase 3 solves the third problem: operational liveness. Loom must move timers, wake-ups, locking, policy evaluation, and recovery into a durable runtime so that agents can focus on bounded reasoning tasks while Loom continues advancing safely in the background. All three reengineering tracks for Loom belong here rather than retroactively rewriting earlier vision phases.

## Target Users

- Builders who want Loom to keep operating while they are away from the editor
- Teams that need forward progress across long CI or review delays without manual re-invocation
- Operators who need post-mortem visibility, replayability, and predictable failure handling before trusting higher autonomy

## Core Features

- A single umbrella vision for Loom reengineering, with sub-phases captured as Phase 3 planning rather than rewritten VP1 or VP2 artifacts
- A durable runtime loop or daemonized controller that owns polling, wake-ups, retries, and transition scheduling
- Event-driven resumption using GitHub signals and scheduled checks instead of relying on a chat session heartbeat
- Deterministic gate evaluation in the runtime for CI status, review state, merge safety, and dependency readiness
- Per-PR or per-run locking and stronger concurrency control for parallel or resumed execution paths
- Replayable traces and fixture-driven simulation so stalled runs can be reproduced locally from recorded events
- Agent invocation model based on bounded jobs such as drafting an issue, summarizing a failed CI run, or composing a review response

## Success Criteria

- Loom can continue progressing through waiting states without requiring a manually resumed interactive session
- A stalled or failed run can be replayed locally from recorded inputs to explain exactly why a transition did or did not occur
- The runtime can decide merge readiness and failure escalation through explicit policy rather than prompt interpretation
- Agents are invoked for bounded tasks with clear inputs and outputs, and a failed agent invocation does not corrupt orchestration state
- Operators can distinguish runtime failure, GitHub failure, and agent-task failure from observability artifacts alone

## Constraints

- Runtime logic must remain deterministic, testable, and recoverable from persisted state
- Agent calls must be optional, retryable, and side-effect-bounded
- The product must still support local development and debugging without requiring a hosted control plane
- Any background execution model must preserve safe pause and manual override paths
- The runtime-first model must coexist with earlier phases long enough to allow incremental migration

## Non-Goals

- Full removal of agents from the product
- Blind autonomy that sacrifices operator visibility or safe pause boundaries
- Immediate support for large-scale distributed orchestration across many repositories
- Replacing GitHub with a generic workflow backend in this phase

## Open Questions

- Should the durable runtime be a long-running local daemon, a resumable job runner, or both?
- What exact event sources should wake Loom: polling only, webhooks, GitHub Actions callbacks, or a hybrid model?
- Which reasoning tasks still justify an agent invocation once policy, gating, and retries are owned by the runtime?
