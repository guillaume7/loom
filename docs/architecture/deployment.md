# Loom v2 — Deployment & Distribution

> Traces to: [VP2 §10](../vision_of_product/VP2-agent-platform/02-vision-agent-platform.md) (implementation roadmap), [VP2 §6.6](../vision_of_product/VP2-agent-platform/02-vision-agent-platform.md) (Gap 6 — auth/permissions)

## 1. Distribution Model

Loom is a **local-first CLI tool**. It runs on the developer's machine (or in a dev container) alongside VS Code.

| Artifact | Format | Target |
|----------|--------|--------|
| `loom` binary | Single Go binary (no CGo) | macOS (arm64, amd64), Linux (amd64, arm64) |
| Agent definitions | `.github/agents/*.agent.md` | Checked into target repo |
| MCP registration | `.github/copilot/mcp.json` | Checked into target repo |

### Binary Distribution

- **GitHub Releases**: `goreleaser` or manual cross-compilation produces binaries per platform.
- **Go install**: `go install github.com/guillaume7/loom/cmd/loom@latest`.
- No container image needed — the binary has no runtime dependencies.

## 2. Operational Concerns

### 2.1 State Location

| File | Location | Lifecycle |
|------|----------|-----------|
| `state.db` | `.loom/state.db` (project root) | Created on first `loom start`; stores checkpoints, action log, and session traces across restarts |
| `dependencies.yaml` | `.loom/dependencies.yaml` | Created by orchestrator or developer |
| `config.toml` | `~/.loom/config.toml` | Per-user, machine-scoped |
| `loom.log` | `~/.loom/loom.log` | Append-only structured JSON log |

### 2.2 Health Monitoring

| Signal | How | Consumer |
|--------|-----|----------|
| FSM state | `loom status` CLI / `loom_get_state` MCP tool | Human / agent |
| Stall detection | Monitor goroutine (5 min timeout → PAUSED) | Automatic |
| Heartbeat | `loom_heartbeat` MCP tool (auto-approved) | Orchestrator agent |
| Action history | `loom log` CLI / `loom://log` MCP resource | Human / agent |
| Session trace | `loom://sessions` and `loom://session/<id>` MCP resources | Human / agent |
| OS notification | VS Code notification on PAUSED | Human |

### 2.3 Recovery Procedures

| Failure | Recovery |
|---------|----------|
| Process crash | Restart `loom`; FSM resumes from last SQLite checkpoint |
| Agent session lost | `loom start` spawns new agent session; checkpoint is intact |
| Context window exhausted | `loom_checkpoint` before each step; new session resumes from checkpoint |
| Retry budget exhausted | MCP elicitation → operator chooses skip/reassign/pause |
| Unexpected run behavior | Open the retained session trace to replay FSM, GitHub, and operator events from one artifact |
| SQLite corruption | `loom reset` + manual re-creation (rare; WAL mode reduces risk) |

## 3. Organization-Level Deployment (P3)

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

## 4. Security

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
