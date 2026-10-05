package main

import (
	"context"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const serverVersion = "1.1.0"

// newServer builds the MCP server with every tool registered.
func newServer() *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "oido-shopify", Version: serverVersion}, nil)
	registerCore(s)
	registerThemes(s)
	registerCatalog(s)
	registerContent(s)
	registerCommerce(s)
	registerOrders(s)
	registerCustomers(s)
	return s
}

// RunMCPServer registers tools and serves over stdio.
func RunMCPServer() {
	log.Println("Oido Shopify MCP Server starting on stdio...")
	if err := newServer().Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
