# Loom

> *A loom weaves threads into fabric. Loom weaves agents, skills, and GitHub
> events into working software — following a pattern, with no human in the loop.*

Loom is a Go CLI tool + MCP server that drives the
[WORKFLOW_GITHUB.md](.github/squad_prompts/WORKFLOW_GITHUB.md) development
playbook end-to-end, autonomously.

---

## What It Does

```
$ loom start

[loom] state=IDLE → reading checkpoint
[loom] state=SCANNING → inspecting epics...
[loom] phase=1 (E2: State Machine) identified
[loom] state=ISSUE_CREATED → created issue #23, assigned @copilot
[loom] state=AWAITING_PR → polling for PR on phase/1-fsm...
[loom] state=AWAITING_READY → PR #24 opened
[loom] state=AWAITING_CI → waiting for CI...
[loom] state=REVIEWING → CI green; requesting Copilot review
[loom] state=MERGING → review approved
[loom] merged PR #24 → state=SCANNING
```

Loom:
- Creates GitHub issues from epic templates
- Assigns `@copilot` to implement each phase
- Polls for PRs, CI status, and review results
- Posts debug comments when CI fails
- Merges approved PRs
- Tags releases when all phases are complete
- Resumes from any checkpoint after a crash or restart

---

## How It Works

Three layers:

| Layer | What | Responsibility |
|---|---|---|
| **Copilot Master Session** | VS Code Agent chat | Intelligence: reads repo context, composes issue bodies, analyses review feedback |
| **Loom Go Binary** | MCP server (stdio) | Plumbing: FSM, GitHub polling, keepalive, checkpointing |
| **GitHub.com** | Cloud | Execution: `@copilot` coding agent, CI, Copilot reviewer |

See [docs/loom/analysis.md](docs/loom/analysis.md) for the full architecture.

---

## Quick Start

### Prerequisites

- Go 1.22+
- VS Code with GitHub Copilot extension
- GitHub personal access token with `repo` scope

### Install

```bash
git clone https://github.com/guillaume7/loom
cd loom
go build -o loom ./cmd/loom
```

### Configure

```bash
export LOOM_OWNER=your-github-org
export LOOM_REPO=your-target-repo
export LOOM_TOKEN=ghp_xxxxxxxxxxxx
```

### Run

```bash
./loom start
```

Loom registers itself as an MCP server and opens a Copilot Agent session. The
Session calls `loom_next_step` → executes the task → calls `loom_checkpoint` →
advances the state machine → loops until complete.

---

## CLI Commands

| Command | Description |
|---|---|
| `loom start` | Begin from IDLE or resume from last checkpoint |
| `loom status` | Print current state, phase, PR, and recent log |
| `loom pause` | Gracefully pause at the next safe checkpoint |
| `loom resume` | Continue from PAUSED state |
| `loom reset` | Clear all state (with confirmation) |
| `loom log` | Stream structured JSON log output |
| `loom mcp` | Start MCP stdio server (called by VS Code) |

---

## Project Structure

```
cmd/loom/        ← CLI entry point
internal/
  fsm/           ← Pure state machine (zero external deps)
  github/        ← GitHub REST API wrapper
  mcp/           ← MCP stdio server (5 tools)
  store/         ← SQLite checkpoint store
  config/        ← Config loading
docs/
  loom/          ← Architecture analysis
  adr/           ← Architecture Decision Records
  epics/         ← E1–E8 epic definitions
  vision_of_product/ ← Product vision
.github/
  agents/        ← 9-agent squad definitions
  skills/        ← Reusable skill files
  workflows/     ← CI (go vet, golangci-lint, go test -race, cross-compile)
```

---

## Documentation

- [Architecture Analysis](docs/loom/analysis.md)
- [ADR-001: Adopt Loom](docs/adr/ADR-001-loom-local-orchestrator.md)
- [Product Vision](docs/vision_of_product/01-vision.md)
- [Epics Overview](docs/epics/README.md)
- [Workflow Playbook](.github/squad_prompts/WORKFLOW_GITHUB.md)

---

## Development

```bash
# Run all tests
go test ./... -race

# Lint
golangci-lint run ./...

# Build
go build ./cmd/loom
```

## License

MIT
