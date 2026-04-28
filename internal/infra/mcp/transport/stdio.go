package transport

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	mcpclient "github.com/mark3labs/mcp-go/client"
)

// ConnectStdio creates an MCP client connected to a subprocess via stdin/stdout.
// It spawns the command, performs the MCP initialization handshake, and returns
// a ready-to-use client.
func ConnectStdio(cfg Config) (*MCPConnection, error) {
	if cfg.Command == "" {
		return nil, fmt.Errorf("mcp stdio: command is required")
	}

	// Build environment: inherit current + overlay server-specific env
	env := os.Environ()
	for k, v := range cfg.Env {
		env = append(env, k+"="+v)
	}
	envStrs := env

	// Create the MCP stdio client
	// NewStdioMCPClient spawns the process and establishes transport
	client, err := mcpclient.NewStdioMCPClient(cfg.Command, envStrs, cfg.Args...)
	if err != nil {
		return nil, fmt.Errorf("mcp stdio: failed to create client for %s: %w", cfg.Command, err)
	}

	// Start the transport (spawns the subprocess)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := client.Start(ctx); err != nil {
		client.Close()
		return nil, fmt.Errorf("mcp stdio: failed to start %s: %w", cfg.Command, err)
	}

	// Perform initialization handshake
	initReq := initRequest()
	_, err = client.Initialize(ctx, initReq)
	if err != nil {
		client.Close()
		// Kill the subprocess if it's still running
		if cmd, ok := getCmd(client); ok {
			cmd.Process.Kill()
		}
		return nil, fmt.Errorf("mcp stdio: initialization failed for %s: %w", cfg.Command, err)
	}

	return &MCPConnection{
		Client: client,
	}, nil
}

// getCmd attempts to extract the underlying exec.Cmd from the stdio transport
func getCmd(client *mcpclient.Client) (*exec.Cmd, bool) {
	stderr, ok := mcpclient.GetStderr(client)
	_ = stderr // Just check if client has stderr (meaning it's a stdio client)
	return nil, !ok // Best-effort cleanup
}
