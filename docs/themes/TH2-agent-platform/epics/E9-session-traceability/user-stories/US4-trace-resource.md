---
id: TH2.E9.US4
title: "Human-readable session trace resource or tab"
status: done
epic: E9
depends-on: [TH2.E9.US3]
---

# TH2.E9.US4 — Human-readable session trace resource or tab

## As a
Loom operator

## I want
to open a single MCP resource (`loom://trace`) that renders the current
session's complete trace as human-readable Markdown

## So that
I can review the session header, FSM transitions, and GitHub ledger without
querying the database directly.

## Acceptance Criteria

- [ ] `loom://trace` is registered as an MCP resource with
      `MIMEType: "text/markdown"`.
- [ ] The resource content includes: session ID, Loom version, repository,
      start time, end time (or "in progress"), outcome, an event ledger table,
      and a GitHub ledger section when at least one PR or issue is referenced.
- [ ] When no trace session ID is configured (e.g. in tests without
      `WithTraceSessionID`), the resource returns a human-readable fallback
      message instead of an error.
- [ ] The resource reflects the live state of the trace and can be read while
      the session is still in progress.

## BDD Scenarios

```gherkin
Given a running session with trace events
When the operator reads loom://trace
Then the response body contains "# Session Trace" and has MIME type "text/markdown"

Given a server started without WithTraceSessionID
When the operator reads loom://trace
Then the response contains "No active session trace" and does not error
```
