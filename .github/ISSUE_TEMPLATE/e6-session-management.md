name: "E6 — Session Management"
about: Keep the Copilot session alive during long async gates and detect stalls with recovery hooks.
title: "E6: Session Management"
labels: ["epic", "E6", "TH1"]
---

## Goal

Keep the VS Code Copilot session alive during long async gates and detect stalls — with automatic recovery where possible.

## User Stories

- [ ] US-6.1 — Heartbeat timer: Go binary emits periodic log entries during gate waits
- [ ] US-6.2 — Stall detection: detect no `loom_checkpoint` call for > N minutes while in a gate state
- [ ] US-6.3 — Stall response: write PAUSED checkpoint, log stall reason
- [ ] US-6.4 — `loom resume` re-opens the session from the last checkpoint
- [ ] US-6.5 — `loom_heartbeat` response includes `wait: true` and `retry_in_seconds` to guide the session

## Acceptance Criteria

- [ ] Go binary logs a heartbeat entry every 60 seconds during gate states
- [ ] Stall detected after configurable timeout (default: 5 minutes without tool call)
- [ ] On stall: state written to `PAUSED`, stall reason logged
- [ ] `loom_heartbeat` response always includes `{ "wait": true, "retry_in_seconds": 30 }` in gate states
- [ ] `loom_heartbeat` response returns `{ "wait": false }` in non-gate states
- [ ] Fake clock used in all stall-detection tests (no `time.Sleep`)
