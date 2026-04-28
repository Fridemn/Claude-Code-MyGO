package tests

import (
	"context"
	"strings"
	"testing"

	"claude-go/internal/command"
	cmdintegration "claude-go/internal/command/integration"
)

func TestCommandRegistry_MCPCommands(t *testing.T) {
	t.Parallel()

	registry := command.EmptyRegistry()
	cmdintegration.Register(registry)

	runtime := command.Runtime{
		PingMCP: func(server string) (string, bool) {
			if server == "local-workspace" {
				return "connected", true
			}
			return "failed", false
		},
		RestartMCP: func(server string) bool {
			return server == "local-workspace"
		},
		MCPSearchTools: func(query string) []command.MCPToolMatchInfo {
			return []command.MCPToolMatchInfo{
				{
					Server:      "local-workspace",
					Name:        "workspace.echo",
					Description: "Echo a value",
					ReadOnly:    true,
				},
			}
		},
		MCPStatus: func() string {
			return "mcp=configured"
		},
		MCPServers: func() []command.MCPServerInfo {
			return []command.MCPServerInfo{
				{
					Name:          "local-workspace",
					Transport:     "sdk",
					Status:        "connected",
					ToolCount:     2,
					ResourceCount: 2,
					Enabled:       true,
					Connected:     true,
				},
			}
		},
	}

	out, ok, err := registry.Execute(context.Background(), "/mcp-ping local-workspace", runtime)
	if err != nil || !ok {
		t.Fatalf("mcp-ping failed: ok=%t err=%v", ok, err)
	}
	if !strings.Contains(out.Value, "status=connected") {
		t.Fatalf("unexpected ping output: %s", out.Value)
	}

	out, ok, err = registry.Execute(context.Background(), "/mcp-restart local-workspace", runtime)
	if err != nil || !ok {
		t.Fatalf("mcp-restart failed: ok=%t err=%v", ok, err)
	}
	if !strings.Contains(out.Value, "mcp server restarted") {
		t.Fatalf("unexpected restart output: %s", out.Value)
	}

	out, ok, err = registry.Execute(context.Background(), "/mcp-search workspace", runtime)
	if err != nil || !ok {
		t.Fatalf("mcp-search failed: ok=%t err=%v", ok, err)
	}
	if !strings.Contains(out.Value, "local-workspace:workspace.echo") {
		t.Fatalf("unexpected search output: %s", out.Value)
	}

	out, ok, err = registry.Execute(context.Background(), "/mcp", runtime)
	if err != nil || !ok {
		t.Fatalf("/mcp failed: ok=%t err=%v", ok, err)
	}
	if !strings.Contains(out.Value, "overview:") || !strings.Contains(out.Value, "actions:") {
		t.Fatalf("unexpected /mcp output: %s", out.Value)
	}
}
