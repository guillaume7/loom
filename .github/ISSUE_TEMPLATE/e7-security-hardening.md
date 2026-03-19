---
name: "E7 — Security Hardening"
about: Harden Loom startup and MCP surfaces with token validation, tool annotations, and config permission checks.
title: "E7: Security Hardening"
labels: ["epic", "E7", "TH2"]
---

## Goal

Harden Loom's security posture: validate GitHub token scopes on startup, ensure correct `readOnlyHint` annotations on MCP tools, and warn about insecure config file permissions.

## User Stories

- [ ] TH2.E7.US1 — GitHub token scope validation on startup
- [ ] TH2.E7.US2 — MCP tool readOnlyHint annotations
- [ ] TH2.E7.US3 — Config file permission warning

## Acceptance Criteria

- [ ] `loom start` validates `repo` scope on GitHub token and fails with a clear error if missing
- [ ] All 5 MCP tools carry correct `readOnlyHint` values per ADR-006
- [ ] `loom start` warns if `~/.loom/config.toml` has permissions more open than 0600