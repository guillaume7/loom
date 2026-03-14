# Loom Gap Analysis — "Missing Hands" for Full Autonomy

## 1. Question

Can GitHub MCP or `gh` CLI provide the missing execution "hands" so Loom can operate fully autonomously?

## 2. Short Answer

**Yes, for actuation.** GitHub MCP already exposes enough write operations to execute the core PR/issue workflow.

**Not by itself, for full autonomy.** You still need an always-on runner, deterministic gate policy, and event/timer orchestration around those tools.

## 3. Capability Matrix

| Capability needed by Loom | GitHub MCP | `gh` CLI | Verdict |
|---|---|---|---|
| Create/update/close issues | Yes (`issue_write`, comments) | Yes (`gh issue create/edit/close/comment`) | Sufficient |
| Assign Copilot to issue | Yes (`assign_copilot_to_issue`) | Partial/indirect | MCP preferred |
| Create/update PRs | Yes (`create_pull_request`, `update_pull_request`) | Yes (`gh pr create/edit`) | Sufficient |
| Move PR draft -> ready | Yes (`update_pull_request` with `draft=false`) | Yes (`gh pr ready`) | Sufficient |
| Request Copilot review | Yes (`request_copilot_review`) | Usually via API call/manual | MCP preferred |
| Read reviews/comments/check runs | Yes (`pull_request_read` methods) | Yes (`gh pr view --json ...`) | Sufficient |
| Merge PR | Yes (`merge_pull_request`) | Yes (`gh pr merge`) | Sufficient |
| Update PR branch from base | Yes (`update_pull_request_branch`) | Yes (`gh pr update-branch`) | Sufficient |
| Poll async job status | Yes (`get_copilot_job_status`) | Partial | MCP preferred |

## 4. What Is Still Missing (Beyond Hands)

The blocker is no longer raw GitHub write access. The remaining gaps are orchestration and reliability:

1. **Event/timer runtime**
- A persistent process must wake Loom on webhook events (`pull_request`, `check_suite`, `check_run`, `pull_request_review`) or polling intervals.
- Without this, Loom stalls in waiting states unless manually re-invoked.

2. **Deterministic gate evaluator**
- A strict policy engine must decide: "safe to merge" vs "hold".
- Inputs include required checks, review state, mergeability, draft status, dependency DAG.

3. **Dependency graph source of truth**
- Loom needs machine-readable dependencies (not just prose in issue bodies).
- Recommended: `.loom/dependencies.yaml` keyed by issue/epic IDs.

4. **Idempotency and concurrency control**
- Use operation keys to avoid duplicate comments/review requests/merges after retries.
- Add per-PR lock to prevent two loops acting on the same PR.

5. **Failure-handling policy**
- Define automatic behavior for red CI, flaky checks, merge conflicts, stalled Copilot jobs.
- Include bounded retry counts and escalation comments.

6. **Auth and permission hardening**
- Token/app permissions must include PR write, checks read, issue write, review operations.
- Branch protection must permit the chosen merge strategy.

## 5. Practical Conclusion for VECTOR

For VECTOR's current workflow, **GitHub MCP is sufficient to provide Loom's missing hands**. `gh` CLI can be a fallback actuator but is not required as primary control.

The next implementation milestone is therefore:

- Keep MCP as primary actuator
- Add a durable Loom runtime + gate evaluator + dependency DAG + idempotency
- Optionally wire `gh` CLI as emergency fallback path when MCP calls fail

## 6. Recommended Implementation Order

1. Add machine-readable dependency graph (`.loom/dependencies.yaml`).
2. Implement gate evaluator (`isReadyForReview`, `isCIGreen`, `isApproved`, `isMergeSafe`).
3. Implement idempotent action log in Loom state store.
4. Add webhook/timer driver to wake Loom transitions.
5. Bind transitions to MCP actions (`update_pull_request`, `request_copilot_review`, `merge_pull_request`, `issue_write`).
6. Add fallback adapters for `gh` CLI only where MCP proves unreliable.

## 7. Decision

**Decision:** Use GitHub MCP as the primary autonomous actuator for Loom. Treat `gh` CLI as optional resilience tooling, not the orchestration backbone.
