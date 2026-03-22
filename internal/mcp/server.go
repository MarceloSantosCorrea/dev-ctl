package mcp

import (
	"github.com/mark3labs/mcp-go/server"

	"github.com/marcelo/devctl/internal/core/project"
)

// NewServer cria e configura o MCP server com todas as ferramentas do devctl.
func NewServer(svc *project.Service, templateDir string) *server.MCPServer {
	s := server.NewMCPServer(
		"devctl",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	h := &handlers{
		svc:         svc,
		templateDir: templateDir,
	}
	registerTools(s, h)

	return s
}
