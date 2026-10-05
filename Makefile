.PHONY: build clean test

PLUGIN_NAME := oido-shopify
BINARY := $(PLUGIN_NAME)-mcp

build:
	@echo "Building $(PLUGIN_NAME) MCP server..."
	CGO_ENABLED=0 go build -o $(BINARY) .
	@echo "✓ Built: $(BINARY)"

test:
	go test -race ./...

clean:
	rm -f $(BINARY)
