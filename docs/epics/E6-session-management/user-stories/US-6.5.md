# US-6.5 — `loom_heartbeat` response includes `wait: true` and `retry_in_seconds`

## Epic
E6: Session Management

## Goal
Update the `loom_heartbeat` MCP tool to include `wait: true` and `retry_in_seconds: 30` in its response when the FSM is in a gate state, giving the Copilot session explicit guidance on how long to wait before calling again.

## Acceptance Criteria

```
Given the FSM is in state AWAITING_CI (a gate state)
When `loom_heartbeat` is called
Then the response JSON is `{"state":"AWAITING_CI","wait":true,"retry_in_seconds":30}`
```

```
Given the FSM is in state MERGING (a non-gate state)
When `loom_heartbeat` is called
Then the response JSON contains `"wait":false`
  And `retry_in_seconds` is absent or 0
```

```
Given `loom_heartbeat` is called 60 times in quick succession in a gate state
When responses are collected
Then every response contains `"wait":true` and `"retry_in_seconds":30`
  And the FSM state is unchanged after all 60 calls
```

## Tasks

1. [ ] Update `tools_heartbeat_test.go` to assert `wait` and `retry_in_seconds` fields (write test changes first)
2. [ ] Update `HeartbeatResponse` struct to add `RetryInSeconds int` and `Wait bool`
3. [ ] Update `handleHeartbeat` to populate `Wait=true` and `RetryInSeconds=30` for gate states
4. [ ] Ensure non-gate states return `Wait=false` and `RetryInSeconds=0`
5. [ ] Run `go test ./internal/mcp/... -race` and confirm green

## Dependencies
- US-6.1

## Size Estimate
S
