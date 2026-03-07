// Package mcp implements the Loom MCP stdio server.
//
// The server exposes the loom_next_step, loom_checkpoint, loom_heartbeat,
// loom_get_state, and loom_abort tools to the VS Code Copilot master session.
package mcp

// Server holds the wired-up dependencies for the Loom MCP server.
// Use NewServer to construct a Server rather than a struct literal.
type Server struct{}

// NewServer constructs a Server with all dependencies injected.
func NewServer() *Server {
	return &Server{}
}
