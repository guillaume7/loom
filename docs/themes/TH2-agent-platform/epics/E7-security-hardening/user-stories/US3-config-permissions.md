---
id: TH2.E7.US3
title: "Config file permission warning"
type: standard
priority: medium
size: S
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "loom start checks file permissions of ~/.loom/config.toml"
  - AC2: "If permissions are more open than 0600, a warning is logged"
  - AC3: "Warning does not block startup (informational only)"
  - AC4: "On non-Unix platforms, the check is skipped gracefully"
depends-on: []
---

# TH2.E7.US3 — Config File Permission Warning

**As a** Loom operator, **I want** `loom start` to warn if my config file has insecure permissions, **so that** I'm aware my token may be readable by other users.

## Acceptance Criteria

- [ ] AC1: `loom start` checks file permissions of `~/.loom/config.toml`
- [ ] AC2: If permissions are more open than `0600`, a warning is logged
- [ ] AC3: Warning does not block startup (informational only)
- [ ] AC4: On non-Unix platforms, the check is skipped gracefully

## BDD Scenarios

### Scenario: Secure permissions — no warning
- **Given** `~/.loom/config.toml` has permissions `0600`
- **When** `loom start` checks permissions
- **Then** no warning is logged

### Scenario: Insecure permissions — warning
- **Given** `~/.loom/config.toml` has permissions `0644`
- **When** `loom start` checks permissions
- **Then** a warning is logged: "config.toml has permissions 0644, recommended 0600"

### Scenario: Config file does not exist
- **Given** no `~/.loom/config.toml` file exists
- **When** `loom start` checks permissions
- **Then** no warning or error is produced (config from env vars)

### Scenario: Non-Unix platform
- **Given** the operating system does not support Unix file permissions
- **When** `loom start` attempts to check permissions
- **Then** the check is skipped without error
