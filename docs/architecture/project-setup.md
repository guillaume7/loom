# Loom v2 — Project Setup

> Traces to: existing codebase structure and [VP2 §11](../vision_of_product/VP2-agent-platform/02-vision-agent-platform.md) (what has not changed)

## 1. Repository Structure

```
loom/
├── .github/
│   ├── agents/                    # v2: Custom agent definitions
│   │   ├── loom-orchestrator.agent.md
│   │   ├── loom-gate.agent.md
│   │   ├── loom-debug.agent.md
│   │   └── loom-merge.agent.md
│   ├── copilot/
│   │   └── mcp.json               # MCP server registration
│   ├── copilot-instructions.md     # Workspace instructions
│   └── skills/                     # Copilot skill definitions
├── .loom/                          # Runtime state (gitignored)
│   ├── state.db                    # SQLite checkpoint, action log, and session trace tables
│   └── dependencies.yaml           # v2: Dependency DAG
├── cmd/
│   └── loom/                       # CLI entry point (cobra commands)
│       ├── main.go
│       ├── cmd_start.go
│       ├── cmd_status.go
│       ├── cmd_pause.go
│       ├── cmd_resume.go
│       ├── cmd_log.go
│       ├── cmd_mcp.go
│       ├── cmd_reset.go
│       └── cmd_version.go
├── internal/
│   ├── config/                     # TOML config + env var loading
│   ├── depgraph/                   # v2: Dependency DAG parser + evaluator
│   ├── fsm/                        # Finite state machine (zero deps)
│   ├── github/                     # GitHub REST API client
│   ├── mcp/                        # MCP server (tools, resources, monitor)
│   ├── store/                      # SQLite persistence layer
│   └── tools/                      # Compile-time dependency checks
├── integration/                    # Integration tests
├── docs/
│   ├── architecture/               # This directory
│   ├── ADRs/                       # Architecture Decision Records
│   ├── themes/                     # Planning: themes/epics/stories
│   ├── plan/                       # Backlog and session log
│   └── vision_of_product/          # Vision documents
├── go.mod
├── go.sum
└── README.md
```

## 2. Build & Test

### Prerequisites

- Go 1.23+
- No CGo (pure Go SQLite driver)
- No external services required for unit tests

### Commands

```bash
# Build
go build -o loom ./cmd/loom

# Unit tests
go test ./...

# Integration tests (require GitHub token)
LOOM_TOKEN=... go test ./integration/ -v

# Lint (if golangci-lint is available)
golangci-lint run ./...

# Verify dependencies
go mod verify
```

### Single Binary

The build produces a single `loom` binary with no runtime dependencies. This is a core design value (ADR-001).

## 3. Development Environment

### Dev Container

The workspace includes a dev container configuration for consistent development. The container runs Debian 12 (bookworm) with Go pre-installed.

### Configuration

Developers create `~/.loom/config.toml`:

```toml
owner = "your-org"
repo  = "your-repo"
token = "ghp_..."
```

Or use environment variables:

```bash
export LOOM_OWNER=your-org
export LOOM_REPO=your-repo
export LOOM_TOKEN=ghp_...
```

### MCP Server Registration

Register Loom as an MCP server in `.github/copilot/mcp.json` (or `.vscode/mcp.json`):

```json
{
  "servers": {
    "loom": {
      "command": "./loom",
      "args": ["mcp"],
      "transport": "stdio"
    }
  }
}
```

## 4. Branching & Commit Convention

- **Main-only** branching (current practice for solo/small team).
- Conventional commits with qualified story ID:

```
feat(TH2.E3.US3): add loom://state MCP resource
feat(TH2.E9.US4): expose session trace resource surface
fix(TH2.E5.US3): correct task capability fallback
```

## 5. v2-Specific Setup

### Agent Definitions

Custom agent files go in `.github/agents/`. They require VS Code v1.106+ with Copilot enabled.

### Dependencies File

`.loom/dependencies.yaml` is created and maintained by the Loom CLI or manually. It is **not** committed to version control (runtime state); the schema is validated by `internal/depgraph`.

### `.gitignore` Additions for v2

```
# Loom runtime state
.loom/state.db
.loom/state.db-journal
.loom/state.db-wal
```

Note: `.loom/dependencies.yaml` may be committed if the project wants the dependency graph version-controlled. This is a project-level decision.
