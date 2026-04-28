package tests

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"claude-go/internal/engine"
	"claude-go/internal/infra/mcp"
	mcptool "claude-go/internal/tool/mcp"
	"claude-go/internal/services"
	"claude-go/internal/tool"
)

func TestMCPServiceLifecycle(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	configPath := filepath.Join(root, "mcp.json")
	service := services.CreateMCPService(configPath)

	// Add a mock server for testing (no default servers anymore)
	service.AddServer(services.MCPServer{
		Name:        "local-workspace",
		Transport:   mcp.TransportSDK,
		Enabled:     true,
		Channel:     "local",
		Description: "local workspace mcp server",
		Tools: []mcp.Tool{
			{Name: "workspace.echo", Response: "{value}", ReadOnly: true},
		},
		Resources: []mcp.Resource{
			{URI: "mcp://local-workspace/readme", Content: "hello"},
		},
		Templates: []mcp.Template{
			{URI: "mcp://local-workspace/{name}", Description: "templated"},
		},
	})

	servers := service.Servers()
	if len(servers) == 0 {
		t.Fatalf("expected mock mcp servers after adding")
	}

	if ok := service.Connect("local-workspace"); !ok {
		t.Fatalf("expected local-workspace to connect")
	}

	toolsList := service.ListTools("local-workspace")
	if len(toolsList) == 0 {
		t.Fatalf("expected local-workspace tools")
	}

	out, err := service.CallTool("local-workspace", "workspace.echo", map[string]any{"value": "hello"})
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	if out != "hello" && out != "echo:hello" {
		t.Fatalf("unexpected tool output: %q", out)
	}

	resource, err := service.ReadResource("local-workspace", "mcp://local-workspace/readme")
	if err != nil {
		t.Fatalf("read resource: %v", err)
	}
	if resource.Content == "" {
		t.Fatalf("expected resource content")
	}

	templateResource, err := service.ReadResource("local-workspace", "mcp://local-workspace/demo")
	if err != nil {
		t.Fatalf("read template resource: %v", err)
	}
	if templateResource.Content == "" {
		t.Fatalf("expected template resource content")
	}

	status, ok := service.Ping("local-workspace")
	if !ok {
		t.Fatalf("expected ping success, status=%s", status)
	}
}

func TestMergedDefinitionsIncludesDynamicMCPTools(t *testing.T) {
	t.Parallel()

	registry := tool.EmptyRegistry()
	tool.RegisterBuiltins(registry)

	runtime := tool.Runtime{
		MCP: stubMCPRuntime{
			dynamic: []tool.MCPDynamicToolInfo{
				{
					Name:        "mcp__demo__workspace_echo",
					Server:      "demo",
					Tool:        "workspace.echo",
					Description: "echo from dynamic mcp",
					ReadOnly:    true,
				},
			},
		},
	}

	defs := mcptool.MergedDefinitions(registry, runtime)
	found := false
	for _, def := range defs {
		if def.Name() == "mcp__demo__workspace_echo" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected merged definitions to include dynamic mcp tool")
	}

	def, ok := mcptool.LookupDefinition(registry, runtime, "mcp__demo__workspace_echo")
	if !ok {
		t.Fatalf("expected dynamic definition lookup to succeed")
	}
	result, err := def.Call(context.Background(), tool.Input{"value": "ok"}, runtime)
	if err != nil {
		t.Fatalf("dynamic tool call failed: %v", err)
	}
	if result.Content != "dynamic:demo:workspace.echo" {
		t.Fatalf("unexpected dynamic tool result: %#v", result.Content)
	}
}

func TestHooksTriggerAndPersistRuntimeState(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	configPath := filepath.Join(root, "hooks.json")
	payload := `{
  "enabled": true,
  "status": "configured",
  "hooks": [
    {
      "event": "pre_turn",
      "source": "test",
      "status": "configured",
      "command": "printf hook-ok",
      "enabled": true,
      "shell": "bash",
      "blocking": true,
      "timeout_ms": 1000
    }
  ]
}
`
	if err := os.WriteFile(configPath, []byte(payload), 0o644); err != nil {
		t.Fatalf("write hooks config: %v", err)
	}

	hooks := services.CreateHooksService(configPath)
	executions, err := hooks.Trigger(context.Background(), engine.HookEvent{
		Name:   "pre_turn",
		Target: "hello",
		Payload: map[string]any{
			"input": "hello",
		},
	})
	if err != nil {
		t.Fatalf("trigger hooks: %v", err)
	}
	if len(executions) != 1 {
		t.Fatalf("expected one hook execution, got %d", len(executions))
	}

	list := hooks.List()
	if len(list) != 1 {
		t.Fatalf("expected one configured hook, got %d", len(list))
	}
	if list[0].RunCount != 1 {
		t.Fatalf("expected run_count=1, got %d", list[0].RunCount)
	}
	if list[0].LastResult == "" {
		t.Fatalf("expected last_result to be recorded")
	}
}

type stubMCPRuntime struct {
	dynamic []tool.MCPDynamicToolInfo
}

func (s stubMCPRuntime) Servers() []string { return []string{"demo"} }
func (s stubMCPRuntime) ListTools(server string) []mcp.Tool {
	return []mcp.Tool{{Name: "workspace.echo"}}
}
func (s stubMCPRuntime) ListResources(server string) []mcp.Resource {
	return []mcp.Resource{{URI: "mcp://demo/readme"}}
}
func (s stubMCPRuntime) ListTemplates(server string) []mcp.Template {
	return []mcp.Template{{URI: "mcp://demo/{name}"}}
}
func (s stubMCPRuntime) SearchTools(query string) []mcp.ToolMatch {
	return []mcp.ToolMatch{{Server: "demo", Name: "workspace.echo"}}
}
func (s stubMCPRuntime) CallTool(server, name string, args map[string]any) (string, error) {
	return "dynamic:" + server + ":" + name, nil
}
func (s stubMCPRuntime) ReadResource(server, uri string) (mcp.Resource, error) {
	return mcp.Resource{URI: uri}, nil
}
func (s stubMCPRuntime) Authenticate(server, token string) error         { return nil }
func (s stubMCPRuntime) Connect(server string) error                     { return nil }
func (s stubMCPRuntime) Disconnect(server string) error                  { return nil }
func (s stubMCPRuntime) Ping(server string) (string, error)              { return "connected", nil }
func (s stubMCPRuntime) DynamicTools() []tool.MCPDynamicToolInfo {
	return append([]tool.MCPDynamicToolInfo(nil), s.dynamic...)
}
