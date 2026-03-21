---
name: "run-autopilot"
description: "Run the local autopilot loop to execute the backlog autonomously in this repository. Use when: running /run-autopilot locally, starting autonomous development, executing backlog."
agent: "Autopilot Orchestrator"
tools: [read, edit, search, execute, agent, todo]
argument-hint: "Optional: theme, epic, or story to prioritize"
---

## Agents & Skills

| Agent | Skills |
|-------|--------|
| @orchestrator | `the-copilot-build-method`, `backlog-management` |
| @developer | `the-copilot-build-method`, `bdd-stories` |
| @reviewer | `the-copilot-build-method`, `code-quality` |
| @troubleshooter | `the-copilot-build-method`, `bdd-stories`, `code-quality` |
| @product-owner | `the-copilot-build-method`, `bdd-stories`, `backlog-management` |

Begin autonomous local execution of the product backlog.

## Artifact Immutability

- During execution, treat settled vision artifacts in `docs/vision_of_product/`, accepted ADRs in `docs/ADRs/`, and accepted theme scope in `docs/themes/` as immutable.
- Expected execution-time changes are limited to backlog status, session log, changelog, release notes, implementation code, tests, and other delivery artifacts.
- If implementation reveals changed understanding, stop and route that change into a new planning artifact rather than rewriting settled history.

## Pre-flight Checks

Before starting the loop, verify:
1. `docs/plan/backlog.yaml` exists and contains valid YAML with at least one theme
2. `docs/architecture/` exists with tech stack and component definitions
3. `docs/themes/` contains at least one theme with epics and stories
4. Check `docs/plan/backlog.yaml` for any `in-progress` stories — if found, trigger crash recovery (assess and continue, reset, or escalate)
5. If the user supplied a theme, epic, or story target, treat it as a prioritization hint rather than permission to skip dependency checks

## Execution

Start the local autopilot loop as defined in your orchestrator instructions.

Process stories in dependency order and run the full cycle for each story:
1. select the next eligible story from `docs/plan/backlog.yaml`
2. implement and test via `@developer`
3. review via `@reviewer`
4. if build/test verification fails, route through `@troubleshooter`
5. write status changes back to `docs/plan/backlog.yaml` and append progress to `docs/plan/session-log.md`

Stop when:
- all themes are complete
- you reach a user checkpoint at theme completion
- or you hit a blocker that requires human input

Report progress after each story completion and include any blocker or checkpoint reason explicitly.
