---
id: TH2.E3.US1
title: "MCP resource registration framework"
type: standard
priority: medium
size: S
agents: [developer]
skills: [bdd-stories]
acceptance-criteria:
  - AC1: "MCP server supports resource registration via mcp-go library"
  - AC2: "Server responds to resources/list with registered resource URIs"
  - AC3: "Server responds to resources/read with resource content for valid URIs"
  - AC4: "Unknown resource URI returns a structured error"
depends-on: []
---

# TH2.E3.US1 — MCP Resource Registration Framework

**As a** Loom MCP server developer, **I want** a framework for registering MCP resources, **so that** each resource (dependencies, state, log) can be added incrementally.

## Acceptance Criteria

- [ ] AC1: MCP server supports resource registration via `mcp-go` library
- [ ] AC2: Server responds to `resources/list` with registered resource URIs
- [ ] AC3: Server responds to `resources/read` with resource content for valid URIs
- [ ] AC4: Unknown resource URI returns a structured error

## BDD Scenarios

### Scenario: List registered resources
- **Given** an MCP server with one resource registered at URI "loom://test"
- **When** a `resources/list` request is received
- **Then** the response contains an entry with URI "loom://test"
- **And** the entry includes a name and description

### Scenario: Read registered resource
- **Given** an MCP server with a resource registered at URI "loom://test" returning "hello"
- **When** a `resources/read` request for "loom://test" is received
- **Then** the response contains "hello"

### Scenario: Read unknown resource
- **Given** an MCP server with no resource at URI "loom://unknown"
- **When** a `resources/read` request for "loom://unknown" is received
- **Then** a structured error is returned indicating the resource was not found
