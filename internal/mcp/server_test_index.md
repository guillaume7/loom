# server_test.go function index

Source file: `internal/mcp/server_test.go`

This index tracks:
- original source range in `server_test.go`
- mapped target range in a `server_*_test.go` file, if any
- current removable block ranges in `server_test.go`
- removal status as blocks are deleted sequentially

## Current removable blocks in server_test.go

| Block | Current source range | Target file | Status |
| --- | --- | --- | --- |
| Core mapped block | removed | `internal/mcp/server_core_test.go` | removed in step 1 |
| Atomic mapped block | removed | `internal/mcp/server_atomic_test.go` | removed in step 2 |
| Monitor mapped block | removed | `internal/mcp/server_monitor_test.go` | removed in step 3 |
| Misc mapped block | removed | `internal/mcp/server_misc_test.go` | removed in step 4 |

## Unmapped helpers kept in server_test.go

| Function | Current source range | Target | Status |
| --- | --- | --- | --- |
| `newMemStore` | 33-34 | none | keep |
| `(s *memStore) ReadCheckpoint` | 35-43 | none | keep |
| `(s *memStore) WriteCheckpoint` | 44-51 | none | keep |
| `(s *memStore) WriteAction` | 52-67 | none | keep |
| `(s *memStore) WriteCheckpointAndAction` | 68-85 | none | keep |
| `(s *memStore) ReadActionByOperationKey` | 86-97 | none | keep |
| `(s *memStore) ReadActions` | 98-113 | none | keep |
| `(s *memStore) DeleteAll` | 114-122 | none | keep |
| `(s *memStore) Close` | 123-129 | none | keep |
| `newFailingStore` | 130-131 | none | keep |
| `(s *failingStore) WriteCheckpoint` | 132-135 | none | keep |
| `(s *failingStore) WriteCheckpointAndAction` | 136-139 | none | keep |
| `(s *failingStore) Close` | 140-147 | none | keep |
| `newDuplicateWithoutCachedResultStore` | 148-151 | none | keep |
| `(s *duplicateWithoutCachedResultStore) WriteCheckpointAndAction` | 152-155 | none | keep |
| `(s *duplicateWithoutCachedResultStore) ReadActionByOperationKey` | 156-168 | none | keep |
| `newTestSession` | 169-175 | none | keep |
| `(s *testSession) Initialize` | 176-176 | none | keep |
| `(s *testSession) Initialized` | 177-177 | none | keep |
| `(s *testSession) NotificationChannel` | 178-178 | none | keep |
| `(s *testSession) SessionID` | 179-179 | none | keep |
| `nextSessionID` | 191-196 | none | keep |
| `newTestServer` | 197-207 | none | keep |
| `callTool` | 208-246 | none | keep |
| `callToolConcurrent` | 247-284 | none | keep |
| `toolText` | 285-294 | none | keep |

## Core mapped functions

| Function | Source range | Target range | Status |
| --- | --- | --- | --- |
| `TestNewServer_ReturnsNonNil` | 298-302 | `server_core_test.go:15-19` | removed from source in step 1 |
| `TestToolsList_RegistersAllToolsWithSchemas` | 303-344 | `server_core_test.go:20-61` | removed from source in step 1 |
| `TestLoomNextStep_ReturnsStateAndInstruction` | 345-357 | `server_core_test.go:62-74` | removed from source in step 1 |
| `TestLoomCheckpoint_ValidAction_AdvancesState` | 358-372 | `server_core_test.go:75-89` | removed from source in step 1 |
| `TestLoomCheckpoint_BackwardCompatEvent_AdvancesState` | 373-388 | `server_core_test.go:90-105` | removed from source in step 1 |
| `TestLoomCheckpoint_InvalidAction_ReturnsError` | 389-397 | `server_core_test.go:106-114` | removed from source in step 1 |
| `TestLoomCheckpoint_MissingAction_ReturnsError` | 398-405 | `server_core_test.go:115-122` | removed from source in step 1 |
| `TestLoomCheckpoint_StoreWriteFailure_ReturnsError` | 406-418 | `server_core_test.go:123-135` | removed from source in step 1 |
| `TestLoomCheckpoint_NonIdempotentStoreWriteFailure_RollsBackFSM` | 419-437 | `server_core_test.go:136-154` | removed from source in step 1 |
| `TestLoomNextStep_Idempotency_RetryReturnsCachedResult` | 438-459 | `server_core_test.go:155-176` | removed from source in step 1 |
| `TestLoomCheckpoint_Idempotency_FirstExecutionLogsAction` | 460-482 | `server_core_test.go:177-199` | removed from source in step 1 |
| `TestLoomCheckpoint_Idempotency_RetryReturnsCachedResult` | 483-497 | `server_core_test.go:200-214` | removed from source in step 1 |
| `TestLoomCheckpoint_Idempotency_DifferentOperationKeyExecutes` | 498-537 | `server_core_test.go:215-241` | removed from source in step 1 |

## Atomic mapped functions

| Function | Source range | Target range | Status |
| --- | --- | --- | --- |
| `newTransientFailStore` | 538-541 | `server_atomic_test.go:30-33` | removed from source in step 2 |
| `(s *transientFailStore) WriteCheckpointAndAction` | 542-555 | `server_atomic_test.go:34-47` | removed from source in step 2 |
| `TestLoomCheckpoint_AtomicWriteFailure_LeavesStoreConsistent` | 556-581 | `server_atomic_test.go:48-73` | removed from source in step 2 |
| `TestLoomCheckpoint_AtomicWriteFailure_StoreWriteFailure_WithOperationKey` | 582-602 | `server_atomic_test.go:74-94` | removed from source in step 2 |
| `TestLoomCheckpoint_SameProcessRetry_AfterTransientWriteFailure` | 603-642 | `server_atomic_test.go:95-134` | removed from source in step 2 |
| `TestLoomCheckpoint_SameProcessRetry_CountersRolledBack` | 643-687 | `server_atomic_test.go:135-179` | removed from source in step 2 |
| `newDuplicateOnWriteStore` | 688-691 | `server_atomic_test.go:180-183` | removed from source in step 2 |
| `(s *duplicateOnWriteStore) WriteCheckpointAndAction` | 692-718 | `server_atomic_test.go:184-210` | removed from source in step 2 |
| `TestLoomCheckpoint_AtomicDuplicateOnWrite_ReturnsCachedResult` | 719-755 | `server_atomic_test.go:211-247` | removed from source in step 2 |
| `TestLoomCheckpoint_IdempotentDuplicateWithoutCachedResult_RollsBackFSM` | 756-776 | `server_atomic_test.go:248-268` | removed from source in step 2 |
| `TestReadOnlyTools_SkipIdempotencyLookup` | 777-791 | `server_atomic_test.go:269-283` | removed from source in step 2 |
| `TestLoomHeartbeat_ReturnsCurrentState` | 792-806 | `server_atomic_test.go:284-298` | removed from source in step 2 |
| `TestLoomGetState_ReturnsState` | 807-818 | `server_atomic_test.go:299-310` | removed from source in step 2 |
| `TestLoomAbort_TransitionsToPaused` | 819-830 | `server_atomic_test.go:311-322` | removed from source in step 2 |
| `TestServer_RaceCondition` | 831-882 | `server_atomic_test.go:323-362` | removed from source in step 2 |

## Monitor mapped functions

| Function | Source range | Target range | Status |
| --- | --- | --- | --- |
| `newFakeClock` | 883-886 | `server_monitor_test.go:28-31` | removed from source in step 3 |
| `(c *fakeClock) Now` | 887-893 | `server_monitor_test.go:32-38` | removed from source in step 3 |
| `(c *fakeClock) Advance` | 894-902 | `server_monitor_test.go:39-47` | removed from source in step 3 |
| `newTestServerWithClock` | 903-914 | `server_monitor_test.go:48-59` | removed from source in step 3 |
| `TestLoomHeartbeat_GateState_ReturnsWaitTrue` | 915-933 | `server_monitor_test.go:60-78` | removed from source in step 3 |
| `TestLoomHeartbeat_NonGateState_ReturnsWaitFalse` | 934-954 | `server_monitor_test.go:79-99` | removed from source in step 3 |
| `TestRunStallCheck_GateState_Stall_WritesPaused` | 955-975 | `server_monitor_test.go:100-120` | removed from source in step 3 |
| `TestRunStallCheck_GateState_WithinTimeout_ReturnsFalse` | 976-996 | `server_monitor_test.go:121-141` | removed from source in step 3 |
| `TestRunStallCheck_NonGateState_ReturnsFalse` | 997-1010 | `server_monitor_test.go:142-155` | removed from source in step 3 |
| `TestRunStallCheck_CheckpointResetsStallTimer` | 1011-1033 | `server_monitor_test.go:156-178` | removed from source in step 3 |
| `TestRunStallCheck_TOCTOU_CheckpointArrivesBeforeLock_ReturnsFalse` | 1034-1076 | `server_monitor_test.go:179-221` | removed from source in step 3 |
| `TestLoomNextStep_AllStates` | 1077-1131 | `server_monitor_test.go:222-276` | removed from source in step 3 |
| `TestWithMonitorConfig_AppliesConfig` | 1132-1163 | `server_monitor_test.go:277-298` | removed from source in step 3 |

## Misc mapped functions

| Function | Source range | Target range | Status |
| --- | --- | --- | --- |
| `(s *failingAbortStore) WriteCheckpoint` | 1164-1167 | `server_misc_test.go:29-32` | removed from source in step 4 |
| `(s *failingAbortStore) Close` | 1168-1169 | `server_misc_test.go:33-34` | removed from source in step 4 |
| `TestLoomAbort_StoreWriteFailure_ReturnsError` | 1170-1182 | `server_misc_test.go:35-47` | removed from source in step 4 |
| `TestServe_CancelledContext` | 1183-1210 | `server_misc_test.go:48-75` | removed from source in step 4 |
| `callResourceRead` | 1211-1236 | `server_misc_test.go:76-101` | removed from source in step 4 |
| `TestMCPServer_ResourceRegistration_ListResources` | 1237-1282 | `server_misc_test.go:102-147` | removed from source in step 4 |
| `TestMCPServer_ResourceRegistration_ReadResource` | 1283-1309 | `server_misc_test.go:148-174` | removed from source in step 4 |
| `TestMCPServer_ResourceRegistration_UnknownURI` | 1310-1342 | `server_misc_test.go:175-414` | removed from source in step 4 |

## Removal log

- Step 0: index created; no removals applied yet.
- Step 1: removed the core mapped block from `server_test.go` original range 298-537; atomic, monitor, and misc block current ranges were shifted upward.
- Step 2: removed the atomic mapped block from `server_test.go` original range 538-882; monitor and misc block current ranges were shifted upward based on the live file after step 1.
- Step 3: removed the monitor mapped block from `server_test.go` original range 883-1163; the misc block current range was recomputed from the live file after step 2.
- Step 4: removed the misc mapped block from `server_test.go` original range 1164-1342; `server_test.go` now contains only shared helper functions and is 294 lines long.