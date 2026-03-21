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

## Artifact Immutability

- Treat settled vision phases in `docs/vision_of_product/` as immutable product history.
- Treat accepted ADRs in `docs/ADRs/` as immutable in substance; architecture changes must create new ADRs and supersede old ones.
- Treat accepted themes in `docs/themes/` as immutable in meaning; if scope changes materially, create new themes, epics, or stories rather than repurposing existing ones.
- Limit edits to settled artifacts to non-semantic corrections, append-only notes, or explicit supersession/status metadata.

## Pipeline

### Step 1 — Architecture
Invoke the @architect agent to analyze `docs/vision_of_product/` and produce:
- `docs/architecture/` — system design, tech stack, components
- `docs/ADRs/` — architecture decision records
- `.github/copilot-instructions.md` — updated workspace instructions with architecture details and code conventions (DRY, naming, formatting) and best practices for the project, TDD, clean code, keep files <500 lines, etc.

Architecture output must preserve accepted ADR history. If a prior decision is no longer correct, create a new ADR and mark the old one superseded or deprecated rather than rewriting it.

### Step 2 — User Stories
Invoke the @product-owner agent to break the vision + architecture into:
- `docs/themes/TH<n>-<name>/` — theme/epic/story hierarchy
- `docs/plan/backlog.yaml` — YAML dependency graph with all stories

Planning output must preserve accepted theme identity. If new scope no longer belongs in a settled theme, add a new theme, epic, or story instead of rewriting the old one.

### Step 3 — GitHub Issue Templates

After Step 2 completes, sync `.github/ISSUE_TEMPLATE/` with the current epic set:

**Generate / update epic templates**

For every epic discovered under `docs/themes/TH<n>-*/epics/E<m>-*/`, create or overwrite `.github/ISSUE_TEMPLATE/<epic-slug>.md` (e.g. `e1-project-foundation.md`) using this structure:

```markdown
---
name: "E<m> — <Epic Title>"
about: <one-sentence goal from epic.md>
title: "E<m>: <Epic Title>"
labels: ["epic", "E<m>", "TH<n>"]
---

## Goal

<goal paragraph from epic.md>

## User Stories

<checkbox list of story IDs and titles from the epic's user-stories/ directory>

## Acceptance Criteria

<acceptance criteria from epic.md, as checkboxes>
```

**Archive stale epic templates**

1. List all files in `.github/ISSUE_TEMPLATE/` whose names match the pattern `e[0-9]*.md`.
2. For any file that does NOT correspond to a current epic (no matching `docs/themes/` directory), move it to `.github/ISSUE_TEMPLATE/archive/`.
3. Never touch utility templates: `debugging.md`, `implementation-status.md`, `refactoring.md`, `config.yml`.

After all three steps complete, display a summary of:
- Number of themes, epics, and stories created
- Issue templates created/updated and any archived
- Dependency graph overview
- Estimated implementation order
