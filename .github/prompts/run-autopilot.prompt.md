---
description: "Run the local autopilot loop to execute the backlog autonomously in this repository. Use when: running /run-autopilot locally, starting autonomous development, executing backlog."
agent: "orchestrator"
tools: [read, edit, search, execute, agent, todo]
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

## Pre-flight Checks

Before starting the loop, verify:
1. `docs/plan/backlog.yaml` exists and contains valid YAML with at least one theme
2. `docs/architecture/` exists with tech stack and component definitions
3. `docs/themes/` contains at least one theme with epics and stories
4. Check `docs/plan/backlog.yaml` for any `in-progress` stories — if found, trigger crash recovery (assess and continue, reset, or escalate)

## Execution

Start the local autopilot loop as defined in your orchestrator instructions. Process stories in dependency order, running the full cycle (implement → test → review) for each.

Report progress after each story completion.
