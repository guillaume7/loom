---
name: backlog-management
description: 'Backlog YAML format, status state machine, dependency resolution, sequencing rules, state persistence. Use when: reading backlog, updating story status, resolving dependencies, managing sequencing, parsing backlog YAML.'
---

# Backlog Management Skill

## Core State File

`docs/plan/backlog.yaml` is the **single source of truth** for the orchestrator. It is a pure YAML file (not markdown with YAML frontmatter) containing the full dependency graph and status of every theme, epic, and story.

## YAML Schema

The backlog is a standalone `.yaml` file with no markdown wrapper:

```yaml
# Backlog — Core State File
backlog:
  project: "<project-name>"
  last-updated: "<ISO 8601 timestamp>"
  themes:
    - id: TH<n>
      name: "<theme name>"
      status: todo               # todo | in-progress | done
      vision-ref: docs/vision_of_product/VP<n>-<slug>/   # string or list of VP paths
      depends-on: []             # theme-level dependencies (cross-theme)
      epics:
        - id: E<m>
          name: "<epic name>"
          status: todo           # todo | in-progress | done
          depends-on: []         # epic-level dependencies [E1, TH1.E2]
          stories:
            - id: TH<n>.E<m>.US<l>
              title: "<title>"
              status: todo       # todo | in-progress | done | failed
              file: docs/themes/TH<n>-<slug>/epics/E<m>-<slug>/stories/US<l>-<slug>.md
              depends-on: []     # story-level dependencies [TH1.E1.US1, TH1.E1.US2]
```

## Status Values

| Status | Meaning | Transitions to |
|:---|:---|:---|
| `todo` | Not started, eligible if dependencies met | `in-progress` |
| `in-progress` | Currently being worked on | `done`, `failed` |
| `done` | Completed and verified | (terminal) |
| `failed` | Failed, needs troubleshooting | `in-progress` |

## Dependency Resolution Rules

### Story Eligibility
A story is **eligible** when:
1. Its epic's dependencies are all `done`
2. Its own `depends-on` stories are all `done`
3. Its status is `todo`

### Epic Eligibility
An epic is **eligible** when:
1. Its theme's dependencies are all `done` (if cross-theme deps exist)
2. Its own `depends-on` epics are all `done`
3. It has at least one `todo` story

### Parallel Execution
- Epics with no mutual dependencies MAY be processed in parallel (interleaved story-by-story)
- Stories within the same epic are processed in order (US1 before US2) unless dependencies explicitly allow otherwise

### Priority-Based Selection
When multiple stories are eligible simultaneously, prefer **higher priority** first:
- `high` > `medium` > `low`
- Default priority (when field is omitted) is `medium`
- Within the same priority level, maintain story order (US1 before US2)

## State Update Protocol

The backlog file is the **sole authoritative store** for status. Story files do NOT contain a `status` field — status lives only in the backlog YAML.

1. **Read** backlog.yaml before every decision
2. **Modify** the relevant status field
3. **Update** `last-updated` timestamp
4. **Write** backlog.yaml atomically

## Session Log

The session log (`docs/plan/session-log.md`) provides a bounded, recent-activity view for crash recovery and resumability. **Git commit history is the primary audit trail** — the session log is supplementary.

After each state change, append a line to `docs/plan/session-log.md`:

```markdown
### <ISO timestamp>
- <action>: <entity id> → <new status>
- Context: <brief description>
```

### Bounded Size

- Keep only the **last 50 entries**
- When appending, prune entries beyond 50 (oldest first)
- This prevents unbounded growth while retaining enough context for recovery

## Crash Recovery Protocol

When the orchestrator starts and finds a story with status `in-progress` in the backlog:

1. **Detect** — scan backlog.yaml for any stories with `status: in-progress`
2. **Assess** — check for partial work:
   - Are there new/modified files related to the story?
   - Was a commit created for the story?
   - Is the session-log showing partial progress?
3. **Decide**:
   - **Continue** — if partial work is valid (files exist, tests can be run), resume from where it left off
   - **Reset** — if no meaningful work exists, reset status to `todo` and restart
   - **Escalate** — if assessment is inconclusive, present the situation to the user and wait for guidance

## Qualified ID Convention

All entity IDs use **fully-qualified dot-notation** to avoid ambiguity across themes and epics:

| Level | Format | Example |
|:------|:-------|:--------|
| Theme | `TH<n>` | `TH1` |
| Epic | `TH<n>.E<m>` | `TH1.E2` |
| Story | `TH<n>.E<m>.US<l>` | `TH1.E1.US1` |

- Story `id` fields in backlog YAML use the full `TH<n>.E<m>.US<l>` format
- `depends-on` references at story level use qualified IDs (e.g., `[TH1.E1.US1, TH1.E2.US3]`)
- Epic `depends-on` may use `TH<n>.E<m>` for cross-theme references (e.g., `[TH1.E1]`)
- This prevents collisions when multiple epics each contain a `US1`
