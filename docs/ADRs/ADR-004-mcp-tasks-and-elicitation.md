# ADR-004: MCP Tasks and Elicitation for Resilient Polling and Failure Handling

## Status
Superseded by ADR-009

## Context

VP2 (§2 Gap 1, §2 Gap 5, §4.2, §4.3) identifies two related problems:

1. **Resilient polling**: `loom_heartbeat` calls that poll CI status or PR
   readiness are currently blocking MCP tool calls. If the client disconnects,
   the polling state is lost and must restart from scratch.

2. **Structured failure handling**: When a retry budget is exhausted, Loom
   currently transitions to `PAUSED` and requires the operator to run
   `loom resume` in a terminal. There is no in-chat mechanism for the operator
   to choose among recovery options.

MCP spec 2025-11-25 introduces **Tasks** (long-running tool calls with explicit
lifecycle events) and **Elicitations** (structured prompts requesting human
input). Both are available in VS Code v1.107+.

## Decision

### 1. MCP Task Wrapping for Polling

Wrap blocking `loom_heartbeat` calls as MCP Tasks:

- When `loom_heartbeat` enters a polling loop (CI status, PR readiness), it
  emits a `task/start` event with a unique task ID, title, and `cancellable: true`.
- Progress events fire every polling interval (default 30s) with current status.
- On completion, a `task/done` event carries the final result.
- If the client disconnects and reconnects, the task ID allows the client to
  re-attach to the in-progress poll.

```json
{"type": "task/start", "id": "loom-ci-poll-pr-42", "title": "Watching CI for PR #42", "cancellable": true}
{"type": "task/progress", "id": "loom-ci-poll-pr-42", "progress": "3/5 checks green, waiting on 'build'"}
{"type": "task/done", "id": "loom-ci-poll-pr-42", "result": {"all_green": true}}
```

Implementation: The `loom_heartbeat` tool handler in `internal/mcp/server.go`
checks whether the MCP client supports Tasks (capability negotiation). If yes,
it uses the Task lifecycle. If no, it falls back to the current blocking behavior.

### 2. MCP Elicitation on Budget Exhaustion

When a retry budget is exhausted, instead of immediately transitioning to `PAUSED`,
Loom issues an elicitation:

```json
{
  "type": "elicitation",
  "title": "PR #42 — CI budget exhausted",
  "schema": {
    "action": {
      "type": "string",
      "enum": ["skip", "reassign", "pause_epic"]
    }
  }
}
```

The Go binary maps each response to an FSM event:
- `skip` → advance to next story (new event: `skip_story`)
- `reassign` → close current PR, create new issue (reset to `ISSUE_CREATED`)
- `pause_epic` → transition to `PAUSED` (existing behavior)

If the MCP client does not support elicitations, Loom falls back to the existing
`PAUSED` transition with a log message.

### VP2 Traceability

| VP2 Section | Requirement | How Addressed |
|---|---|---|
| §2 Gap 1 | Event/timer runtime for persistent polling | MCP Task wrapping with reconnect |
| §2 Gap 5 | Bounded retry + structured escalation | MCP elicitation with enum choices |
| §4.2 | MCP Task wrapping specification | Task lifecycle events for `loom_heartbeat` |
| §4.3 | Elicitation schema specification | Structured elicitation on budget exhaustion |

## Consequences

### Positive
- Polling survives client disconnects — no lost state on reconnect.
- Operators get structured choices in the chat UI — no terminal switching.
- Graceful degradation — falls back to v1 behavior if Tasks/elicitation not supported.

### Negative
- MCP Tasks spec (2025-11-25) is new. The `mcp-go` library may need updates to
  support task lifecycle events. Mitigation: implement task events as raw JSON
  messages if the library lacks typed support.
- Elicitation adds a new FSM event (`skip_story`) that does not exist in v1.
  Mitigation: additive change to the event set; existing transitions unchanged.

### Risks
- VS Code's MCP Task/elicitation support may have implementation gaps.
  Mitigation: capability negotiation + fallback. Never depend on Tasks for correctness.

## Alternatives Considered

### A. Keep blocking tool calls (v1 status quo)
- Pros: Simple; no protocol dependency.
- Cons: Client disconnect loses polling progress; no structured failure choices.
- Rejected because: VP2 explicitly requires resilient polling and structured escalation.

### B. WebSocket-based polling outside MCP
- Pros: Full control over connection lifecycle.
- Cons: Requires custom transport; MCP clients won't understand it.
- Rejected because: MCP Tasks are the standard mechanism for this exact use case.

### C. OS-level notifications only (no elicitation)
- Pros: Works without MCP elicitation support.
- Cons: Operator must switch to terminal to respond.
- Rejected because: VP2 §4.3 specifies in-chat structured choices.
