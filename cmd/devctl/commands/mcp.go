package commands

import (
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"

	"github.com/marcelo/devctl/internal/config"
	devctlmcp "github.com/marcelo/devctl/internal/mcp"
)

func init() {
	rootCmd.AddCommand(mcpCmd)
}

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Inicia o MCP server (stdio) para integração com Claude Code",
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, cfg, cleanup, err := buildProjectService()
		if err != nil {
			return err
		}
		defer cleanup()

		templateDir := config.ResolveTemplatesDir(cfg)
		srv := devctlmcp.NewServer(svc, templateDir)
		return server.ServeStdio(srv)
	},
}
