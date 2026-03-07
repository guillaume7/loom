# Skill: Epic and Story Breakdown

How to decompose epics into implementable user stories, and how to break user stories into coding tasks.

---

## Implementation Order

Always respect this sequence:

```
E1 (Foundation)
  ↓
E2 (State Machine) ← needed by everything else
  ↓
E3 (GitHub Client) ← used by MCP server and CLI
  ↓
E4 (MCP Server) ← exposes FSM + GitHub to Copilot session
  ↓
E5 (CLI) ← wraps MCP server + config for human operator
  ↓
E6 (Session Management) ← keepalive, stall detection, restart
  ↓
E7 (Checkpointing) ← SQLite persistence, resume on restart
  ↓
E8 (Integration & Hardening) ← E2E tests, retry budget exercises
```

---

## Epic Structure

Each epic (`docs/epics/E*/epic.md`) contains:

```markdown
# E{N}: {Epic Name}
## Goal
## Description
## User Stories (linked)
## Dependencies
## Acceptance Criteria (checkboxes)
```

---

## User Story Sizing

Stories should be implementable by one agent in one session (roughly: < 200 lines of new Go code, < 8 new tests). Split if:

| Situation | How to Split |
|---|---|
| Story covers multiple FSM states | One story per state or state group |
| Story mixes interface definition + implementation | Define interface first, implement second |
| Story has happy path + many error paths | Core behaviour story + error-hardening story |

No story should have more than 8 acceptance criteria.

---

## Acceptance Criteria Quality Check

Use Given/When/Then format:

```
Given the FSM is in state AWAITING_PR
  And the retry count equals MaxRetries
When the EventTimeout event is received
Then the FSM transitions to PAUSED
  And a PAUSED checkpoint is written to the store
```

---

## Task Breakdown (within a Story)

```
Example for US-2.1 (FSM States):

1. [ ] Define State constants in states.go (write test first)
2. [ ] Define Event constants in events.go (write test first)
3. [ ] Implement transition table in machine.go
4. [ ] Implement retry budget in Machine struct
5. [ ] Implement PAUSED escape hatch
6. [ ] Full test matrix for all valid transitions
7. [ ] Full test matrix for all invalid transitions
8. [ ] Self-review against review-checklist skill
9. [ ] Open PR
```

Tasks are in TDD order: tests first, implementation second.

---

## Dependency Graph

| Epic | Depends On |
|---|---|
| E1 | — |
| E2 | E1 |
| E3 | E1 |
| E4 | E2, E3 |
| E5 | E4 |
| E6 | E4, E5 |
| E7 | E4 |
| E8 | E2, E3, E4, E5, E6, E7 |
