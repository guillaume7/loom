package mcp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/guillaume7/loom/internal/fsm"
	"github.com/guillaume7/loom/internal/mcp"
	"github.com/guillaume7/loom/internal/store"
	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --------------------------------------------------------------------------
// TH2.E3.US5: MCP server instructions
// --------------------------------------------------------------------------

func TestMCPServer_Instructions_ContainsStateAndPhase(t *testing.T) {
	machine := fsm.NewMachine(fsm.DefaultConfig())
	require.NoError(t, advanceMachine(machine,
		fsm.EventStart,
		fsm.EventPhaseIdentified,
		fsm.EventCopilotAssigned,
		fsm.EventPROpened,
		fsm.EventPRReady,
	))

	st := newMemStore()
	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{State: string(machine.State()), Phase: 3}))

	s := mcp.NewServer(machine, st, nil)
	result := callInitialize(t, s.MCPServer())

	assert.Contains(t, result.Instructions, "Current state: AWAITING_CI (phase 3)")
	assert.Contains(t, result.Instructions, "Phase summary: Poll CI check runs; call loom_checkpoint with action=ci_green or action=ci_red")
}

func TestMCPServer_Instructions_ContainsDependencyDigest(t *testing.T) {
	originalWD, err := os.Getwd()
	require.NoError(t, err)
	tempDir := t.TempDir()
	require.NoError(t, os.Chdir(tempDir))
	t.Cleanup(func() {
		require.NoError(t, os.Chdir(originalWD))
	})

	require.NoError(t, os.Mkdir(filepath.Join(tempDir, ".loom"), 0o755))
	deps := `version: 1
epics:
  - id: E1
    depends_on: []
    stories:
      - id: TH2.E3.US1
        depends_on: []
      - id: TH2.E3.US2
        depends_on: [TH2.E3.US1]
      - id: TH2.E3.US3
        depends_on: []
`
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, ".loom", "dependencies.yaml"), []byte(deps), 0o644))

	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newMemStore()
	s := mcp.NewServer(machine, st, nil)
	result := callInitialize(t, s.MCPServer())

	assert.Contains(t, result.Instructions, "Dependency digest:")
	assert.Contains(t, result.Instructions, "- Unblocked stories: TH2.E3.US1, TH2.E3.US3")
	assert.Contains(t, result.Instructions, "- Blocked stories: 1 story IDs blocked by incomplete dependencies")
}

func TestMCPServer_Instructions_NoDepFile_ShowsFallback(t *testing.T) {
	originalWD, err := os.Getwd()
	require.NoError(t, err)
	tempDir := t.TempDir()
	require.NoError(t, os.Chdir(tempDir))
	t.Cleanup(func() {
		require.NoError(t, os.Chdir(originalWD))
	})

	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newMemStore()
	s := mcp.NewServer(machine, st, nil)
	result := callInitialize(t, s.MCPServer())

	assert.Contains(t, result.Instructions, "Phase summary:")
	assert.Contains(t, result.Instructions, "Dependency digest:")
	assert.Contains(t, result.Instructions, "No dependency graph loaded")
}

func TestMCPServer_Instructions_RefreshesOnReinit(t *testing.T) {
	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newMemStore()
	s := mcp.NewServer(machine, st, nil)

	firstMCPServer := s.MCPServer()
	first := callInitialize(t, firstMCPServer)
	assert.Contains(t, first.Instructions, "Current state: IDLE")

	checkpoint := callTool(t, firstMCPServer, "loom_checkpoint", map[string]interface{}{"action": "start"})
	assert.False(t, checkpoint.IsError, "expected start checkpoint to succeed")

	second := callInitialize(t, s.MCPServer())
	assert.Contains(t, second.Instructions, "Current state: SCANNING")
	assert.NotEqual(t, first.Instructions, second.Instructions, "instructions should be recomputed on each MCPServer() call")
}

// --------------------------------------------------------------------------
// TH2.E3.US3: Built-in loom://state resource
// --------------------------------------------------------------------------

func TestLoomStateResource_ListIncludesURI(t *testing.T) {
	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newMemStore()
	s := mcp.NewServer(machine, st, nil)
	mcpSvr := s.MCPServer()

	ctx := context.Background()
	sess := newTestSession(nextSessionID())
	require.NoError(t, mcpSvr.RegisterSession(ctx, sess))
	ctx = mcpSvr.WithContext(ctx, sess)

	msg, err := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "resources/list",
		"params":  map[string]interface{}{},
	})
	require.NoError(t, err)

	raw := mcpSvr.HandleMessage(ctx, msg)
	require.NotNil(t, raw)

	resp, ok := raw.(mcplib.JSONRPCResponse)
	require.True(t, ok, "expected JSONRPCResponse, got %T", raw)

	result, ok := resp.Result.(mcplib.ListResourcesResult)
	require.True(t, ok, "expected ListResourcesResult, got %T", resp.Result)

	var uris []string
	for _, r := range result.Resources {
		uris = append(uris, r.URI)
	}
	assert.Contains(t, uris, "loom://state")
}

func TestLoomStateResource_ReadReturnsJSON(t *testing.T) {
	originalWD, err := os.Getwd()
	require.NoError(t, err)
	tempDir := t.TempDir()
	require.NoError(t, os.Chdir(tempDir))
	t.Cleanup(func() {
		require.NoError(t, os.Chdir(originalWD))
	})

	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newMemStore()
	s := mcp.NewServer(machine, st, nil)
	mcpSvr := s.MCPServer()

	resp := callResourceRead(t, mcpSvr, "loom://state")

	result, ok := resp.Result.(mcplib.ReadResourceResult)
	require.True(t, ok, "expected ReadResourceResult, got %T", resp.Result)
	require.Len(t, result.Contents, 1, "expected single resource content")

	tc, ok := result.Contents[0].(mcplib.TextResourceContents)
	require.True(t, ok, "expected TextResourceContents, got %T", result.Contents[0])
	assert.Equal(t, "loom://state", tc.URI)
	assert.Equal(t, "application/json", tc.MIMEType)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &body))

	assert.Contains(t, body, "state")
	assert.Contains(t, body, "phase")
	assert.Contains(t, body, "pr_number")
	assert.Contains(t, body, "issue_number")
	assert.Contains(t, body, "retry_count")
	assert.Contains(t, body, "updated_at")
	assert.Contains(t, body, "unblocked_stories")
	assert.Contains(t, body, "controller_state")
	assert.Contains(t, body, "driven_by")

	assert.Equal(t, "IDLE", body["state"])
	assert.Equal(t, float64(0), body["phase"])
	assert.Equal(t, float64(0), body["pr_number"])
	assert.Equal(t, float64(0), body["issue_number"])
	assert.Equal(t, float64(0), body["retry_count"])
	assert.Equal(t, "", body["updated_at"])
	assert.Equal(t, "idle", body["controller_state"])
	assert.Equal(t, "persisted_runtime_state", body["driven_by"])

	unblockedStories, ok := body["unblocked_stories"].([]interface{})
	require.True(t, ok, "expected unblocked_stories to be an array, got %T", body["unblocked_stories"])
	assert.Empty(t, unblockedStories)
}

func TestLoomStateResource_UnblockedStoriesBehavior(t *testing.T) {
	t.Run("missing dependencies file still includes empty unblocked_stories", func(t *testing.T) {
		originalWD, err := os.Getwd()
		require.NoError(t, err)
		tempDir := t.TempDir()
		require.NoError(t, os.Chdir(tempDir))
		t.Cleanup(func() {
			require.NoError(t, os.Chdir(originalWD))
		})

		machine := fsm.NewMachine(fsm.DefaultConfig())
		st := newMemStore()
		s := mcp.NewServer(machine, st, nil)
		mcpSvr := s.MCPServer()

		resp := callResourceRead(t, mcpSvr, "loom://state")
		result, ok := resp.Result.(mcplib.ReadResourceResult)
		require.True(t, ok, "expected ReadResourceResult, got %T", resp.Result)
		require.Len(t, result.Contents, 1)

		tc, ok := result.Contents[0].(mcplib.TextResourceContents)
		require.True(t, ok, "expected TextResourceContents, got %T", result.Contents[0])

		var body map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(tc.Text), &body))

		value, exists := body["unblocked_stories"]
		require.True(t, exists, "expected unblocked_stories key to be present")
		stories, ok := value.([]interface{})
		require.True(t, ok, "expected unblocked_stories to be an array, got %T", value)
		assert.Empty(t, stories)
	})

	t.Run("valid dependencies file returns unblocked story IDs", func(t *testing.T) {
		originalWD, err := os.Getwd()
		require.NoError(t, err)
		tempDir := t.TempDir()
		require.NoError(t, os.Chdir(tempDir))
		t.Cleanup(func() {
			require.NoError(t, os.Chdir(originalWD))
		})

		require.NoError(t, os.Mkdir(filepath.Join(tempDir, ".loom"), 0o755))
		deps := `version: 1
epics:
  - id: E2
    depends_on: []
    stories:
      - id: US-2.1
        depends_on: []
      - id: US-2.2
        depends_on: []
      - id: US-2.3
        depends_on: [US-2.1]
`
		require.NoError(t, os.WriteFile(filepath.Join(tempDir, ".loom", "dependencies.yaml"), []byte(deps), 0o644))

		machine := fsm.NewMachine(fsm.DefaultConfig())
		st := newMemStore()
		s := mcp.NewServer(machine, st, nil)
		mcpSvr := s.MCPServer()

		resp := callResourceRead(t, mcpSvr, "loom://state")
		result, ok := resp.Result.(mcplib.ReadResourceResult)
		require.True(t, ok, "expected ReadResourceResult, got %T", resp.Result)
		require.Len(t, result.Contents, 1)

		tc, ok := result.Contents[0].(mcplib.TextResourceContents)
		require.True(t, ok, "expected TextResourceContents, got %T", result.Contents[0])

		var body map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(tc.Text), &body))

		value, exists := body["unblocked_stories"]
		require.True(t, exists, "expected unblocked_stories key to be present")
		rawStories, ok := value.([]interface{})
		require.True(t, ok, "expected unblocked_stories to be an array, got %T", value)

		stories := make([]string, 0, len(rawStories))
		for _, raw := range rawStories {
			storyID, ok := raw.(string)
			require.True(t, ok, "expected story ID to be a string, got %T", raw)
			stories = append(stories, storyID)
		}

		assert.Equal(t, []string{"US-2.1", "US-2.2"}, stories)
	})
}

func TestLoomStateResource_ReflectsCurrentState(t *testing.T) {
	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newMemStore()
	s := mcp.NewServer(machine, st, nil)
	mcpSvr := s.MCPServer()

	before := callResourceRead(t, mcpSvr, "loom://state")
	beforeResult, ok := before.Result.(mcplib.ReadResourceResult)
	require.True(t, ok, "expected ReadResourceResult, got %T", before.Result)
	require.Len(t, beforeResult.Contents, 1)

	beforeText, ok := beforeResult.Contents[0].(mcplib.TextResourceContents)
	require.True(t, ok, "expected TextResourceContents, got %T", beforeResult.Contents[0])

	var beforeBody struct {
		State string `json:"state"`
	}
	require.NoError(t, json.Unmarshal([]byte(beforeText.Text), &beforeBody))
	assert.Equal(t, "IDLE", beforeBody.State)

	checkpoint := callTool(t, mcpSvr, "loom_checkpoint", map[string]interface{}{"action": "start"})
	assert.False(t, checkpoint.IsError, "expected start checkpoint to succeed")

	after := callResourceRead(t, mcpSvr, "loom://state")
	afterResult, ok := after.Result.(mcplib.ReadResourceResult)
	require.True(t, ok, "expected ReadResourceResult, got %T", after.Result)
	require.Len(t, afterResult.Contents, 1)

	afterText, ok := afterResult.Contents[0].(mcplib.TextResourceContents)
	require.True(t, ok, "expected TextResourceContents, got %T", afterResult.Contents[0])

	var afterBody struct {
		State string `json:"state"`
		Phase int    `json:"phase"`
	}
	require.NoError(t, json.Unmarshal([]byte(afterText.Text), &afterBody))
	assert.Equal(t, "SCANNING", afterBody.State)
	assert.Equal(t, 0, afterBody.Phase)
}

func TestLoomStateResource_RehydratesFromCheckpointAfterOutOfBandChange(t *testing.T) {
	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newMemStore()
	s := mcp.NewServer(machine, st, nil)
	mcpSvr := s.MCPServer()

	require.NoError(t, st.WriteCheckpoint(context.Background(), store.Checkpoint{
		State: "AWAITING_READY",
		Phase: 4,
	}))

	resp := callResourceRead(t, mcpSvr, "loom://state")
	result, ok := resp.Result.(mcplib.ReadResourceResult)
	require.True(t, ok, "expected ReadResourceResult, got %T", resp.Result)
	require.Len(t, result.Contents, 1)

	tc, ok := result.Contents[0].(mcplib.TextResourceContents)
	require.True(t, ok, "expected TextResourceContents, got %T", result.Contents[0])

	var body struct {
		State           string `json:"state"`
		Phase           int    `json:"phase"`
		ControllerState string `json:"controller_state"`
		DrivenBy        string `json:"driven_by"`
	}
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &body))
	assert.Equal(t, "AWAITING_READY", body.State)
	assert.Equal(t, 4, body.Phase)
	assert.Equal(t, "resuming", body.ControllerState)
	assert.Equal(t, "persisted_runtime_state", body.DrivenBy)
}

// --------------------------------------------------------------------------
// TH2.E3.US4: Built-in loom://log resource
// --------------------------------------------------------------------------

func TestLoomLogResource_ListIncludesURI(t *testing.T) {
	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newMemStore()
	s := mcp.NewServer(machine, st, nil)
	mcpSvr := s.MCPServer()

	ctx := context.Background()
	sess := newTestSession(nextSessionID())
	require.NoError(t, mcpSvr.RegisterSession(ctx, sess))
	ctx = mcpSvr.WithContext(ctx, sess)

	msg, err := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "resources/list",
		"params":  map[string]interface{}{},
	})
	require.NoError(t, err)

	raw := mcpSvr.HandleMessage(ctx, msg)
	require.NotNil(t, raw)

	resp, ok := raw.(mcplib.JSONRPCResponse)
	require.True(t, ok, "expected JSONRPCResponse, got %T", raw)

	result, ok := resp.Result.(mcplib.ListResourcesResult)
	require.True(t, ok, "expected ListResourcesResult, got %T", resp.Result)

	var uris []string
	for _, r := range result.Resources {
		uris = append(uris, r.URI)
	}
	assert.Contains(t, uris, "loom://log")
}

func TestLoomLogResource_EmptyLog_ReturnsEmptyContent(t *testing.T) {
	_, mcpSvr := newTestServer(t)

	resp := callResourceRead(t, mcpSvr, "loom://log")
	result, ok := resp.Result.(mcplib.ReadResourceResult)
	require.True(t, ok, "expected ReadResourceResult, got %T", resp.Result)
	require.Len(t, result.Contents, 1)

	tc, ok := result.Contents[0].(mcplib.TextResourceContents)
	require.True(t, ok, "expected TextResourceContents, got %T", result.Contents[0])
	assert.Equal(t, "loom://log", tc.URI)
	assert.Equal(t, "application/x-ndjson", tc.MIMEType)
	assert.Equal(t, "", tc.Text)
}

func TestLoomLogResource_ReturnsNDJSON(t *testing.T) {
	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newMemStore()

	require.NoError(t, st.WriteAction(context.Background(), store.Action{SessionID: "s1", OperationKey: "op1", StateBefore: "IDLE", StateAfter: "SCANNING", Event: "start", Detail: "d1"}))
	require.NoError(t, st.WriteAction(context.Background(), store.Action{SessionID: "s1", OperationKey: "op2", StateBefore: "SCANNING", StateAfter: "CODING", Event: "pr_opened", Detail: "d2"}))
	require.NoError(t, st.WriteAction(context.Background(), store.Action{SessionID: "s1", OperationKey: "op3", StateBefore: "CODING", StateAfter: "TESTING", Event: "ci_green", Detail: "d3"}))

	s := mcp.NewServer(machine, st, nil)
	mcpSvr := s.MCPServer()

	resp := callResourceRead(t, mcpSvr, "loom://log")
	result, ok := resp.Result.(mcplib.ReadResourceResult)
	require.True(t, ok, "expected ReadResourceResult, got %T", resp.Result)
	require.Len(t, result.Contents, 1)

	tc, ok := result.Contents[0].(mcplib.TextResourceContents)
	require.True(t, ok, "expected TextResourceContents, got %T", result.Contents[0])
	require.NotEmpty(t, tc.Text)

	lines := strings.Split(strings.TrimSpace(tc.Text), "\n")
	require.Len(t, lines, 3, "expected 3 NDJSON lines")

	for _, line := range lines {
		var obj map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(line), &obj), "line must be valid JSON: %s", line)
		assert.Contains(t, obj, "id")
		assert.Contains(t, obj, "session_id")
		assert.Contains(t, obj, "operation_key")
		assert.Contains(t, obj, "state_before")
		assert.Contains(t, obj, "state_after")
		assert.Contains(t, obj, "event")
		assert.Contains(t, obj, "detail")
		assert.Contains(t, obj, "created_at")
	}
}

func TestLoomLogResource_LimitTo200(t *testing.T) {
	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newMemStore()

	for i := 1; i <= 250; i++ {
		require.NoError(t, st.WriteAction(context.Background(), store.Action{
			SessionID:    "s1",
			OperationKey: fmt.Sprintf("op-%03d", i),
			StateBefore:  "A",
			StateAfter:   "B",
			Event:        "event",
			Detail:       "detail",
		}))
	}

	s := mcp.NewServer(machine, st, nil)
	mcpSvr := s.MCPServer()

	resp := callResourceRead(t, mcpSvr, "loom://log")
	result, ok := resp.Result.(mcplib.ReadResourceResult)
	require.True(t, ok, "expected ReadResourceResult, got %T", resp.Result)
	require.Len(t, result.Contents, 1)

	tc, ok := result.Contents[0].(mcplib.TextResourceContents)
	require.True(t, ok, "expected TextResourceContents, got %T", result.Contents[0])

	lines := strings.Split(strings.TrimSpace(tc.Text), "\n")
	require.Len(t, lines, 200, "expected last 200 NDJSON lines")

	seen := make(map[string]bool, 200)
	for i, line := range lines {
		var obj map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(line), &obj), "line must be valid JSON: %s", line)
		opRaw, ok := obj["operation_key"]
		require.True(t, ok, "operation_key field missing")
		op, ok := opRaw.(string)
		require.True(t, ok, "operation_key is not a string")

		expected := fmt.Sprintf("op-%03d", i+51)
		assert.Equal(t, expected, op, "expected ascending most-recent window")
		seen[op] = true
	}

	for i := 1; i <= 50; i++ {
		assert.False(t, seen[fmt.Sprintf("op-%03d", i)], "older action should not be present")
	}
	for i := 51; i <= 250; i++ {
		assert.True(t, seen[fmt.Sprintf("op-%03d", i)], "most recent action should be present")
	}
}
