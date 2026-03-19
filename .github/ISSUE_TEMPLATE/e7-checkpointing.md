name: "E7 — Checkpointing"
about: Persist FSM state to SQLite after every transition so Loom can resume from crashes, reboots, or VS Code restarts.
title: "E7: Checkpointing"
labels: ["epic", "E7", "TH1"]
---

## Goal

Persist FSM state to SQLite after every transition so Loom can resume from any point after a crash, reboot, or VS Code restart.

## User Stories

- [ ] US-7.1 — Define SQLite schema: `checkpoints` table with state, phase, PR number, issue number, retry counts, timestamp
- [ ] US-7.2 — Implement `WriteCheckpoint` with upsert semantics
- [ ] US-7.3 — Implement `ReadCheckpoint` returning zero-value when empty
- [ ] US-7.4 — `loom start` reads checkpoint on startup, resumes from persisted state
- [ ] US-7.5 — `loom reset` deletes all rows (with confirmation)
- [ ] US-7.6 — SQLite tests using `":memory:"` database

## Acceptance Criteria

- [ ] `WriteCheckpoint` then `ReadCheckpoint` round-trip returns identical data
- [ ] Writing the same checkpoint twice is idempotent
- [ ] `ReadCheckpoint` on empty database returns zero-value Checkpoint, no error
- [ ] SQLite file created at `.loom/state.db` on first write (directory created if absent)
- [ ] `modernc.org/sqlite` used (no CGo dependency)
- [ ] All Store tests use `":memory:"` — no filesystem access
- [ ] `go test ./internal/store/... -race` exits 0
