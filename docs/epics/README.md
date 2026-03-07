# Loom — Epics

This document defines the implementation order and phase sequence for building
Loom itself.

> Loom is self-hosted: after E1 is bootstrapped manually, subsequent epics
> are meant to be driven by the `loom start` command. Loom builds itself.

---

## Implementation Order

```
E1 — Project Foundation
  ↓
E2 — State Machine          ← FSM; zero deps; fully testable
  ↓
E3 — GitHub Client          ← REST wrapper; httptest fixtures
  ↓
E4 — MCP Server             ← wires FSM + GitHub; exposes 5 tools
  ↓
E5 — CLI                    ← cobra; wraps MCP server + config
  ↓
E6 — Session Management     ← keepalive; stall detection; restart
  ↓
E7 — Checkpointing          ← SQLite; resume on restart
  ↓
E8 — Integration & Hardening ← E2E tests; retry budget exercises
```

---

## Epic Summary

| Epic | Goal | Key Output |
|---|---|---|
| **E1** | Repository scaffold, CI green, go.mod | Empty binary that compiles and CI passes |
| **E2** | Pure FSM with all 13 states and retry budgets | `internal/fsm` with 100% branch coverage |
| **E3** | GitHub REST wrapper that passes httptest tests | `internal/github` with all required API methods |
| **E4** | MCP server exposing 5 tools | `internal/mcp`; round-trip tool call tests |
| **E5** | CLI with 7 subcommands | `cmd/loom`; `loom --help` works; `loom mcp` starts server |
| **E6** | Keepalive heartbeat; stall detection; session restart | `loom start` stays alive during 30-minute CI poll |
| **E7** | SQLite checkpoint; resume from any state | `loom start` after kill resumes correctly |
| **E8** | Integration tests; cross-compilation; release tag | `loom start` drives a full simulated run |

---

## Phase → Branch Mapping

| Phase | Epic | Branch |
|---|---|---|
| 0 | E1 | `phase/0-foundation` |
| 1 | E2 | `phase/1-fsm` |
| 2 | E3 | `phase/2-github-client` |
| 3 | E4 | `phase/3-mcp-server` |
| 4 | E5 | `phase/4-cli` |
| 5 | E6 | `phase/5-session` |
| 6 | E7 | `phase/6-checkpointing` |
| 7 | E8 | `phase/7-integration` |

---

## Dependency Graph

| Epic | Depends On |
|---|---|
| E1 | — |
| E2 | E1 |
| E3 | E1 |
| E4 | E2, E3 |
| E5 | E4 |
| E6 | E4, E5 |
| E7 | E4 |
| E8 | E2, E3, E4, E5, E6, E7 |

E2 and E3 can be developed in parallel after E1 is merged.
