package mcp

import (
	"context"
	"strings"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

func (s *Server) capabilityHooks() *mcpserver.Hooks {
	hooks := &mcpserver.Hooks{}
	hooks.AddBeforeInitialize(func(ctx context.Context, _ any, request *mcplib.InitializeRequest) {
		s.setSessionTaskSupport(sessionIDFromContext(ctx), clientSupportsTasks(request.Params.Capabilities))
	})
	return hooks
}

func clientSupportsTasks(capabilities mcplib.ClientCapabilities) bool {
	for key, value := range capabilities.Experimental {
		if !strings.EqualFold(strings.TrimSpace(key), "tasks") {
			continue
		}
		if capabilityValueEnabled(value) {
			return true
		}
	}
	return false
}

func capabilityValueEnabled(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case map[string]any:
		if enabled, ok := v["enabled"]; ok {
			e, ok := enabled.(bool)
			return ok && e
		}
		return true
	case nil:
		return false
	default:
		// Presence of the capability key with a non-bool/object value still
		// indicates support for forward-compatible client encodings.
		return true
	}
}
