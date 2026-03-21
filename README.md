# Loom

Loom drives software delivery in GitHub repositories with as little human involvement as possible.

Its promise is simple: give Loom a repository and a workflow, and it will keep the delivery loop moving through GitHub for you.

That means handling work such as:

- picking the next item to work on
- creating or advancing the issue for that work
- driving implementation through agents
- waiting for pull requests and CI
- requesting and processing reviews
- looping on review fixes when needed
- merging when safe
- moving on to the next item

If something needs a real decision or a manual intervention, Loom pauses and tells you. Otherwise, it keeps going.

## What Loom Is For

Loom is for teams or solo builders who want GitHub delivery to run as an operating loop instead of a manually supervised sequence.

Use it when you want to reduce the time spent doing repetitive coordination work such as:

- checking whether the PR is ready yet
- waiting on CI and coming back later
- asking for the next fix after review comments
- deciding when it is safe to merge
- remembering what should happen after the merge

Loom is the operator for that loop.

## What You Need To Use Loom

To operate Loom, you need:

- a GitHub repository
- VS Code with GitHub Copilot
- a GitHub token with `repo` scope
- the `loom` binary available on your machine

Optional but useful:

- `read:org` scope for organization-aware features

## Quick Start

### 1. Install Loom

Build from source:

```bash
git clone https://github.com/guillaume7/loom
cd loom
go build -o loom ./cmd/loom
```

Or install with Go:

```bash
go install github.com/guillaume7/loom/cmd/loom@latest
```

Then verify:

```bash
loom --version
```

### 2. Configure GitHub Access

The simplest setup is environment variables:

```bash
export LOOM_OWNER=your-org-or-user
export LOOM_REPO=your-repo
export LOOM_TOKEN=ghp_xxxxxxxxxxxx
```

You can also use `~/.loom/config.toml`:

```toml
owner = "your-org-or-user"
repo = "your-repo"
token = "ghp_xxxxxxxxxxxx"
db_path = ".loom/state.db"
log_path = "/home/you/.loom/loom.log"
max_parallel = 3
```

Recommended permissions:

```bash
mkdir -p ~/.loom
chmod 700 ~/.loom
chmod 600 ~/.loom/config.toml
```

### 3. Register Loom As An MCP Server

Add Loom to your MCP configuration so Copilot can use it:

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

If you are running from source and not from your `PATH`, use `./loom` or `go run ./cmd/loom` instead of `loom`.

### 4. Start Loom

Run the runtime:

```bash
loom start
```

This starts or resumes Loom's control loop and keeps it running in the foreground.

### 5. Run Loom From Copilot

Open the target repository in VS Code and run:

```text
/run-loom
```

That is the operator entry point for the GitHub delivery loop.

## What Happens During A Run

At a high level, Loom works like this:

1. It reads the current state of the work.
2. It decides what the next delivery action is.
3. It drives that action through GitHub and agents.
4. It waits through PR, CI, and review states.
5. It records progress and keeps moving.
6. It pauses only when a real escalation or manual choice is required.

From a user point of view, the outcome is what matters:

- work keeps progressing in GitHub
- the loop survives waits and interruptions
- you can inspect status at any time
- you can pause, resume, or reset when needed

## Day-To-Day Commands

Start or resume Loom:

```bash
loom start
```

See where it is:

```bash
loom status
```

Pause the current run:

```bash
loom pause
```

Resume after a pause:

```bash
loom resume
```

See recent action history:

```bash
loom log -n 20
```

Follow the legacy log file:

```bash
loom log --follow
```

Clear Loom state if you need a fresh start:

```bash
loom reset
```

## The Main Operating Modes

Loom supports two user-facing modes:

### Loom Weaving

This is the GitHub delivery mode.

Use `/run-loom` when you want Loom to drive the repository through the GitHub execution loop: implementation, PR progression, review handling, merge, and next item selection.

### Local Autopilot

This is the local repository execution mode.

Use `/run-autopilot` when you want the agent squad to work story by story inside the repository using the planning backlog.

If your goal is to drive software delivery in GitHub, `/run-loom` is the mode you care about.

## Troubleshooting

If Loom is not doing what you expect, start here:

1. Check the current state with `loom status`.
2. Check recent actions with `loom log -n 20`.
3. Confirm your `LOOM_OWNER`, `LOOM_REPO`, and `LOOM_TOKEN` values are correct.
4. Confirm the MCP server is registered correctly.
5. If needed, pause with `loom pause`, then resume with `loom resume`.
6. If the run state is no longer useful, reset with `loom reset` and start again.

## CLI Reference

| Command | Purpose |
| --- | --- |
| `loom start` | Start or resume Loom |
| `loom status` | Show current workflow and controller state |
| `loom pause` | Pause the current run |
| `loom resume` | Resume a paused run |
| `loom reset` | Clear stored state after confirmation |
| `loom log` | Show recent action history |
| `loom version` | Show version information |
| `loom mcp` | Start the MCP server |

You can also print version metadata with:

```bash
loom --version
```

## Where Loom Stores State

By default, Loom uses:

| Path | Purpose |
| --- | --- |
| `.loom/state.db` | Local workflow state |
| `~/.loom/config.toml` | User configuration |
| `~/.loom/loom.log` | File log |

## Learn More

If you want the deeper architecture and planning documents, start with:

- `docs/architecture/README.md`
- `docs/architecture/deployment.md`
- `docs/plan/backlog.yaml`

## License

MIT
