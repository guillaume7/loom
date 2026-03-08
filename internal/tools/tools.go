// Package tools contains compile-only import tests that ensure every
// approved direct dependency is declared in go.mod.  If a dependency is
// missing the package will fail to compile, surfacing the omission at
// build time rather than at runtime.
package tools

import (
	// GitHub REST API client used by internal/github.
	_ "github.com/google/go-github/v68/github"
	// MCP stdio server toolkit used by internal/mcp.
	_ "github.com/mark3labs/mcp-go/mcp"
	// TOML config parser used by internal/config.
	_ "github.com/pelletier/go-toml/v2"
	// Pure-Go SQLite driver used by internal/store.
	_ "modernc.org/sqlite"
)
