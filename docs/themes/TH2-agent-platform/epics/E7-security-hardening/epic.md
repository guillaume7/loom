# E7 — Security Hardening

> Theme: [TH2 — Native Agent Platform](../../README.md)
> ADR: [ADR-006](../../../../ADRs/ADR-006-security-model.md)
> Priority: P2

## Goal

Harden Loom's security posture: validate GitHub token scopes on startup,
ensure correct `readOnlyHint` annotations on MCP tools, and warn about
insecure config file permissions.

## Stories

| Story | Title | Size | Depends On |
|-------|-------|------|------------|
| US1 | GitHub token scope validation | M | — |
| US2 | MCP tool readOnlyHint annotations | S | — |
| US3 | Config file permission warning | S | — |

## Acceptance

Epic is done when:
- `loom start` validates `repo` scope on GitHub token; fails with clear error if missing
- All 5 MCP tools carry correct `readOnlyHint` values per ADR-006
- `loom start` warns if `~/.loom/config.toml` has permissions more open than 0600
