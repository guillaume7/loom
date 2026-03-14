# server.go function index

Source file: `internal/mcp/server.go` (673 lines total)

This index tracks which declarations belong to `server.go` alone and which are
duplicated in `handlers.go` and must be removed.

## Declarations to KEEP in server.go (lines 1–279)

These do **not** appear in `handlers.go`. They must stay.

| Symbol | Kind | Source range |
| --- | --- | --- |
| package doc + `package mcp` | comment + decl | 1–15 |
| imports | import block | 17–34 |
| `FSM` | interface | 38–48 |
| `resourceEntry` | struct | 50–53 |
| `Server` | struct | 56–70 |
| `Option` | type alias | 72 |
| `WithClock` | func | 76 |
| `WithMonitorConfig` | func | 79 |
| `NewServer` | func | 84–101 |
| `(s *Server) Store` | method | 102 |
| `(s *Server) AddResource` | method | 106–113 |
| `CheckpointRequest` | struct | 115–119 |
| `NextStepResult` | struct | 121–125 |
| `CheckpointResult` | struct | 127–132 |
| `HeartbeatResult` | struct | 134–140 |
| `GetStateResult` | struct | 142–146 |
| `AbortResult` | struct | 148–151 |
| `stateInstruction` | func | 153–187 |
| `toolResultJSON` | func | 189–196 |
| `(s *Server) readCheckpoint` | method | 199–208 |
| `checkCtx` | func | 210–216 |
| `marshalResultText` | func | 218–224 |
| `optionalStringArgument` | func | 226–236 |
| `sessionIDFromContext` | func | 238–244 |
| `(s *Server) readActionByOperationKey` | method | 246–256 |
| `(s *Server) writeActionOrReturnCached` | method | 258–279 |

---

## Removable block in server.go (lines 280–673)

All seven of these methods are **already declared in `handlers.go`** and cause
`method already declared` build errors. The entire block from line 280 to the
end of the file must be deleted.

| Symbol | server.go range | handlers.go range |
| --- | --- | --- |
| `(s *Server) handleNextStep` | 280–348 | 18–86 |
| `(s *Server) handleCheckpoint` | 349–490 | 87–228 |
| `(s *Server) handleHeartbeat` | 491–522 | 229–260 |
| `(s *Server) handleGetState` | 523–546 | 261–284 |
| `(s *Server) handleAbort` | 547–583 | 285–321 |
| `(s *Server) MCPServer` | 584–667 | 322–405 |
| `(s *Server) Serve` | 668–673 | 406–411 |

**Single removal action**: delete lines 280–673 from `server.go`.
After the deletion `server.go` ends at line 279 with the closing `}` of
`writeActionOrReturnCached`.

---

## Removal log

- Step 0: index created; no removals applied yet.
- Step 1: deleted lines 280–673 (all 7 duplicate methods) from `server.go`; also removed unused imports `"io"` and `"os"`. File is now 276 lines and builds cleanly.
