package main

import "log"

// Oido Shopify MCP server. The store domain and Admin API access token are
// injected by oido-core as SHOPIFY_STORE / SHOPIFY_ACCESS_TOKEN from the
// extension settings; this process consumes them.
func main() {
	log.Println("Starting Oido Shopify MCP Server v1.0.0...")
	RunMCPServer()
}
