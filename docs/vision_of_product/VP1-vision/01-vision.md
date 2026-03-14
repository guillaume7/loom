# Loom — Product Vision

> *A loom weaves individual threads into fabric. Loom weaves agents, skills,
> and GitHub workflows into working software — following a pattern, with no
> human in the loop.*

---

## Why Loom Exists

Modern AI coding assistants (GitHub Copilot, Claude, etc.) are powerful within
a single turn. But development is multi-turn, multi-agent, and asynchronous:
an issue must be created, a coding agent must implement it, CI must validate it,
a reviewer must approve it, and the merge must happen — in sequence, across
multiple GitHub actors and time boundaries.

A human today acts as the glue between these steps: creating issues, checking
CI, requesting reviews, posting debug comments, merging PRs. This creates a
bottleneck limited by human availability and attention span.

**Loom exists to remove that bottleneck.**

---

## The Product

Loom is a Go CLI tool + MCP server that:

1. **Drives the GitHub development loop** end-to-end: create issue → assign
   @copilot → wait for PR → wait for CI → request review → handle feedback →
   merge → refactor → next phase.

2. **Runs locally** — as a single binary, registered as an MCP server in
   VS Code, callable from a Copilot Agent session.

3. **Persists state** in a local SQLite database — survives crashes, reboots,
   and session restarts.

4. **Exposes five MCP tools** that a Copilot Agent session calls in a loop:
   `loom_next_step`, `loom_checkpoint`, `loom_heartbeat`, `loom_get_state`,
   `loom_abort`.

5. **Has no opinion about what gets built** — it automates the workflow
   mechanics, not the implementation decisions. Those belong to the agents and
   skills in `.github/`.

---

## Design Values

### 1. Plumbing vs. Intelligence

Loom's Go binary is pure plumbing: deterministic, testable, observable. It knows
nothing about software design, code quality, or game rules. The Copilot session
is the intelligence. Neither role bleeds into the other.

### 2. Locally Testable

The FSM is a pure Go package. You can run `go test ./internal/fsm/...` with no
GitHub token, no VS Code, no internet connection. If it can't be tested on a
laptop, it shouldn't be in the state machine.

### 3. Stable APIs Only

Two previous automation attempts (Autopoietic Loop Driver v1 and v2) failed
because they depended on undocumented Copilot APIs that silently do nothing.
Loom uses only `mcp_io_github_git_*` tools (official MCP) and GitHub's public
REST API (`google/go-github`).

### 4. Observable by Default

Every state transition, every API call, every retry is written to a structured
JSON log. `loom status` and `loom log` give the human operator full visibility
without requiring them to be present.

### 5. Graceful Degradation

Every async gate has a retry budget. When a budget is exhausted, Loom
transitions to `PAUSED` and waits for human intervention (`loom resume`) rather
than retrying forever or silently failing.

---

## The Name

A mechanical loom takes individual threads (warp and weft) and weaves them
into fabric according to a pattern. Loom the tool:

- **Threads** = agents, skills, GitHub events
- **Pattern** = WORKFLOW_GITHUB.md playbook + FSM
- **Fabric** = shipping software

The weaving is autonomous. The pattern is explicit. The output is observable.

---

## What Loom Is Not

- **Not a CI/CD system** — that's GitHub Actions. Loom drives *when* to create
  issues and *when* to merge; it doesn't run builds.
- **Not an AI agent** — it has no LLM inside. The intelligence lives in the
  Copilot session that calls Loom's tools.
- **Not a general workflow engine** — it is specifically designed for the
  GitHub-native, Copilot-assisted development loop described in
  WORKFLOW_GITHUB.md.
- **Not a replacement for the squad** — agents and skills in `.github/agents/`
  and `.github/skills/` still define what gets built. Loom automates the human
  orchestrator role only.

---

## v1.0 Scope

Loom v1.0 can:

- Drive a single GitHub repository through a defined sequence of phases
- Manage the full 13-state FSM with retry budgets on every async gate
- Persist and resume from any checkpoint
- Expose 5 MCP tools callable from a VS Code Copilot session
- Provide a 6-command CLI: `start`, `status`, `pause`, `resume`, `reset`, `log`
- Compile to a single binary (Linux, macOS, Windows; amd64 + arm64)

---

## Beyond v1.0 (Out of Scope)

- Multi-repository support
- Parallel phase execution
- Web dashboard / real-time UI
- Support for non-GitHub forges (GitLab, Bitbucket)
- Custom phase sequence (v1.0 always follows `docs/epics/README.md` order)
