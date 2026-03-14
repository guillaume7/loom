---
description: "Diagnose and fix a failing build, test, or story. Use when: tests are broken, build fails, a story is stuck, or you need root-cause analysis outside the autopilot loop."
agent: "troubleshooter"
tools: [read, edit, search, execute]
---

## Agents & Skills

| Agent | Skills |
|-------|--------|
| @troubleshooter | `the-copilot-build-method`, `bdd-stories`, `code-quality` |

Diagnose and fix the current failure.

## Context Gathering

1. If a story path is provided as argument, read the story file for acceptance criteria and expected behavior
2. Check `docs/plan/backlog.yaml` for any stories with `status: failed` — start with those
3. Run the project's test suite to reproduce the failure
4. Read error output carefully before proposing any fix

## Diagnosis Process

1. **Reproduce** — run the failing test or build command
2. **Categorize** — logic error, test bug, build/dependency issue, integration error, or requirement ambiguity
3. **Root cause** — trace the failure to its source (don't fix symptoms)
4. **Fix** — apply the minimal change needed
5. **Verify** — re-run tests to confirm the fix works and doesn't break other tests

## Output

Return a structured troubleshooting report with root cause, category, fix applied, and verification results.
