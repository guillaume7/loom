# ADR-006: Security Model — Tool Eligibility, Auth, and Org Registry

## Status
Proposed

## Context

VP2 (§2 Gap 6) addresses auth and permission hardening for Loom. The original
gap analysis identified three concerns:

1. **Token permissions**: Ensuring the GitHub token has minimum required scopes.
2. **Branch protection alignment**: Merge operations must respect branch protection rules.
3. **Tool approval**: Destructive tools should not be auto-approved.

VS Code v1.106–v1.107 introduced:
- **MCP CIMD auth**: Client ID Metadata Document flow for remote MCP connections.
- **Organization MCP registry**: Centrally managed MCP server versions.
- **Tool eligibility policy**: `chat.tools.eligibleForAutoApproval` setting.
- **Post-approval for external data**: Tool calls that pull external data go
  through confirmation.

## Decision

### 1. Tool Eligibility Classification

Every Loom MCP tool is annotated with `readOnlyHint` to control auto-approval:

| Tool | `readOnlyHint` | Auto-Approve | Rationale |
|------|---------------|-------------|-----------|
| `loom_get_state` | `true` | Yes | Pure read; no side effects |
| `loom_heartbeat` | `true` | Yes | Pure read; polling only |
| `loom_next_step` | `false` | No | Returns action plan; precedes write operations |
| `loom_checkpoint` | `false` | No | Mutates FSM state; advances workflow |
| `loom_abort` | `false` | No | Forces PAUSED; disruptive |

Enterprise admins can further restrict via `chat.tools.eligibleForAutoApproval`
to require manual approval even for read-only tools.

### 2. Agent Tool Constraints

Tool isolation is enforced at the agent definition level (ADR-002):

- **Gate agent**: Only read tools — cannot create issues, merge PRs, or checkpoint.
- **Debug agent**: Read + comment — can post comments but cannot merge or checkpoint.
- **Merge agent**: Merge operation only — cannot create issues or modify FSM beyond merge.
- **Orchestrator**: Full tool set — the only agent that can drive the complete loop.

This is defense-in-depth: even if an agent's LLM reasoning goes off-track, the
tool set boundary prevents unauthorized operations.

### 3. GitHub Token Scope Validation

On `loom start`, the Go binary validates the GitHub token has the minimum required
scopes by calling the GitHub `/user` endpoint and inspecting `X-OAuth-Scopes`:

- Required: `repo` (issues, PRs, checks, merges)
- Optional: `read:org` (org membership, for org-level features)

If the token lacks required scopes, `loom start` fails with a clear error message.

### 4. Secret Protection

- `LOOM_TOKEN` is excluded from all `slog` output (the Config struct's Token field
  is never passed to any log call).
- `~/.loom/config.toml` should have file permissions `0600`. The `loom start`
  command warns if permissions are too open.
- The token is never passed to agent sessions — agents call GitHub via the MCP
  server, which holds the token in-process.

### 5. Organization Registry (P3)

For team deployments, Loom is published to the organization's private MCP registry.
This ensures:
- All team members use the same vetted server version.
- The registry version is updated by an admin, not individual developers.
- Registry entries can specify required token scopes.

### VP2 Traceability

| VP2 Section | Requirement | How Addressed |
|---|---|---|
| §2 Gap 6 | Token/app permissions | Token scope validation on `loom start` |
| §2 Gap 6 | Merge strategy alignment | Merge tool respects branch protection (GitHub API enforces) |
| §2 Gap 6 | MCP CIMD auth | Future remote MCP connections (P3) |
| §2 Gap 6 | Org MCP registry | P3 deliverable |
| §2 Gap 6 | Tool eligibility policy | `readOnlyHint` annotations + `eligibleForAutoApproval` |

## Consequences

### Positive
- Read-only tools are auto-approved — no human bottleneck during polling.
- Destructive tools always require approval — prevents accidental state corruption.
- Agent-level tool constraints provide defense-in-depth.
- Token scope validation catches misconfigurations early.

### Negative
- Manual approval for `loom_checkpoint` adds latency to every FSM transition.
  Mitigation: operators can enable auto-approval for `loom_checkpoint` if they
  trust the orchestrator agent.
- Config file permission checking is OS-dependent (no-op on Windows).

### Risks
- Tool eligibility policy is enterprise-managed. Solo developers may not have
  access to configure it. Mitigation: defaults are safe; solo users can
  override via VS Code settings.

## Alternatives Considered

### A. GitHub App authentication (instead of PAT)
- Pros: Fine-grained permissions, installation-scoped, no personal token.
- Cons: Requires GitHub App registration; adds OAuth complexity for local tool.
- Rejected because: Disproportionate complexity for a local-first CLI tool. Can
  be added as a future enhancement without architectural changes (the `Client`
  interface abstracts the auth mechanism).

### B. No tool eligibility annotations
- Pros: Simpler MCP server implementation.
- Cons: All tools require manual approval, including polling — unusable for
  autonomous operation.
- Rejected because: VP2 Gap 6 explicitly requires auto-approval for read-only tools.

### C. Separate read-only and read-write MCP servers
- Pros: Hard isolation between read and write operations.
- Cons: Two server processes; agent sessions need two MCP connections.
- Rejected because: Tool-level `readOnlyHint` provides the same guarantee with
  less operational complexity.
