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
`.github/agents/loom-mcp-operator.md` together with the skill
`.github/skills/loom-mcp-loop.md` so the session follows the exact Loom MCP
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
