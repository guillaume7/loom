---
id: TH2.E7.US1
title: "GitHub token scope validation on startup"
type: standard
priority: medium
size: M
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "loom start calls GitHub /user endpoint and inspects X-OAuth-Scopes header"
  - AC2: "If 'repo' scope is missing, loom start fails with a clear error listing required scopes"
  - AC3: "If 'read:org' scope is missing, a warning is logged (not a fatal error)"
  - AC4: "Token validation is skipped if --skip-auth flag is provided (for testing)"
depends-on: []
---

# TH2.E7.US1 — GitHub Token Scope Validation on Startup

**As a** Loom operator, **I want** `loom start` to validate my GitHub token has required scopes, **so that** I get a clear error upfront instead of cryptic API failures later.

## Acceptance Criteria

- [ ] AC1: `loom start` calls GitHub `/user` endpoint and inspects `X-OAuth-Scopes` header
- [ ] AC2: If `repo` scope is missing, `loom start` fails with a clear error listing required scopes
- [ ] AC3: If `read:org` scope is missing, a warning is logged (not a fatal error)
- [ ] AC4: Token validation is skipped if `--skip-auth` flag is provided (for testing)

## BDD Scenarios

### Scenario: Token with all required scopes
- **Given** a GitHub token with `repo` and `read:org` scopes
- **When** `loom start` validates the token
- **Then** startup proceeds normally

### Scenario: Token missing repo scope
- **Given** a GitHub token without `repo` scope
- **When** `loom start` validates the token
- **Then** startup fails with error "Token missing required scope: repo"

### Scenario: Token missing optional scope
- **Given** a GitHub token with `repo` but without `read:org`
- **When** `loom start` validates the token
- **Then** startup proceeds with a warning: "Optional scope 'read:org' not present"

### Scenario: Skip validation for testing
- **Given** the `--skip-auth` flag is provided
- **When** `loom start` is called
- **Then** token scope validation is skipped entirely

### Scenario: Invalid token
- **Given** an invalid or expired GitHub token
- **When** `loom start` calls the `/user` endpoint
- **Then** startup fails with error "GitHub token is invalid or expired"
