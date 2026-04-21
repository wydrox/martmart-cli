package commands

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"github.com/wydrox/martmart-cli/internal/mcpserver"
)

func newMCPCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Run MCP server on stdio transport.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			provider, err := selectedProvider(cmd)
			if err != nil {
				return err
			}
			server := mcpserver.New(provider)
			return server.Run(context.Background(), &mcp.StdioTransport{})
		},
	}
}
