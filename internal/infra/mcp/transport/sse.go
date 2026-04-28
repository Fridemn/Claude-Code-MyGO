package transport

import (
	"context"
	"fmt"
	"time"

	mcpclient "github.com/mark3labs/mcp-go/client"
	mcptransport "github.com/mark3labs/mcp-go/client/transport"
)

// ConnectSSE creates an MCP client connected to a remote server via SSE.
// It establishes the HTTP connection, performs the MCP initialization handshake,
// and returns a ready-to-use client.
func ConnectSSE(cfg Config) (*MCPConnection, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("mcp sse: url is required")
	}

	// Build client options
	var opts []mcptransport.ClientOption
	if len(cfg.Headers) > 0 {
		opts = append(opts, mcptransport.WithHeaders(cfg.Headers))
	}

	// Create the SSE MCP client
	client, err := mcpclient.NewSSEMCPClient(cfg.URL, opts...)
	if err != nil {
		return nil, fmt.Errorf("mcp sse: failed to create client for %s: %w", cfg.URL, err)
	}

	// Start the transport
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := client.Start(ctx); err != nil {
		client.Close()
		return nil, fmt.Errorf("mcp sse: failed to start %s: %w", cfg.URL, err)
	}

	// Perform initialization handshake
	initReq := initRequest()
	_, err = client.Initialize(ctx, initReq)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("mcp sse: initialization failed for %s: %w", cfg.URL, err)
	}

	return &MCPConnection{
		Client: client,
	}, nil
}
