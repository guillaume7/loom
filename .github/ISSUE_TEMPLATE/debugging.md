---
name: "Debugging"
about: Report a bug or failing test that requires root-cause analysis and a fix with a regression test.
title: "debug: "
labels: ["bug", "debugging"]
---

## Assigned Agent

**[Debugger](../agents/debugger.md)** — apply [`loom-architecture`](../skills/loom-architecture.md) · [`tdd-workflow`](../skills/tdd-workflow.md).

## Problem Description

<!-- Describe the observed behaviour and what you expected to happen. -->

**Observed:** 

**Expected:** 

## Reproduction Steps

<!-- Minimal steps to reproduce the issue. Include command output or test names where applicable. -->

1. 
2. 
3. 

## Failing Tests / Logs

<!-- Paste relevant test output, stack traces, or structured log lines. -->

```
```

## Affected Component

<!-- Check all that apply. -->

- [ ] `internal/fsm` — State machine
- [ ] `internal/github` — GitHub client
- [ ] `internal/mcp` — MCP server / tool handlers
- [ ] `internal/store` — SQLite checkpointing
- [ ] `internal/config` — Configuration
- [ ] `cmd/loom` — CLI
- [ ] CI / build pipeline

## Root Cause Hypothesis

<!-- Your initial hypothesis. Leave blank if unknown. -->

## Proposed Fix

<!-- Outline the fix approach. Leave blank if unknown. -->

## Debugging Checklist

- [ ] Minimal reproduction case identified
- [ ] Root cause confirmed (not just symptoms)
- [ ] Regression test written **before** applying the fix
- [ ] Fix applied and regression test passes
- [ ] No unrelated behaviour changed
- [ ] `go test ./... -race` passes after fix
