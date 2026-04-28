package tests

import (
	"testing"

	mcpinfra "claude-go/internal/infra/mcp"
)

func TestMCPConnectionManager_AuthConnectPingAndTemplateRead(t *testing.T) {
	t.Parallel()

	manager := mcpinfra.CreateConnectionManager()
	manager.SetServers([]mcpinfra.ServerConfig{
		{
			Name:        "demo",
			Transport:   mcpinfra.TransportSDK,
			Enabled:     true,
			Auth:        "required",
			Channel:     "local",
			Description: "demo server",
			Tools: []mcpinfra.Tool{
				{Name: "workspace.echo", Response: "echo:{value}", ReadOnly: true},
			},
			Resources: []mcpinfra.Resource{
				{URI: "mcp://demo/readme", Content: "hello"},
			},
			Templates: []mcpinfra.Template{
				{URI: "mcp://demo/{name}", Description: "templated"},
			},
		},
	})

	if err := manager.ConnectServer("demo"); err == nil {
		t.Fatalf("expected auth-required connect to fail")
	}

	if err := manager.Authenticate("demo", "token-123"); err != nil {
		t.Fatalf("authenticate: %v", err)
	}

	status, err := manager.PingServer("demo")
	if err != nil {
		t.Fatalf("ping server: %v", err)
	}
	if status != mcpinfra.StatusConnected {
		t.Fatalf("unexpected ping status: %s", status)
	}

	out, err := manager.CallTool("demo", "workspace.echo", map[string]any{"value": "ok"})
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	if out != "echo:ok" {
		t.Fatalf("unexpected tool output: %q", out)
	}

	resource, err := manager.ReadResource("demo", "mcp://demo/user-guide")
	if err != nil {
		t.Fatalf("read templated resource: %v", err)
	}
	if resource.URI != "mcp://demo/user-guide" || resource.Content == "" {
		t.Fatalf("unexpected templated resource: %#v", resource)
	}
}

func TestMCPConnectionManager_ChannelPermissionsAndDisabledServer(t *testing.T) {
	t.Parallel()

	manager := mcpinfra.CreateConnectionManager()
	manager.SetPermissions(mcpinfra.ChannelPermissions{
		Allowed: map[string]bool{
			"local": false,
		},
	})
	manager.SetServers([]mcpinfra.ServerConfig{
		{
			Name:      "blocked",
			Transport: mcpinfra.TransportSDK,
			Enabled:   true,
			Channel:   "local",
		},
		{
			Name:      "disabled",
			Transport: mcpinfra.TransportSDK,
			Enabled:   false,
			Channel:   "sdk",
		},
	})

	if err := manager.ConnectServer("blocked"); err == nil {
		t.Fatalf("expected blocked channel connect to fail")
	}
	if err := manager.ConnectServer("disabled"); err == nil {
		t.Fatalf("expected disabled server connect to fail")
	}
}

