---
name: "Refactoring"
about: Improve code quality, reduce technical debt, or extract reusable abstractions without changing observable behaviour.
title: "refactor: "
labels: ["refactoring"]
---

## Assigned Agent

**[Refactoring Agent](../agents/refactoring-agent.md)** — apply [`go-standards`](../skills/go-standards.md) · [`review-checklist`](../skills/review-checklist.md).

## Motivation

<!-- Why does this refactoring need to happen? What problem does it solve? -->

## Scope

<!-- Which packages or files are in scope for this refactoring? -->

- [ ] `internal/fsm`
- [ ] `internal/github`
- [ ] `internal/mcp`
- [ ] `internal/store`
- [ ] `internal/config`
- [ ] `cmd/loom`
- [ ] Other: 

## Proposed Changes

<!-- Describe what will change. Be specific about interfaces, types, naming, or structural changes. -->

## Observable Behaviour

<!-- Confirm that no observable behaviour changes. List any edge cases to verify. -->

This refactoring must **not** change:

- 

## Pre-Refactoring Checklist

- [ ] All existing tests pass before starting (`go test ./... -race`)
- [ ] Coverage baseline recorded: __%
- [ ] Scope is clearly bounded — no feature work included

## Post-Refactoring Checklist

- [ ] All existing tests still pass (`go test ./... -race`)
- [ ] Coverage is equal to or higher than baseline
- [ ] No `panic` introduced in library code
- [ ] No global mutable state introduced
- [ ] Naming follows Go conventions and project style
- [ ] `golangci-lint run ./...` passes

## Notes

<!-- Any additional context, known risks, or related issues. -->
