// Package mcpserver implements an MCP (Model Context Protocol) server that
// exposes MartMart grocery operations as tools consumable by AI agents.
package mcpserver

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/wydrox/martmart-cli/internal/session"
)

// serverVersion is reported to MCP clients as the server implementation version.
const serverVersion = "0.1.0"

var mcpDefaultProvider = session.ProviderFrisco

func setMCPDefaultProvider(provider string) {
	provider = session.NormalizeProvider(provider)
	if provider == "" {
		provider = session.ProviderFrisco
	}
	if err := session.ValidateProvider(provider); err != nil {
		provider = session.ProviderFrisco
	}
	mcpDefaultProvider = provider
}

// New builds an MCP server exposing MartMart tools.
func New(provider string) *mcp.Server {
	setMCPDefaultProvider(provider)
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "martmart",
		Version: serverVersion,
	}, nil)

	registerCartAndProductsTools(server)
	registerOrdersAndReservationTools(server)
	registerAccountSessionAuthTools(server)

	return server
}
