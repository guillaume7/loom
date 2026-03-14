package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/guillaume7/loom/internal/fsm"
	"github.com/guillaume7/loom/internal/mcp"
	"github.com/guillaume7/loom/internal/store"
	mcplib "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --------------------------------------------------------------------------
// E8: Coverage — handleAbort store failure, startMonitor goroutine, Serve
// --------------------------------------------------------------------------

// failingAbortStore fails on all writes, used to test handleAbort error path.
type failingAbortStore struct {
	memStore
}

func (s *failingAbortStore) WriteCheckpoint(_ context.Context, _ store.Checkpoint) error {
	return errors.New("simulated abort store failure")
}

func (s *failingAbortStore) Close() error { return nil }

func TestLoomAbort_StoreWriteFailure_ReturnsError(t *testing.T) {
	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := &failingAbortStore{}
	s := mcp.NewServer(machine, st, nil)
	mcpSvr := s.MCPServer()

	result := callTool(t, mcpSvr, "loom_abort", nil)
	assert.True(t, result.IsError, "expected tool error when abort store write fails")
}

// TestServe_CancelledContext verifies that Serve returns without error when
// the provided context is already cancelled. This exercises the Serve method
// and the startMonitor goroutine startup.
func TestServe_CancelledContext(t *testing.T) {
	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newMemStore()
	s := mcp.NewServer(machine, st, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	// Use a pipe so Serve has valid I/O but the context is already done.
	pr, pw := io.Pipe()
	defer pr.Close()
	defer pw.Close()

	var buf bytes.Buffer
	// Serve should return quickly because ctx is already cancelled.
	err := s.Serve(ctx, pr, &buf)
	// Accept either nil or a context error.
	if err != nil {
		assert.ErrorIs(t, err, context.Canceled)
	}
}

// --------------------------------------------------------------------------
// TH2.E3.US1: MCP resource registration framework
// --------------------------------------------------------------------------

// callResourceRead sends a resources/read JSON-RPC message to mcpSvr and returns
// the raw response. A protocol-level error fails the test.
func callResourceRead(t *testing.T, mcpSvr *mcpserver.MCPServer, uri string) mcplib.JSONRPCResponse {
	t.Helper()

	ctx := context.Background()
	sess := newTestSession(nextSessionID())
	require.NoError(t, mcpSvr.RegisterSession(ctx, sess))
	ctx = mcpSvr.WithContext(ctx, sess)

	msg, err := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "resources/read",
		"params": map[string]interface{}{
			"uri": uri,
		},
	})
	require.NoError(t, err)

	raw := mcpSvr.HandleMessage(ctx, msg)
	require.NotNil(t, raw)

	resp, ok := raw.(mcplib.JSONRPCResponse)
	require.True(t, ok, "expected JSONRPCResponse, got %T", raw)
	return resp
}

// callInitialize sends an initialize JSON-RPC message to mcpSvr and returns
// the typed InitializeResult. A protocol-level error fails the test.
func callInitialize(t *testing.T, mcpSvr *mcpserver.MCPServer) mcplib.InitializeResult {
	t.Helper()

	ctx := context.Background()
	sess := newTestSession(nextSessionID())
	require.NoError(t, mcpSvr.RegisterSession(ctx, sess))
	ctx = mcpSvr.WithContext(ctx, sess)

	msg, err := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": mcplib.LATEST_PROTOCOL_VERSION,
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "loom-test-client",
				"version": "0.0.0",
			},
		},
	})
	require.NoError(t, err)

	raw := mcpSvr.HandleMessage(ctx, msg)
	require.NotNil(t, raw)

	resp, ok := raw.(mcplib.JSONRPCResponse)
	require.True(t, ok, "expected JSONRPCResponse, got %T", raw)

	encoded, err := json.Marshal(resp.Result)
	require.NoError(t, err)

	var result mcplib.InitializeResult
	require.NoError(t, json.Unmarshal(encoded, &result))
	return result
}

func TestMCPServer_ResourceRegistration_ListResources(t *testing.T) {
	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newMemStore()
	s := mcp.NewServer(machine, st, nil)

	testResource := mcplib.NewResource("loom://test/resource", "Test Resource",
		mcplib.WithResourceDescription("A test resource"),
		mcplib.WithMIMEType("text/plain"),
	)
	s.AddResource(testResource, func(_ context.Context, req mcplib.ReadResourceRequest) ([]mcplib.ResourceContents, error) {
		return []mcplib.ResourceContents{
			mcplib.TextResourceContents{URI: req.Params.URI, MIMEType: "text/plain", Text: "hello"},
		}, nil
	})

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
	assert.Contains(t, uris, "loom://test/resource")
}

func TestMCPServer_ResourceRegistration_ReadResource(t *testing.T) {
	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newMemStore()
	s := mcp.NewServer(machine, st, nil)

	const resourceURI = "loom://test/hello"
	testResource := mcplib.NewResource(resourceURI, "Hello Resource",
		mcplib.WithMIMEType("text/plain"),
	)
	s.AddResource(testResource, func(_ context.Context, req mcplib.ReadResourceRequest) ([]mcplib.ResourceContents, error) {
		return []mcplib.ResourceContents{
			mcplib.TextResourceContents{URI: req.Params.URI, MIMEType: "text/plain", Text: "hello"},
		}, nil
	})

	mcpSvr := s.MCPServer()
	resp := callResourceRead(t, mcpSvr, resourceURI)

	result, ok := resp.Result.(mcplib.ReadResourceResult)
	require.True(t, ok, "expected ReadResourceResult, got %T", resp.Result)

	require.NotEmpty(t, result.Contents, "expected non-empty contents")
	tc, ok := result.Contents[0].(mcplib.TextResourceContents)
	require.True(t, ok, "expected TextResourceContents, got %T", result.Contents[0])
	assert.Equal(t, "hello", tc.Text)
}

func TestMCPServer_ResourceRegistration_UnknownURI(t *testing.T) {
	_, mcpSvr := newTestServer(t) // no resources registered

	ctx := context.Background()
	sess := newTestSession(nextSessionID())
	require.NoError(t, mcpSvr.RegisterSession(ctx, sess))
	ctx = mcpSvr.WithContext(ctx, sess)

	msg, err := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "resources/read",
		"params": map[string]interface{}{
			"uri": "loom://unknown/resource",
		},
	})
	require.NoError(t, err)

	raw := mcpSvr.HandleMessage(ctx, msg)
	require.NotNil(t, raw)

	// An unknown URI must produce either a JSONRPCError or a result with an error.
	_, isErr := raw.(mcplib.JSONRPCError)
	if !isErr {
		resp, ok := raw.(mcplib.JSONRPCResponse)
		require.True(t, ok, "expected JSONRPCResponse or JSONRPCError, got %T", raw)
		// If it's a response, there should be no valid resource contents.
		result, isResult := resp.Result.(mcplib.ReadResourceResult)
		if isResult {
			assert.Empty(t, result.Contents, "expected empty contents for unknown URI")
		}
	}
}

func advanceMachine(machine *fsm.Machine, events ...fsm.Event) error {
	for _, event := range events {
		if _, err := machine.Transition(event); err != nil {
			return err
		}
	}
	return nil
}

// --------------------------------------------------------------------------
// TH2.E3.US2: Built-in loom://dependencies resource
// --------------------------------------------------------------------------

func TestLoomDependenciesResource_ListIncludesURI(t *testing.T) {
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
	assert.Contains(t, uris, "loom://dependencies")
}

func TestLoomDependenciesResource_ReadReturnsYAML(t *testing.T) {
	originalWD, err := os.Getwd()
	require.NoError(t, err)
	tempDir := t.TempDir()
	require.NoError(t, os.Chdir(tempDir))
	t.Cleanup(func() {
		require.NoError(t, os.Chdir(originalWD))
	})

	require.NoError(t, os.Mkdir(filepath.Join(tempDir, ".loom"), 0o755))
	wantYAML := "epics:\n  - id: E1\n    stories:\n      - TH2.E3.US1\n      - TH2.E3.US2\n"
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, ".loom", "dependencies.yaml"), []byte(wantYAML), 0o644))

	machine := fsm.NewMachine(fsm.DefaultConfig())
	st := newMemStore()
	s := mcp.NewServer(machine, st, nil)
	mcpSvr := s.MCPServer()

	resp := callResourceRead(t, mcpSvr, "loom://dependencies")

	result, ok := resp.Result.(mcplib.ReadResourceResult)
	require.True(t, ok, "expected ReadResourceResult, got %T", resp.Result)
	require.Len(t, result.Contents, 1, "expected single resource content")

	tc, ok := result.Contents[0].(mcplib.TextResourceContents)
	require.True(t, ok, "expected TextResourceContents, got %T", result.Contents[0])
	assert.Equal(t, "loom://dependencies", tc.URI)
	assert.Equal(t, "text/yaml", tc.MIMEType)
	assert.Equal(t, wantYAML, tc.Text)
}

func TestLoomDependenciesResource_FileNotFound_ReturnsError(t *testing.T) {
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

	ctx := context.Background()
	sess := newTestSession(nextSessionID())
	require.NoError(t, mcpSvr.RegisterSession(ctx, sess))
	ctx = mcpSvr.WithContext(ctx, sess)

	msg, err := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "resources/read",
		"params": map[string]interface{}{
			"uri": "loom://dependencies",
		},
	})
	require.NoError(t, err)

	raw := mcpSvr.HandleMessage(ctx, msg)
	require.NotNil(t, raw)

	if rpcErr, ok := raw.(mcplib.JSONRPCError); ok {
		assert.Contains(t, strings.ToLower(rpcErr.Error.Message), "not found")
		return
	}

	resp, ok := raw.(mcplib.JSONRPCResponse)
	require.True(t, ok, "expected JSONRPCResponse or JSONRPCError, got %T", raw)
	payload, marshalErr := json.Marshal(resp)
	require.NoError(t, marshalErr)
	assert.True(t, strings.Contains(strings.ToLower(string(payload)), "not found"), "expected not found message in response payload: %s", string(payload))
}
