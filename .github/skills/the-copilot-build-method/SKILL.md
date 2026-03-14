---
name: the-copilot-build-method
description: 'The overarching autonomous product development methodology. Covers the 4-phase lifecycle (Vision → Architecture → Planning → Autopilot), VP↔TH mapping, Directory conventions, Definition of Done, agent squad roles, and lifecycle ceremonies. Use when: understanding the methodology, onboarding to the process, checking conventions, verifying Definition of Done.'
---

# The Copilot Build Method

An autonomous product development methodology powered by a squad of specialized AI agents operating through a structured lifecycle.

## Philosophy

- **Vision-first**: Products start as free-form ideas, not code
- **Architecture before implementation**: Design decisions are documented before a single line of code
- **BDD-driven**: Every feature is specified as testable scenarios before implementation
- **Incremental delivery**: Products are built in vision phases (VP<n>) that map to implementation themes (TH<n>), with 1:N mapping for large VPs
- **Autonomous execution**: The orchestrator agent loops the squad through implement → test → review cycles
- **Persistent state**: All progress is tracked in `docs/plan/backlog.yaml` for resumability
- **Ceremony at boundaries**: Epic and theme completions trigger quality gates (integration tests, refactor, release notes)

## The 4 Phases

### Phase 1 — Vision Design (Human + AI)
- Prompt: `/kickstart-vision`
- Output: `docs/vision_of_product/VP<n>-<name>/`
- Free-form brainstorming canvas — no rigid structure
- Each VP<n> maps 1:1 to a theme TH<n>

### Phase 2 — Architecture (Architect Agent)
- Prompt: `/plan-product` (step 1)
- Output: `docs/architecture/` + `docs/ADRs/`
- System design, tech stack selection, component boundaries
- Every significant decision recorded as an ADR

### Phase 3 — Planning (Product Owner Agent)
- Prompt: `/plan-product` (step 2)
- Output: `docs/themes/TH<n>/` + `docs/plan/backlog.yaml`
- Vision decomposed into themes → epics → user stories
- Stories are hybrid BDD (acceptance criteria + Given/When/Then)
- Backlog YAML is the dependency graph + status state machine

### Phase 4 — Autopilot Execution (Orchestrator Agent)
- Prompt: `/run-autopilot`
- Loop: implement → test → review per story
- Epic end ceremony: integration tests + refactor + review + changelog
- Theme end ceremony: regression tests + release readiness + release notes + vision revalidation
- Failed stories: troubleshooter loop (max 3 attempts, then escalate)

## VP ↔ TH Mapping Convention

One vision phase can produce **one or more** themes (1:N). Theme numbering is sequential and independent of VP numbering.

| Vision Phase | Theme(s) | Relationship |
|:---|:---|:---|
| `VP1-mvp/` | `TH1-<name>/` | 1:1 (simple case) |
| `VP1-mvp/` | `TH1-<name>/`, `TH2-<name>/` | 1:N (large vision phase) |
| `VP2-<feat>/` | `TH3-<name>/` | Sequential numbering continues |

## Definition of Done

### Story Done
1. Code compiles / lints clean
2. All BDD scenario tests pass (if applicable — trivial/spike stories may have fewer or no BDD tests)
3. All acceptance criteria verified
4. Build artifacts produce successfully
5. Code review agent approves (trivial stories: lightweight self-review only, skip full reviewer)
6. Relevant documentation updated

### Epic Done (Story DoD + ceremony)

Ceremony scales with epic size:

**Small epic (≤3 stories)**:
1. All stories `done`
2. Run full test suite across epic stories
3. Brief changelog entry

**Large epic (4+ stories)**:
1. All stories `done`
2. Integration test suite passes across all epic stories
3. Reviewer performs lightweight code quality check
4. Orchestrator generates full epic changelog entry

### Theme Done (Epic DoD + ceremony)
1. All epics `done`
2. Full test suite passes (all tests across all epics)
3. Release readiness: artifacts build, docs complete, no `failed` stories
4. If `docs/architecture/deployment.md` exists, verify deployment readiness (CI/CD, health checks, rollback)
5. If vision includes NFRs (performance, scalability targets), verify they are covered by test results
6. Orchestrator produces theme release notes
7. Product-owner revalidates theme against `docs/vision_of_product/VP<n>/`
8. **User checkpoint**: orchestrator pauses and presents a demo summary to the user:
   - User can **accept** (proceed to next theme), **reject** (rework), or **amend** vision for next VP
   - Vision is frozen only for the theme currently in execution — future VPs can be updated at checkpoints

## Naming Conventions

| Entity | Pattern | Example |
|:---|:---|:---|
| Vision Phase | `VP<n>-<slug>/` | `VP1-mvp/` |
| Theme | `TH<n>-<slug>/` | `TH1-core-platform/` |
| Epic | `E<m>-<slug>/` | `E1-user-auth/` |
| User Story | `US<l>-<slug>.md` | `US1-login-form.md` |
| ADR | `ADR-<NNN>-<slug>.md` | `ADR-001-database-choice.md` |

## Agent Squad Roles

| Agent | Phase | Responsibility |
|:---|:---|:---|
| orchestrator | 4 | Autopilot loop, sequencing, state management |
| product-owner | 3 | Vision → themes/epics/stories + backlog |
| architect | 2 | Vision → architecture + ADRs |
| developer | 4 | Implements + tests one user story per session |
| reviewer | 4 | Code review: correctness, security, conventions |
| troubleshooter | 4 | Diagnoses + fixes failed stories |

## Anti-Patterns

- Never hardcode state in agent memory — read/write `docs/plan/backlog.yaml`
- Never skip the troubleshooter — failed stories must be fixed before epic completion
- Never modify vision docs during Phase 4 for the **theme currently in execution** — future VPs can be amended at user checkpoints
- Never implement multiple stories in one agent session
- Never skip the code quality review at epic end
