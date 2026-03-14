---
name: bdd-stories
description: 'Hybrid BDD user story conventions: story format, acceptance criteria writing, Given/When/Then scenario patterns, story sizing, edge case coverage, frontmatter schema. Use when: writing user stories, creating BDD scenarios, defining acceptance criteria, parsing story files.'
---

# BDD Stories Skill

## User Story File Format

Every story file lives at `docs/themes/TH<n>-<slug>/epics/E<m>-<slug>/stories/US<l>-<slug>.md`.

### Required Frontmatter

```yaml
---
id: TH<n>.E<m>.US<l>
title: "<story title>"
type: standard            # standard | trivial | spike
priority: medium          # (optional) high | medium | low — default: medium
size: M                   # (optional) S | M | L — estimated complexity
agents: [developer]             # agents assigned to this story
skills: [bdd-stories]           # skills the agents should load
acceptance-criteria:
  - AC1: "<criterion>"
  - AC2: "<criterion>"
depends-on: []                  # qualified story IDs (e.g., [TH1.E1.US1, TH1.E2.US3])
---
```

### Story Body Structure

```markdown
# TH<n>.E<m>.US<l> — <Title>

**As a** <role>, **I want** <goal>, **so that** <benefit>.

## Acceptance Criteria

- [ ] AC1: <criterion>
- [ ] AC2: <criterion>

## BDD Scenarios

### Scenario: <happy path name>
- **Given** <initial context>
- **When** <action taken>
- **Then** <expected outcome>

### Scenario: <edge case name>
- **Given** <context>
- **When** <action>
- **Then** <outcome>

### Scenario: <error case name>
- **Given** <context>
- **When** <action>
- **Then** <error handling outcome>
```

## Writing Good Acceptance Criteria

- Each AC must be **independently verifiable** — a test can prove it passes or fails
- Use concrete values, not vague language: "responds within 200ms" not "responds quickly"
- Include boundary conditions: "accepts passwords 8-128 characters"
- Separate functional from non-functional: AC for behavior, AC for performance/security

### Non-Functional Requirements (NFRs) as ACs

When the vision specifies performance, scalability, or reliability targets, express them as concrete, testable acceptance criteria:

| NFR Category | Example AC |
|:---|:---|
| Performance | "API responds within 200ms at p95 under 100 concurrent users" |
| Scalability | "System handles 10k records without degradation" |
| Reliability | "Service recovers within 30s after dependency failure" |
| Security | "All endpoints require authentication; unauthenticated requests return 401" |

NFR acceptance criteria should include:
- **Measurable threshold** (200ms, 99.9%, 10k records)
- **Load/condition** (100 concurrent users, peak traffic)
- **Measurement method** (p95 latency, error rate, recovery time)

## Writing Good BDD Scenarios

### Happy Path (required)
The main success flow. Always write this first.

### Edge Cases (required)
- Boundary values (empty input, max length, zero, negative)
- Concurrent operations
- Missing optional data

### Error Cases (required)
- Invalid input
- Unauthorized access
- Service unavailable / timeout
- Data not found

### Scenario Patterns

**State transition**: Given state A, When event, Then state B
**Data validation**: Given invalid data, When submitted, Then rejection with reason
**Authorization**: Given user role X, When accessing resource, Then allow/deny
**Integration**: Given external service state, When called, Then handle response

## Story Sizing Rules

A story is correctly sized when:
- It can be implemented in a **single agent session** (~1 focused feature)
- It has **2-6 acceptance criteria** (fewer = too vague, more = too large)
- It has **3-8 BDD scenarios** (fewer = untestable, more = split the story)
- It changes **1-5 source files** (more = likely too large)

### Size Estimates

The product-owner assigns a `size` during planning to set expectations:

| Size | ACs | Scenarios | Files Changed | Typical Scope |
|:---|:---|:---|:---|:---|
| `S` | 1-2 | 1-3 | 1-2 | Config, small fixes, doc updates |
| `M` | 3-4 | 3-5 | 2-4 | Standard feature, moderate logic |
| `L` | 5-6 | 5-8 | 3-5 | Complex feature, many edge cases |

## Story Types

| Type | Purpose | Output | BDD Scenarios |
|:---|:---|:---|:---|
| `standard` | Normal feature work | Production code + tests | Required (3-8) |
| `trivial` | Config, docs, small fixes | Minimal code changes | Optional (1-3 if any) |
| `spike` | Technical investigation | ADR updates + feasibility report | Optional |

### Spike Stories

Spikes investigate risky technical assumptions before committing the full backlog.

- **Output**: ADR updates in `docs/ADRs/` and/or a feasibility report — **not production code**
- **Acceptance criteria**: Required (what question does the spike answer?)
- **BDD scenarios**: Optional — spikes validate feasibility, not behavior
- **Creators**: Product-owner (for business-driven investigations) or architect (for technical investigations)
- **Sizing**: Same session limit as standard stories — if investigation is too broad, split into multiple spikes

## Status State Machine

```
todo → in-progress → done
                   → failed → in-progress (troubleshooter fixes) → done
```
