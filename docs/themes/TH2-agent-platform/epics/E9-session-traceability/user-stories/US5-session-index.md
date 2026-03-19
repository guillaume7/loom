---
id: TH2.E9.US5
title: "Session index, discoverability, and retention"
status: done
epic: E9
depends-on: [TH2.E9.US1]
---

# TH2.E9.US5 — Session index, discoverability, and retention

## As a
Loom operator

## I want
a session index resource (`loom://trace/index`) that lists the most recent
run-loom sessions ordered newest-first

## So that
I can quickly discover historical sessions, compare outcomes across runs, and
verify that the trace is append-only and non-authoritative.

## Acceptance Criteria

- [ ] `loom://trace/index` is registered as an MCP resource with
      `MIMEType: "application/json"`.
- [ ] The resource returns a JSON array of up to 50 session headers, ordered
      newest-first.
- [ ] Each entry includes: `session_id`, `loom_ver`, `repository`,
      `started_at`, `ended_at` (omitted when still running), `outcome`.
- [ ] Session traces are never used as workflow orchestration state; the FSM
      checkpoint remains the sole source of truth.
- [ ] `DeleteAll` removes all session traces and events (consistent with the
      existing reset contract).

## BDD Scenarios

```gherkin
Given three sessions have been recorded
When the operator reads loom://trace/index
Then the JSON array contains three entries ordered newest-first

Given an empty store
When the operator reads loom://trace/index
Then the response is an empty JSON array "[]"

Given a completed session
When the operator reads loom://trace/index
Then the entry for that session has a non-empty ended_at field
```
