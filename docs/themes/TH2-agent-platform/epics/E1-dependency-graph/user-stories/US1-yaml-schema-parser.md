---
id: TH2.E1.US1
title: "YAML schema definition and parser"
type: standard
priority: high
size: M
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "Load() parses a valid .loom/dependencies.yaml into typed Go structs"
  - AC2: "Schema version field is validated (version: 1 accepted, unknown versions rejected)"
  - AC3: "Parse errors return descriptive error messages with line numbers"
  - AC4: "Empty or missing file returns a clear error, not a panic"
depends-on: []
---

# TH2.E1.US1 — YAML Schema Definition and Parser

**As a** Loom developer, **I want** a Go package that parses `.loom/dependencies.yaml` into typed structs, **so that** the dependency graph can be evaluated programmatically.

## Acceptance Criteria

- [ ] AC1: `Load(path)` parses a valid `.loom/dependencies.yaml` into typed Go structs
- [ ] AC2: Schema version field is validated (`version: 1` accepted, unknown versions rejected)
- [ ] AC3: Parse errors return descriptive error messages with line numbers
- [ ] AC4: Empty or missing file returns a clear error, not a panic

## BDD Scenarios

### Scenario: Parse valid dependencies file
- **Given** a `.loom/dependencies.yaml` with `version: 1` and two epics with stories
- **When** `Load(path)` is called
- **Then** a `Graph` is returned with the correct number of epics and stories
- **And** each epic and story has its `ID` and `DependsOn` fields populated

### Scenario: Reject unknown schema version
- **Given** a `.loom/dependencies.yaml` with `version: 99`
- **When** `Load(path)` is called
- **Then** an error is returned containing "unsupported version"

### Scenario: Handle missing file
- **Given** no `.loom/dependencies.yaml` exists at the given path
- **When** `Load(path)` is called
- **Then** an error is returned wrapping `os.ErrNotExist`

### Scenario: Handle malformed YAML
- **Given** a `.loom/dependencies.yaml` with invalid YAML syntax
- **When** `Load(path)` is called
- **Then** an error is returned containing the YAML parse error details

### Scenario: Handle empty file
- **Given** an empty `.loom/dependencies.yaml`
- **When** `Load(path)` is called
- **Then** an error is returned indicating the file is empty or has no content
