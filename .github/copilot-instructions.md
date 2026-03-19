# Loom + Copilot Autopilot — Workspace Instructions

## Project Purpose

Loom uses the **Copilot Autopilot process conventions** as its planning and implementation framework, with an added Loom-operated GitHub execution mode.

- **Local loop**: story-level autonomous implementation inside the repository (`/run-autopilot`)
- **Server-side loop**: Loom MCP operator drives GitHub PRs human-out-of-loop (`/run-loom`)

## Key Entry Points

| Phase | Prompt | Output |
|-------|--------|--------|
| 1. Vision | `/kickstart-vision` | `docs/vision_of_product/VP<n>/` |
| 2. Architecture | `/plan-product` | `docs/architecture/` + `docs/ADRs/` |
| 3. Planning | `/plan-product` | `docs/themes/TH<n>/` + `docs/plan/backlog.yaml` |
| 4A. Local Autopilot | `/run-autopilot` | Local implement → test → review loop |
| 4B. Loom Weaving | `/run-loom` | Loom MCP loop weaving server-side PRs to completion |

## Core State

- `docs/plan/backlog.yaml` — **single source of truth** for local autopilot orchestration state (pure YAML)
- `docs/plan/session-log.md` — session history for resumability
- Loom FSM checkpoints (SQLite + MCP tools) — source of truth for server-side PR weaving state
- Loom session traces (SQLite + MCP resources) — append-only audit artifacts for `/run-loom` analysis; never the authoritative workflow state

## Agent Squad

| Agent | Phase | Role |
|-------|-------|------|
| **product-owner** | 3 | Vision → themes/epics/stories + backlog |
| **architect** | 2 | Vision → architecture + ADRs |
| **orchestrator** | 4A | Local autopilot loop: sequencing, state management |
| **developer** | 4A | Implements + tests one user story |
| **reviewer** | 4A | Code review: correctness, security, conventions |
| **troubleshooter** | 4A | Diagnoses + fixes failed stories |
| **loom-mcp-operator** | 4B | Uses Loom MCP tools to drive GitHub issue/PR lifecycle until complete |

## Skills Reference

Each topic below is owned by exactly one skill. See the skill for canonical details.

| Topic | Skill | Covers |
|-------|-------|--------|
| Lifecycle & conventions | `the-copilot-build-method` | 4-phase lifecycle, VP↔TH mapping, directory conventions, naming conventions, Definition of Done, agent roles |
| Story format | `bdd-stories` | Frontmatter schema, As-a/I-want/So-that, acceptance criteria, BDD scenarios |
| Backlog format | `backlog-management` | YAML schema, status state machine, dependency resolution, sequencing rules |
| Code review | `code-quality` | Review checklist, OWASP security audit |
| Architecture | `architecture-decisions` | ADR format, tech stack analysis, component boundaries |
| Loom PR weaving | `loom-mcp-loop` | Canonical loom_next_step → GitHub action → loom_checkpoint loop |

## Anti-Patterns

- **Never hardcode state in agent memory** — always read/write `docs/plan/backlog.yaml` for local autopilot, and always ask Loom for next state in server-side mode
- **Never treat a session trace as authoritative state** — the FSM checkpoint and SQLite store remain the source of truth; traces are observability artifacts
- **Never skip the troubleshooter** — failed stories must be fixed before epic completion
- **Never modify vision docs during Phase 4** — vision is frozen for the theme currently in execution (future VPs can be amended at user checkpoints)
- **Never implement multiple stories in one agent session** — 1 story = 1 developer call
- **Never skip the code quality review at epic end** — technical debt compounds
- **Never have the length of code files exceed 500 lines** — break down into subcomponents
