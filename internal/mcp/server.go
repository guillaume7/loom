// Package mcp implements the scaffolding for the Loom MCP stdio server.
//
// In the future this package will host the server that exposes the
// loom_next_step, loom_checkpoint, loom_heartbeat, loom_get_state, and
// loom_abort tools to the VS Code Copilot master session.
package mcp

// Server is a placeholder for the Loom MCP server implementation.
//
// As the MCP tools are implemented, this struct will grow fields for any
// dependencies they require. Prefer using NewServer so call sites do not
// need to change when wiring is added.
type Server struct{}

// NewServer constructs a Server.
//
// At present it returns an empty Server; dependency fields will be added as
// the MCP tool surface is implemented.
func NewServer() *Server {
	return &Server{}
}
