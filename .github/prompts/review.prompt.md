---
description: "Run an ad-hoc code review on recent changes. Use when: reviewing implementation quality, security audit, checking conventions outside the autopilot loop."
agent: "reviewer"
tools: [read, search]
---

## Agents & Skills

| Agent | Skills |
|-------|--------|
| @reviewer | `the-copilot-build-method`, `code-quality` |

Perform a thorough code review on recent changes.

## Context Gathering

1. Run `git diff --name-only HEAD~1` (or `git diff --name-only --staged` if there are staged changes) to identify changed files
2. If a story path is provided as argument, read the story file for acceptance criteria and BDD scenarios
3. Read `docs/architecture/` for tech stack and conventions
4. If no story context is given, review the changes as a standalone quality/security audit

## Review Scope

Apply the full checklist from skill: `code-quality`:
- **Correctness** — logic errors, edge cases, race conditions
- **Security** — OWASP Top 10 audit
- **Architecture** — component boundaries, ADR compliance
- **Code quality** — conventions, DRY, complexity
- **Tests** — coverage, determinism, meaningful assertions

## Output

Return a structured review report with verdict (APPROVE / REQUEST_CHANGES), issues by severity (critical, suggestion, nit), and security assessment.
