---
description: "Transform product vision into architecture and implementation plan. Runs architect then product-owner agents sequentially. Use when: planning implementation, generating backlog, creating architecture from vision."
agent: "agent"
tools: [read, edit, search, agent, todo, execute, web]
---

## Agents & Skills

| Agent | Skills |
|-------|--------|
| @architect | `the-copilot-build-method`, `architecture-decisions` |
| @product-owner | `the-copilot-build-method`, `bdd-stories`, `backlog-management` |

Execute the planning pipeline to transform vision into an actionable backlog.

## Pipeline

### Step 1 — Architecture
Invoke the @architect agent to analyze `docs/vision_of_product/` and produce:
- `docs/architecture/` — system design, tech stack, components
- `docs/ADRs/` — architecture decision records
- `.github/copilot-instructions.md` — updated workspace instructions with architecture details and code conventions (DRY, naming, formatting) and best practices for the project, TDD, clean code, keep files <500 lines, etc.

### Step 2 — User Stories
Invoke the @product-owner agent to break the vision + architecture into:
- `docs/themes/TH<n>-<name>/` — theme/epic/story hierarchy
- `docs/plan/backlog.yaml` — YAML dependency graph with all stories

After both steps complete, display a summary of:
- Number of themes, epics, and stories created
- Dependency graph overview
- Estimated implementation order
