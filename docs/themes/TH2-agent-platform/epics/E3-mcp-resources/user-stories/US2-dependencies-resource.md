---
id: TH2.E3.US2
title: "loom://dependencies MCP resource"
type: standard
priority: medium
size: M
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "Resource is registered at URI loom://dependencies"
  - AC2: "Reading the resource returns the raw YAML content of .loom/dependencies.yaml"
  - AC3: "If .loom/dependencies.yaml does not exist, an error message is returned"
  - AC4: "Content type is text/yaml"
depends-on: [TH2.E3.US1]
---

# TH2.E3.US2 — loom://dependencies MCP Resource

**As a** Loom agent, **I want** to read the dependency graph via `loom://dependencies`, **so that** I can understand story dependencies without a tool call.

## Acceptance Criteria

- [ ] AC1: Resource is registered at URI `loom://dependencies`
- [ ] AC2: Reading the resource returns the raw YAML content of `.loom/dependencies.yaml`
- [ ] AC3: If `.loom/dependencies.yaml` does not exist, an error message is returned
- [ ] AC4: Content type is `text/yaml`

## BDD Scenarios

### Scenario: Read dependency graph
- **Given** a `.loom/dependencies.yaml` with two epics and four stories
- **When** `resources/read` is called for "loom://dependencies"
- **Then** the response contains the YAML content verbatim
- **And** the MIME type is "text/yaml"

### Scenario: File not found
- **Given** no `.loom/dependencies.yaml` exists
- **When** `resources/read` is called for "loom://dependencies"
- **Then** a structured error is returned indicating the file is missing

### Scenario: File updated between reads
- **Given** a `.loom/dependencies.yaml` is modified after initial read
- **When** `resources/read` is called again
- **Then** the updated content is returned (no caching)
