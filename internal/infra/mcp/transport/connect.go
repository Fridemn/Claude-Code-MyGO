package transport

import mcpclient "github.com/mark3labs/mcp-go/client"

// MCPConnection wraps an MCP client with connection metadata
type MCPConnection struct {
	Client *mcpclient.Client
	Name   string
}

// Config holds the parameters needed to establish an MCP connection
type Config struct {
	Transport string
	Command   string
	Args      []string
	Env       map[string]string
	URL       string
	Headers   map[string]string
}

// Connect establishes an MCP connection based on the transport type.
// Returns an MCPConnection with a live client, or an error.
func Connect(cfg Config) (*MCPConnection, error) {
	switch cfg.Transport {
	case "stdio":
		return ConnectStdio(cfg)
	case "sse", "sse-ide":
		return ConnectSSE(cfg)
	case "http":
		return ConnectHTTP(cfg)
	case "sdk", "":
		return nil, nil // SDK transport is in-process, no external connection needed
	default:
		return nil, nil // Unknown transport, skip silently
	}
}
