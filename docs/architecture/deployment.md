# Loom Runtime-First Architecture — Deployment & Distribution

> Traces to: [VP3](../vision_of_product/VP3-runtime-first/03-vision-runtime-first.md), [ADR-008](../ADRs/ADR-008-runtime-first-control-plane-and-wake-model.md), [ADR-010](../ADRs/ADR-010-bounded-agent-jobs-and-run-locking.md)

## 1. Distribution Model

Loom remains a **local-first** tool. VP3 adds a stronger runtime boundary, not a
requirement for hosted infrastructure.

| Artifact | Format | Target |
|----------|--------|--------|
| `loom` binary | Single Go binary (no CGo) | macOS (arm64, amd64), Linux (amd64, arm64) |
| Agent definitions | `.github/agents/*.agent.md` | Checked into target repo |
| MCP registration | `.github/copilot/mcp.json` or `.vscode/mcp.json` | Checked into target repo |

### Binary Distribution

- **GitHub Releases**: `goreleaser` or manual cross-compilation produces binaries per platform.
- **Go install**: `go install github.com/guillaume7/loom/cmd/loom@latest`.
- No container image needed — the binary has no runtime dependencies.

## 2. Runtime Modes

Detailed baseline selection and rejected alternatives are recorded in
[TH3.E1.US1 runtime mode decision](../themes/TH3-runtime-first-reengineering/epics/E1-runtime-kernel-foundation/runtime-mode-decision.md).

### 2.1 Resumable Runner (baseline)

- `loom start` launches a foreground run.
- The runtime persists checkpoint, schedules, locks, and trace state.
- A later invocation resumes from persisted state without requiring the prior
	session to stay alive.

### 2.2 Optional Local Daemon (future)

- A daemon mode may own continuous wake-up execution for teams that want
	always-on local operation.
- The daemon remains optional and must not become a hard requirement for Loom
	development or debugging.

## 3. Operational Concerns

### 3.1 State Location

| File | Location | Lifecycle |
|------|----------|-----------|
| `state.db` | `.loom/state.db` (project root) | Created on first `loom start`; stores checkpoints, schedules, locks, policy decisions, action log, and traces |
| `dependencies.yaml` | `.loom/dependencies.yaml` | Created by orchestrator or developer |
| `config.toml` | `~/.loom/config.toml` | Per-user, machine-scoped |
| `loom.log` | `~/.loom/loom.log` | Append-only structured JSON log |

### 3.2 Health Monitoring

| Signal | How | Consumer |
|--------|-----|----------|
| FSM state | `loom status` CLI / `loom_get_state` MCP tool | Human / agent |
| Wake schedule backlog | Runtime scheduler inspection | Human / runtime |
| Lock ownership | Runtime lease records | Human / runtime |
| Policy outcomes | Structured decision records | Human / runtime |
| Action history | `loom log` CLI / `loom://log` MCP resource | Human / agent |
| Session trace | `loom://sessions` and `loom://session/<id>` MCP resources | Human / agent |
| OS notification | VS Code notification on PAUSED or operator-required choice | Human |

### 3.3 Recovery Procedures

| Failure | Recovery |
|---------|----------|
| Process crash | Restart `loom`; runtime resumes from checkpoint, schedules, and leases |
| Agent job failure | Runtime marks job failure, retries or escalates without corrupting checkpoint truth |
| Lost interactive session | Resume through runtime state; no waiting state requires the old session |
| Retry budget exhausted | Runtime policy emits explicit escalation outcome or operator choice |
| Unexpected run behavior | Open the retained session trace to replay FSM, GitHub, and operator events from one artifact |
| SQLite corruption | `loom reset` + manual re-creation (rare; WAL mode reduces risk) |

## 4. Organization-Level Deployment (Optional)

VP2 §10 identifies org-level distribution as a P3 deliverable.

### 3.1 Org MCP Registry

Loom can be published to an organization's private MCP server registry (VS Code v1.106+). All team members use the same vetted server version without manual installation.

### 3.2 Organization-Level Agents

Agent definitions in `.github/agents/` can be promoted to organization-level custom agents (VS Code v1.107+), ensuring consistent orchestration behavior across repositories.

### 3.3 Tool Eligibility Policy

Enterprise admins configure which Loom tools require human approval:

| Tool | Auto-Approve | Rationale |
|------|-------------|-----------|
| `loom_get_state` | Yes | Read-only, `readOnlyHint: true` |
| `loom_heartbeat` | Yes | Read-only, `readOnlyHint: true` |
| `loom_next_step` | No | Returns action plan; side-effect potential in subsequent steps |
| `loom_checkpoint` | No | Mutates FSM state |
| `loom_abort` | No | Transitions to PAUSED; disruptive |

Configured via `chat.tools.eligibleForAutoApproval` in VS Code settings or org policy.

## 5. Security

### 4.1 Authentication

| Layer | Mechanism |
|-------|-----------|
| GitHub API | Personal Access Token or GitHub App token in `LOOM_TOKEN` |
| MCP server | stdio transport (local process, no network auth needed) |
| Remote MCP (future) | CIMD + `WWW-Authenticate` scope escalation (MCP spec v1.106+) |

### 4.2 Token Scope Requirements

Minimum GitHub token scopes for full Loom operation:

- `repo` — issues, PRs, checks, merges
- `read:org` — org membership (for org-level agent eligibility)

### 4.3 Secrets Management

- `LOOM_TOKEN` is never logged (config field tagged; slog excluded).
- Token is read from environment variable or `~/.loom/config.toml` (file permissions 0600).
- Agents never see the token — the Go binary handles all GitHub API calls directly.

> See [ADR-006](../ADRs/ADR-006-security-model.md) for the full security model decision.
