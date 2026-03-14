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

## Agent Squad & Autopilot

Loom ships two operating modes driven by a squad of specialist AI agents:

| Mode | Prompt | Description |
|---|---|---|
| **Local Autopilot** | `/run-autopilot` | Orchestrator loops through the backlog locally: developer → reviewer → troubleshooter |
| **Loom Weaving** | `/run-loom` | `loom-mcp-operator` drives GitHub PRs server-side via `loom_next_step` / `loom_checkpoint` |

### Lifecycle Prompts

| Prompt | Phase | Purpose |
|---|---|---|
| `/kickstart-vision` | 1 | Interactively capture the product vision |
| `/plan-product` | 2–3 | Architect → product-owner: architecture + backlog |
| `/run-autopilot` | 4A | Local autonomous development loop |
| `/run-loom` | 4B | Server-side PR weaving loop |
| `/review` | 4 | Ad-hoc code review |
| `/troubleshoot` | 4 | Ad-hoc failure diagnosis and fix |

### Agent Roster

| Agent | Phase | Role |
|---|---|---|
| **orchestrator** | 4A | Squad leader; sequences stories, manages lifecycle loop |
| **product-owner** | 3 | Vision → themes, epics, BDD stories, backlog |
| **architect** | 2 | Vision → architecture, tech stack, ADRs |
| **developer** | 4A | Implements + tests exactly one user story |
| **reviewer** | 4A | Code review: correctness, security, conventions |
| **troubleshooter** | 4A | Diagnoses and fixes failed stories |
| **loom-mcp-operator** | 4B | Drives Loom MCP tools in the master session |
| **loom-orchestrator** | 4B | Orchestrates the Loom FSM end-to-end |
| **loom-gate** | 4B | Evaluates whether a PR is safe to merge |
| **loom-debug** | 4B | Diagnoses CI failures on a pull request |
| **loom-merge** | 4B | Merges a pull request by number |

See [`.github/agents/README.md`](.github/agents/README.md) for the full agent ↔ skill matrix.

### Skills

| Skill | Covers |
|---|---|
| `the-copilot-build-method` | 4-phase lifecycle, directory conventions, Definition of Done |
| `bdd-stories` | Story format, acceptance criteria, BDD scenarios |
| `backlog-management` | YAML schema, status state machine, dependency resolution |
| `code-quality` | Review checklist, OWASP Top 10 security audit |
| `architecture-decisions` | ADR format, tech stack analysis, component boundaries |
| `loom-mcp-loop` | Canonical `loom_next_step` → GitHub action → `loom_checkpoint` loop |

---

## Quick Start

### Prerequisites

- VS Code with GitHub Copilot extension
- GitHub personal access token with `repo` scope

If you want to build Loom from source instead of installing a release binary,
you also need Go 1.23+.

### Install From A Release

Each GitHub release publishes these assets:

- `loom-linux-amd64`
- `loom-linux-arm64`
- `loom-darwin-amd64`
- `loom-darwin-arm64`
- `loom-windows-amd64.exe`
- `checksums.txt`

Pick the asset that matches your OS and CPU architecture, download it from the
[GitHub Releases](https://github.com/guillaume7/loom/releases) page, verify the
checksum, and place it on your `PATH` as `loom`.

Example for Linux/macOS:

```bash
VERSION=v1.0.0
OS=linux
ARCH=amd64

curl -L -o loom "https://github.com/guillaume7/loom/releases/download/${VERSION}/loom-${OS}-${ARCH}"
curl -L -o checksums.txt "https://github.com/guillaume7/loom/releases/download/${VERSION}/checksums.txt"
grep "  loom-${OS}-${ARCH}$" checksums.txt | sha256sum -c -
install -m 0755 loom /usr/local/bin/loom
loom --version
```

Example for Windows PowerShell:

```powershell
$Version = "v1.0.0"
Invoke-WebRequest -Uri "https://github.com/guillaume7/loom/releases/download/$Version/loom-windows-amd64.exe" -OutFile "loom.exe"
Invoke-WebRequest -Uri "https://github.com/guillaume7/loom/releases/download/$Version/checksums.txt" -OutFile "checksums.txt"
Get-FileHash .\loom.exe -Algorithm SHA256
```

Compare the SHA-256 hash from `Get-FileHash` with the `loom-windows-amd64.exe`
entry in `checksums.txt`, then move `loom.exe` somewhere on your `PATH`.

### Build From Source

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

### Configure As An MCP Server

Once `loom` is on your `PATH`, register it in your VS Code or Copilot MCP
configuration:

```json
{
  "mcpServers": {
    "loom": {
      "type": "stdio",
      "command": "loom",
      "args": ["mcp"]
    }
  }
}
```

### Run

```bash
loom start
```

Loom registers itself as an MCP server and opens a Copilot Agent session. The
Session calls `loom_next_step` → executes the task → calls `loom_checkpoint` →
advances the state machine → loops until complete.

For that master session, use the workspace agent
`.github/agents/loom-mcp-operator.agent.md` together with the skill
`.github/skills/loom-mcp-loop/SKILL.md` so the session follows the exact Loom MCP
contract instead of improvising workflow steps.

If you built from source and kept the binary in the repository root rather than
installing it globally, run `./loom start` instead.

### Dev Container

If you want to keep your host Go toolchain untouched, open the repo in the dev
container instead:

```bash
code .
```

Then run `Dev Containers: Reopen in Container` in VS Code. The container uses
Go 1.23 and includes `nodejs`/`npm`, so the workspace MCP tools in
`.vscode/mcp.json` can start `go run ./cmd/loom mcp` and `npx
@upstash/context7-mcp@latest` without installing anything on the host.

---

## Release Process

To publish a new Loom release such as `v1.0.0`:

1. Ensure `main` is releasable.
2. Run `go test ./...`, `go test -race ./...`, and `go build ./cmd/loom`.
3. Confirm `loom --version` reports the expected build metadata in a tagged build.
4. Create and push an annotated tag:

```bash
git checkout main
git pull --ff-only
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0
```

The release workflow in `.github/workflows/release.yml` then runs GoReleaser,
which:

- cross-compiles the supported binaries
- injects version, commit, and build date metadata
- generates `checksums.txt`
- generates Git-based release notes
- publishes all artifacts to the GitHub Release for that tag

Users then install Loom by downloading the matching release asset from GitHub
Releases and placing it on their `PATH`.

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
| `loom version` | Print the Loom version, commit, and build date |
| `loom mcp` | Start MCP stdio server (called by VS Code) |

You can also print build metadata directly with `loom --version`.

---

## Project Structure

```
cmd/loom/        ← CLI entry point
internal/
  fsm/           ← Pure state machine (zero external deps)
  github/        ← GitHub REST API wrapper
  mcp/           ← MCP stdio server
  store/         ← SQLite checkpoint store
  config/        ← Config loading
  depgraph/      ← Dependency graph engine
  agentspawn/    ← Agent spawn utilities
  gitworktree/   ← Git worktree management
  tools/         ← Shared tool helpers
docs/
  loom/          ← Architecture analysis
  ADRs/          ← Architecture Decision Records
  architecture/  ← System architecture docs (components, data model, tech stack)
  themes/        ← TH1, TH2 … themes with epics and user stories
  vision_of_product/ ← VP1, VP2 … product vision documents
  plan/          ← backlog.yaml, session-log.md
.github/
  agents/        ← 11-agent squad + archived agents
  skills/        ← 6 reusable skills (the-copilot-build-method, bdd-stories,
                    backlog-management, code-quality, architecture-decisions, loom-mcp-loop)
  prompts/       ← Lifecycle prompts (/kickstart-vision, /plan-product,
                    /run-autopilot, /run-loom, /review, /troubleshoot)
  workflows/     ← CI (go vet, golangci-lint, go test -race, cross-compile)
```

---

## Documentation

- [Architecture Analysis](docs/loom/analysis.md)
- [System Architecture](docs/architecture/README.md)
- [ADR-001: Adopt Loom](docs/ADRs/ADR-001-loom-local-orchestrator.md)
- [Product Vision](docs/vision_of_product/VP1-vision/01-vision.md)
- [Themes & Epics Overview](docs/themes/TH1-loom-1/epics/README.md)
- [Agent Squad](.github/agents/README.md)
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
